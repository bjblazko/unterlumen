// published-galleries.js — cross-channel "Published Galleries" overview

/* --- PublishedGalleryAPI --- */

const PublishedGalleryAPI = {
    async list() {
        const r = await fetch('/api/channels/galleries');
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async rename(channelSlug, postID, title) {
        const r = await fetch(`/api/channels/${encodeURIComponent(channelSlug)}/galleries/${encodeURIComponent(postID)}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async remove(channelSlug, postID, deleteRemote) {
        const r = await fetch(`/api/channels/${encodeURIComponent(channelSlug)}/galleries/${encodeURIComponent(postID)}`, {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ deleteRemote: !!deleteRemote }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async checkReachability(targets, onResult) {
        const r = await fetch('/api/channels/galleries/reachability', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ targets }),
        });
        if (!r.ok) throw new Error(await r.text());

        const reader = r.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });
            const blocks = buffer.split('\n\n');
            buffer = blocks.pop() ?? '';
            for (const block of blocks) {
                const line = block.split('\n').find(l => l.startsWith('data: '));
                if (!line) continue;
                try {
                    const evt = JSON.parse(line.slice(6));
                    if (!evt.complete) onResult(evt);
                } catch { /* skip malformed */ }
            }
        }
    },
};

/* --- Status column helpers ---
 * The backend's `status` field (Task 4) is only ever "draft", "generated", or
 * "live-pending" — it never says "live", because /api/channels/galleries has
 * no deploy/reachability signal of its own. "live-pending" just means
 * "generated, with a pending draft on top" — it is not a claim the gallery is
 * actually reachable. Reachability is a separate, client-side concern (the
 * SSE sweep in _startReachabilityCheck/_updateRowStatus below); these helpers
 * layer that per-row result on top of the backend's draft/generated signal. */

function _statusLabel(g, reachability) {
    if (g.status === 'draft') return `Draft · ${g.pendingCount} photo${g.pendingCount !== 1 ? 's' : ''}`;
    // g.status is 'generated' or 'live-pending' here — both just mean "something is generated
    // on disk"; whether it's actually reachable comes from the reachability check, not this string.
    const base = reachability || 'Generated';
    return g.pendingCount > 0 ? `${base} · ${g.pendingCount} pending` : base;
}

function _statusClass(g, reachState) {
    // reachState: undefined (not checkable/not yet resolved), 'checking', 'ok', 'down'.
    if (g.status === 'draft') return 'pub-gal-status--draft';
    if (reachState === 'checking') return 'pub-gal-status--pending';
    if (reachState === 'ok') return 'pub-gal-status--ok';
    if (reachState === 'down') return 'pub-gal-status--down';
    return 'pub-gal-status--generated';
}

/* --- PublishedGalleriesPane --- */

class PublishedGalleriesPane {
    constructor(container) {
        this.container = container;
        this._rendered = false;
        this._filterChannel = null;
    }

    setFilterChannel(channelSlug) {
        this._filterChannel = channelSlug || null;
    }

    render() {
        if (this._rendered) {
            this._load();
            return;
        }
        this._rendered = true;
        this.container.innerHTML = `
            <div class="pub-gal-pane">
                <div class="pub-gal-filter" id="pub-gal-filter" hidden>
                    <span></span>
                    <button class="btn btn-sm pub-gal-clear-filter">Show all channels</button>
                </div>
                <table class="pub-gal-table">
                    <thead>
                        <tr>
                            <th>Title</th>
                            <th>Channel</th>
                            <th>Published</th>
                            <th>Photos</th>
                            <th>URL</th>
                            <th>Status</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="pub-gal-rows"></tbody>
                </table>
            </div>`;
        this.container.querySelector('#pub-gal-rows').addEventListener('click', (e) => {
            const publishBtn = e.target.closest('.pub-gal-publish');
            if (publishBtn) { new PublishDialog().open(this._rows[publishBtn.dataset.idx]); return; }
            const editBtn = e.target.closest('.pub-gal-edit');
            if (editBtn) { this._editGallery(editBtn.dataset.channel, editBtn.dataset.postid); return; }
            const deleteBtn = e.target.closest('.pub-gal-delete');
            if (deleteBtn) this._deleteGallery(deleteBtn.dataset.idx);
        });
        this.container.querySelector('.pub-gal-clear-filter').addEventListener('click', () => {
            this.setFilterChannel(null);
            this._load();
        });
        this._load();
    }

    async _load() {
        const tbody = this.container.querySelector('#pub-gal-rows');
        tbody.innerHTML = '<tr><td colspan="7" class="pub-gal-empty">Loading…</td></tr>';
        let rows;
        try {
            rows = await PublishedGalleryAPI.list();
        } catch (err) {
            tbody.innerHTML = `<tr><td colspan="7" class="pub-gal-empty">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
            return;
        }

        const filterBar = this.container.querySelector('#pub-gal-filter');
        if (this._filterChannel) {
            const channelName = rows.find(r => r.channelSlug === this._filterChannel)?.channelName || this._filterChannel;
            filterBar.querySelector('span').textContent = `Showing: ${channelName}`;
            filterBar.hidden = false;
            rows = rows.filter(r => r.channelSlug === this._filterChannel);
        } else {
            filterBar.hidden = true;
        }

        if (rows.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="pub-gal-empty">No published galleries yet.</td></tr>';
            return;
        }
        this._rows = rows;
        tbody.innerHTML = rows.map((row, idx) => this._rowHTML(row, idx)).join('');
        this._startReachabilityCheck(rows);
    }

    _findRow(channelSlug, postID) {
        return (this._rows || []).find(r => r.channelSlug === channelSlug && r.postID === postID);
    }

    async _editGallery(channelSlug, postID) {
        const row = this._findRow(channelSlug, postID);
        if (!row) return;
        const input = prompt('Rename gallery:', row.title || '');
        if (input === null) return;
        const title = input.trim();
        if (!title || title === row.title) return;
        try {
            await PublishedGalleryAPI.rename(channelSlug, postID, title);
            this._load();
        } catch (err) {
            alert('Rename failed: ' + err.message);
        }
    }

    async _deleteGallery(idx) {
        const row = (this._rows || [])[idx];
        if (!row) return;

        // A draft-only row has no generated gallery on disk — there is
        // nothing for PublishedGalleryAPI.remove to delete, so route it to
        // the drafts API instead.
        if (row.status === 'draft') {
            if (!confirm(`Discard draft "${row.title || row.channelName}"? This removes the ${row.pendingCount} pending photo${row.pendingCount !== 1 ? 's' : ''}.`)) return;
            try {
                await ChannelAPI.deleteDraft(row.channelSlug, row.draftID);
                this._load();
            } catch (err) {
                alert('Delete failed: ' + err.message);
            }
            return;
        }

        if (!confirm(`Delete "${row.title || '(untitled)'}"? This removes it from the local output.`)) return;

        let deleteRemote = false;
        if (row.channelHandler === 'rsync') {
            deleteRemote = confirm('Also delete the remote copy over SSH?');
        }
        try {
            const result = await PublishedGalleryAPI.remove(row.channelSlug, row.postID, deleteRemote);
            if (result.remoteDeleteError) alert('Local delete succeeded, but remote delete failed: ' + result.remoteDeleteError);
            this._load();
        } catch (err) {
            alert('Delete failed: ' + err.message);
        }
    }

    _rowHTML(row, idx) {
        // Draft-only rows (Status naming note) have no real publish date yet
        // — galleryListItem.PublishedAt is a Go zero-time in that case, which
        // would otherwise render as a nonsense date.
        const dateStr = row.status === 'draft' ? '—' : new Date(row.publishedAt).toLocaleDateString();
        const badge = row.unlisted ? '<span class="album-badge album-badge--unlisted">Unlisted</span>' : '';
        const urlCell = row.url
            ? `<a href="${escapeHtml(row.url)}" target="_blank" rel="noopener">${escapeHtml(row.url)}</a>${row.urlGuessed ? ' <span class="form-hint">(guessed)</span>' : ''}`
            : '<span class="form-hint">No URL configured</span>';
        const reachState = row.status !== 'draft' && row.url ? 'checking' : undefined;
        const statusText = _statusLabel(row, reachState === 'checking' ? 'Checking…' : undefined);
        const statusCell = `<span class="pub-gal-status ${_statusClass(row, reachState)}">${escapeHtml(statusText)}</span>`;
        // Edit (rename) only makes sense for a row with a real generated
        // gallery/album on disk — a draft-only row has no postID yet.
        const editBtn = row.status === 'draft' ? '' :
            `<button class="btn btn-sm pub-gal-edit" data-channel="${escapeHtml(row.channelSlug)}" data-postid="${escapeHtml(row.postID)}">Edit</button>`;
        return `
            <tr data-idx="${idx}" data-channel="${escapeHtml(row.channelSlug)}" data-postid="${escapeHtml(row.postID)}">
                <td>${escapeHtml(row.title || '(untitled)')} ${badge}</td>
                <td>${escapeHtml(row.channelName)}</td>
                <td>${dateStr}</td>
                <td>${row.photoCount}</td>
                <td class="pub-gal-url">${urlCell}</td>
                <td class="pub-gal-status-cell">${statusCell}</td>
                <td class="pub-gal-actions">
                    <button class="btn btn-sm pub-gal-publish" data-idx="${idx}">Publish</button>
                    ${editBtn}
                    <button class="btn btn-sm pub-gal-delete" data-idx="${idx}">Delete</button>
                </td>
            </tr>`;
    }

    async _startReachabilityCheck(rows) {
        const targets = rows
            .filter(r => r.url)
            .map(r => ({ channelSlug: r.channelSlug, postID: r.postID, url: r.url }));
        if (targets.length === 0) return;

        try {
            await PublishedGalleryAPI.checkReachability(targets, (evt) => {
                this._updateRowStatus(evt.channelSlug, evt.postID, evt.reachable, evt.error);
            });
        } catch { /* leave any still-pending rows as "Checking…" */ }
    }

    _updateRowStatus(slug, postID, reachable, error) {
        const row = this.container.querySelector(`tr[data-channel="${CSS.escape(slug)}"][data-postid="${CSS.escape(postID)}"]`);
        if (!row) return;
        const cell = row.querySelector('.pub-gal-status');
        if (!cell) return;
        const rowData = this._findRow(slug, postID);
        const reachState = reachable ? 'ok' : 'down';
        const label = reachable ? '● Live' : '✗ Unreachable';
        cell.className = 'pub-gal-status ' + (rowData ? _statusClass(rowData, reachState) : `pub-gal-status--${reachState}`);
        cell.textContent = rowData ? _statusLabel(rowData, label) : label;
        if (error) cell.title = error;
    }
}
