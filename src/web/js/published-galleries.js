// published-galleries.js — the Galleries place: every gallery across every
// destination, grouped by destination, plus one gallery's detail view.
//
// Vocabulary (ADR-0029): a channel is a "destination" and an album is a
// "gallery" in everything a person reads. The backend names (channel,
// galleryExport, siteExport, drafts.json) are unchanged, so this file still
// talks to /api/channels/….

/* --- PublishedGalleryAPI --- */

const PublishedGalleryAPI = {
    async list() {
        const r = await fetch('/api/channels/galleries');
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async rename(channelSlug, postID, title, unlisted) {
        const body = { title };
        if (unlisted !== undefined) body.unlisted = unlisted;
        const r = await fetch(`/api/channels/${encodeURIComponent(channelSlug)}/galleries/${encodeURIComponent(postID)}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
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

/* --- Gallery state (ADR-0029) ---
 *
 * States are adjectives, actions are verbs. The backend's `status` field only
 * ever says "draft", "generated" or "live-pending" — it knows what exists on
 * disk, never whether the result is reachable. Reachability is a separate
 * check result layered *on top of* a state, not a state of its own: a gallery
 * can be "Changes not online" and unreachable at the same time.
 *
 * `generatedAt` and `deployedAt` are per gallery. Both are absent for
 * galleries built before they were recorded, and absent means "not recorded",
 * never "never" — an old gallery is not accused of having failed to upload.
 */

// A recorded timestamp, or null. Go's omitempty does nothing for a
// time.Time, so an unrecorded time arrives as "0001-01-01T00:00:00Z" rather
// than as an absent field — and must not be read as "this never happened".
function recordedTime(value) {
    if (!value) return null;
    const d = new Date(value);
    return isNaN(d) || d.getUTCFullYear() <= 1970 ? null : d;
}

function galleryState(row, channel) {
    if (row.status === 'draft') return { key: 'draft', label: 'Not online yet' };
    if (row.status === 'live-pending') return { key: 'pending', label: 'Changes not online' };

    // A files destination builds no pages and has nowhere to upload to: the
    // files are ready in a folder, and posting them is the user's own job.
    const buildsPages = !!(row.galleryExport || channel?.siteExport);
    if (!buildsPages) return { key: 'exported', label: 'Exported to folder' };

    // Built after the last upload — or never uploaded at all. Only decidable
    // for galleries built since these timestamps were recorded; for older
    // ones the absence of a timestamp means "not recorded", not "never".
    const generated = recordedTime(row.generatedAt);
    const deployed = recordedTime(row.deployedAt);
    if (generated && (!deployed || deployed < generated)) {
        return { key: 'built', label: 'Built, not uploaded' };
    }
    if (!row.url) return { key: 'built', label: 'Built, no address configured' };
    return { key: 'online', label: 'Online' };
}

// The one thing worth doing to this gallery right now, if there is one.
function galleryAction(row, check, channel) {
    const state = galleryState(row, channel);
    if (state.key === 'draft') return { act: 'publish', label: 'Publish' };
    if (state.key === 'pending') {
        return { act: 'publish', label: `Publish ${row.pendingCount} change${row.pendingCount !== 1 ? 's' : ''}` };
    }
    if (state.key === 'built' && channel?.handler) return { act: 'publish', label: 'Retry upload' };
    if (state.key === 'exported') return { act: 'reveal', label: 'Show in Finder' };
    if (check && check.reachable === false) return { act: 'recheck', label: 'Check again' };
    return null;
}

function formatTime(date) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

/* --- GalleriesPane --- */

class GalleriesPane {
    constructor(container) {
        this.container = container;
        this._rendered = false;
        this._filterChannel = null;
        this._openRowKey = null;
        // rowKey → { reachable, error, at }. Kept for the life of the pane, so
        // switching places does not re-probe every address (ADR-0029).
        this._checks = new Map();
    }

    setFilterChannel(channelSlug) {
        this._filterChannel = channelSlug || null;
    }

    render() {
        this._rendered = true;
        this._load();
    }

    async _load() {
        this.container.innerHTML = '<div class="gal-pane"><div class="gal-loading">Loading…</div></div>';
        let rows, channels;
        try {
            [rows, channels] = await Promise.all([PublishedGalleryAPI.list(), ChannelAPI.list()]);
        } catch (err) {
            this.container.innerHTML = `<div class="gal-pane"><div class="gal-error">Could not load galleries: ${escapeHtml(err.message)}</div></div>`;
            return;
        }
        this._rows = rows;
        this._channels = channels;
        if (this._openRowKey && this._findRow(this._openRowKey)) this._renderDetail();
        else { this._openRowKey = null; this._renderList(); }
    }

    _findRow(rowKey) {
        return (this._rows || []).find(r => r.rowKey === rowKey);
    }

    _channelBySlug(slug) {
        return (this._channels || []).find(c => c.slug === slug);
    }

    // Share links / Website / Files — what the destination is, in one word.
    _destinationType(ch) {
        if (!ch) return '';
        if (ch.siteExport) return 'Website';
        if (ch.galleryExport) return 'Share links';
        return 'Files';
    }

    // Returns { text, isAddress } — "no address configured" is a sentence,
    // not a path, and must not be set in the data voice.
    _destinationAddress(ch) {
        const base = ch ? _deployBaseURL(ch) : null;
        if (base) return { text: base.url.replace(/^https?:\/\//, '') + (base.guessed ? ' (guessed)' : ''), isAddress: true };
        if (ch?.outputPath) return { text: ch.outputPath, isAddress: true };
        return { text: 'no address configured', isAddress: false };
    }

    /* --- List --- */

    _renderList() {
        let rows = this._rows;
        const filtered = this._filterChannel
            ? rows.filter(r => r.channelSlug === this._filterChannel)
            : rows;

        const bySlug = new Map();
        for (const row of filtered) {
            if (!bySlug.has(row.channelSlug)) bySlug.set(row.channelSlug, []);
            bySlug.get(row.channelSlug).push(row);
        }

        const todo = filtered.filter(r => galleryAction(r, this._checks.get(r.rowKey), this._channelBySlug(r.channelSlug))?.act === 'publish').length;

        const groups = [...bySlug.entries()].map(([slug, groupRows]) => {
            const ch = this._channelBySlug(slug);
            const name = ch?.name || groupRows[0].channelName;
            return `
                <section class="gal-group" aria-label="${escapeHtml(name)}">
                    <div class="gal-group-head">
                        <h2 class="gal-group-name">${escapeHtml(name)}</h2>
                        <span class="gal-group-meta">${escapeHtml(this._destinationType(ch))} · ${(() => {
                            const addr = this._destinationAddress(ch);
                            return addr.isAddress
                                ? `<span class="gal-group-addr">${escapeHtml(addr.text)}</span>`
                                : escapeHtml(addr.text);
                        })()}</span>
                        <span class="gal-group-spacer"></span>
                        <button class="btn btn-sm gal-dest-settings" data-slug="${escapeHtml(slug)}">Destination settings</button>
                    </div>
                    ${groupRows.map(r => this._rowHTML(r)).join('')}
                </section>`;
        }).join('');

        this.container.innerHTML = `
            <div class="gal-pane">
                <div class="gal-head">
                    <h1 class="gal-title">Galleries</h1>
                    <span class="gal-head-count">${filtered.length} ${filtered.length === 1 ? 'gallery' : 'galleries'}${todo ? ` · ${todo} to publish` : ''}</span>
                    <span class="gal-group-spacer"></span>
                    ${this._filterChannel
                        ? `<span class="gal-filter" id="gal-filter">Showing ${escapeHtml(this._channelBySlug(this._filterChannel)?.name || this._filterChannel)}</span>
                           <button class="btn btn-sm gal-clear-filter">Show all destinations</button>`
                        : ''}
                </div>
                <div class="gal-body">
                    ${filtered.length === 0
                        ? `<p class="gal-empty">${this._filterChannel
                            ? 'Nothing has been collected for this destination yet.'
                            : 'No galleries yet. Select photos in a library and choose "Add to channel…" to collect the first one.'}</p>`
                        : groups}
                </div>
            </div>`;

        this.container.querySelector('.gal-clear-filter')?.addEventListener('click', () => {
            this.setFilterChannel(null);
            this._renderList();
        });
        for (const btn of this.container.querySelectorAll('.gal-dest-settings')) {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                App.showDestination(btn.dataset.slug);
            });
        }
        for (const el of this.container.querySelectorAll('.gal-row')) {
            el.addEventListener('click', (e) => {
                if (e.target.closest('.gal-row-action')) return;
                this._openRowKey = el.dataset.rowkey;
                this._renderDetail();
            });
            el.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    this._openRowKey = el.dataset.rowkey;
                    this._renderDetail();
                }
            });
        }
        for (const btn of this.container.querySelectorAll('.gal-row-action')) {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                this._runAction(btn.dataset.act, btn.dataset.rowkey);
            });
        }

        this._startReachabilityCheck(filtered);
    }

    _rowHTML(row) {
        const channel = this._channelBySlug(row.channelSlug);
        const state = galleryState(row, channel);
        const check = this._checks.get(row.rowKey);
        const action = galleryAction(row, check, channel);
        const sub = state.key === 'draft'
            ? `${row.pendingCount} photo${row.pendingCount !== 1 ? 's' : ''} collected`
            : state.key === 'pending'
                ? `${row.photoCount} online · ${row.pendingCount} added since ${new Date(row.publishedAt).toLocaleDateString()}`
                : `${row.photoCount} photo${row.photoCount !== 1 ? 's' : ''} · ${new Date(row.publishedAt).toLocaleDateString()}`;
        return `
            <div class="gal-row" role="link" tabindex="0" data-rowkey="${escapeHtml(row.rowKey)}" data-postid="${escapeHtml(row.postID || '')}">
                <span class="gal-row-main">
                    <span class="gal-row-title">${escapeHtml(row.title || '(untitled)')}</span>
                    <span class="gal-row-sub">${escapeHtml(sub)}${row.unlisted && row.galleryExport ? ' · hidden from search' : ''}</span>
                </span>
                <span class="gal-row-state">
                    <span class="gal-state gal-state--${state.key}">${escapeHtml(state.label)}</span>
                    ${this._checkHTML(row)}
                </span>
                <span class="gal-row-actions">
                    ${action ? `<button class="btn btn-sm gal-row-action" data-act="${action.act}" data-rowkey="${escapeHtml(row.rowKey)}">${escapeHtml(action.label)}</button>` : ''}
                </span>
            </div>`;
    }

    // The link check is a result about the state, never a state of its own.
    _checkHTML(row) {
        const state = galleryState(row, this._channelBySlug(row.channelSlug)).key;
        if (state === 'draft' || state === 'exported' || !row.url) return '';
        const check = this._checks.get(row.rowKey);
        if (!check) return '<span class="gal-check gal-check--running">Checking the link…</span>';
        const at = formatTime(check.at);
        if (check.reachable) return `<span class="gal-check">Link answered at ${escapeHtml(at)}</span>`;
        return `<span class="gal-check gal-check--down" title="${escapeHtml(check.error || '')}">Not reachable · ${escapeHtml(at)}</span>`;
    }

    async _runAction(act, rowKey) {
        const row = this._findRow(rowKey);
        if (!row) return;
        if (act === 'publish') {
            await new PublishDialog().open(row);
            this._checks.delete(rowKey);
            this._load();
            return;
        }
        if (act === 'reveal') {
            ChannelAPI.reveal(row.channelSlug);
            return;
        }
        if (act === 'recheck') {
            this._checks.delete(rowKey);
            this._updateRowCheck(rowKey);
            this._startReachabilityCheck([row], { force: true });
        }
    }

    /* --- Reachability --- */

    async _startReachabilityCheck(rows, { force = false } = {}) {
        // A draft has no gallery on its address yet, so probing it would say
        // nothing. Already-checked rows keep their cached result until the
        // user asks again or a publish invalidates it.
        const targets = rows
            .filter(r => r.status !== 'draft' && r.url && (force || !this._checks.has(r.rowKey)))
            .map(r => ({ channelSlug: r.channelSlug, postID: r.postID, rowKey: r.rowKey, url: r.url }));
        if (targets.length === 0) return;

        try {
            await PublishedGalleryAPI.checkReachability(targets, (evt) => {
                // The event's `reachable` is omitempty on the Go side, so a
                // failed check arrives as an absent field, not as false.
                this._checks.set(evt.rowKey, { reachable: !!evt.reachable, error: evt.error, at: new Date() });
                this._updateRowCheck(evt.rowKey);
            });
        } catch { /* rows keep saying the check is running, which is the truth */ }
    }

    _updateRowCheck(rowKey) {
        const row = this._findRow(rowKey);
        const rowEl = this.container.querySelector(`.gal-row[data-rowkey="${CSS.escape(rowKey)}"]`);
        if (!row || !rowEl) return;
        const channel = this._channelBySlug(row.channelSlug);
        const state = galleryState(row, channel);
        rowEl.querySelector('.gal-row-state').innerHTML =
            `<span class="gal-state gal-state--${state.key}">${escapeHtml(state.label)}</span>${this._checkHTML(row)}`;
        const action = galleryAction(row, this._checks.get(rowKey), channel);
        rowEl.querySelector('.gal-row-actions').innerHTML = action
            ? `<button class="btn btn-sm gal-row-action" data-act="${action.act}" data-rowkey="${escapeHtml(rowKey)}">${escapeHtml(action.label)}</button>`
            : '';
        const btn = rowEl.querySelector('.gal-row-action');
        if (btn) btn.addEventListener('click', (e) => { e.stopPropagation(); this._runAction(btn.dataset.act, rowKey); });
    }

    /* --- Detail --- */

    async _renderDetail() {
        const row = this._findRow(this._openRowKey);
        if (!row) { this._renderList(); return; }
        const ch = this._channelBySlug(row.channelSlug);
        const state = galleryState(row, ch);
        const check = this._checks.get(row.rowKey);
        const action = galleryAction(row, check, ch);
        const canEditVisibility = !!row.galleryExport;

        const published = new Date(row.publishedAt).toLocaleDateString();
        const stateLine = state.key === 'draft'
            ? 'Never published. Publishing exports the photos, builds the page and — where an upload is set up — uploads it.'
            : state.key === 'pending'
                ? `Online since ${published}. ${row.pendingCount} change${row.pendingCount !== 1 ? 's are' : ' is'} not online yet.`
                : state.key === 'exported'
                    ? `Exported ${published} as ${row.photoCount} file${row.photoCount !== 1 ? 's' : ''}. This destination has no upload configured, so putting them anywhere is up to you.`
                    : state.key === 'built'
                        ? (ch?.handler
                            ? `Built${recordedTime(row.generatedAt) ? ' on ' + recordedTime(row.generatedAt).toLocaleDateString() : ''}, and not uploaded since. Publishing again uploads it.`
                            : `Built${recordedTime(row.generatedAt) ? ' on ' + recordedTime(row.generatedAt).toLocaleDateString() : ''}. This destination has no upload configured, so the files only exist in the local output folder.`)
                        : check && check.reachable === false
                            ? `Published ${published}. The address did not answer at ${formatTime(check.at)}.`
                            : `Published ${published}.`;

        this.container.innerHTML = `
            <div class="gal-pane gal-detail">
                <div class="gal-head">
                    <nav class="gal-crumbs" aria-label="Breadcrumb">
                        <button class="link-btn gal-back">Galleries</button>
                        <span class="gal-crumb-sep">/</span>
                        <span class="gal-crumb-here">${escapeHtml(row.title || '(untitled)')}</span>
                    </nav>
                    <span class="gal-group-spacer"></span>
                    ${row.url && state.key !== 'draft' ? `<a class="btn btn-sm" href="${escapeHtml(row.url)}" target="_blank" rel="noopener">Open in browser</a>` : ''}
                </div>
                <div class="gal-detail-body">
                    <div class="gal-detail-main">
                        <div class="gal-detail-title-row">
                            <h1 class="gal-title">${escapeHtml(row.title || '(untitled)')}</h1>
                            <span class="gal-state gal-state--${state.key}">${escapeHtml(state.label)}</span>
                        </div>
                        <p class="gal-detail-state-line">${escapeHtml(stateLine)}</p>
                        ${action
                            ? `<div class="gal-detail-primary"><button class="btn btn-accent gal-row-action" data-act="${action.act}" data-rowkey="${escapeHtml(row.rowKey)}">${escapeHtml(action.label)}</button></div>`
                            : `<div class="gal-detail-primary">
                                   <button class="btn btn-sm gal-row-action" data-act="publish" data-rowkey="${escapeHtml(row.rowKey)}">Publish again</button>
                                   <span class="form-hint">Nothing is waiting. Publishing again rebuilds this gallery — needed only after a theme or format change.</span>
                               </div>`}
                        <section class="gal-pending" id="gal-pending"></section>
                    </div>
                    <aside class="gal-detail-panel" aria-label="Gallery settings">
                        <div class="form-field">
                            <label class="form-label" for="gal-title-input">Title</label>
                            <div class="gal-title-edit">
                                <input class="form-input" id="gal-title-input" value="${escapeHtml(row.title || '')}" ${state.key === 'draft' ? 'disabled' : ''}>
                                <button class="btn btn-sm" id="gal-title-save" ${state.key === 'draft' ? 'disabled' : ''}>Save</button>
                            </div>
                            ${state.key === 'draft' ? '<span class="form-hint">The title is set when you publish this gallery for the first time.</span>' : ''}
                        </div>
                        <div class="form-field">
                            <span class="form-label">Date shown on the gallery</span>
                            <span class="gal-detail-value">${state.key === 'draft' ? 'Set on first publish' : escapeHtml(new Date(row.publishedAt).toLocaleDateString())}</span>
                            <span class="form-hint">Stays as it is when you publish changes.</span>
                        </div>
                        <div class="form-field">
                            <span class="form-label">Search engines</span>
                            <div id="gal-visibility"></div>
                            <span class="form-hint">${canEditVisibility
                                ? 'Hidden adds a noindex tag. The link itself stays unguessable either way.'
                                : 'Website albums are listed on the site\'s index and in its sitemap. Set when the album was created.'}</span>
                        </div>
                        <div class="form-field">
                            <span class="form-label">Address</span>
                            <span class="gal-detail-value${row.url ? ' gal-detail-url' : ''}">${row.url ? escapeHtml(row.url) : 'No address configured'}</span>
                            ${row.urlGuessed ? '<span class="form-hint">Guessed from the upload host, not configured.</span>' : ''}
                        </div>
                        <dl class="gal-detail-kv">
                            <dt>Destination</dt><dd>${escapeHtml(ch?.name || row.channelName)}</dd>
                            <dt>Type</dt><dd>${escapeHtml(this._destinationType(ch))}</dd>
                        </dl>
                        <div class="gal-detail-danger" id="gal-danger"></div>
                    </aside>
                </div>
            </div>`;

        this.container.querySelector('.gal-back').addEventListener('click', () => {
            this._openRowKey = null;
            this._renderList();
        });
        const primary = this.container.querySelector('.gal-detail-primary .gal-row-action');
        if (primary) primary.addEventListener('click', () => this._runAction(primary.dataset.act, row.rowKey));

        this._wireTitleSave(row);
        this._wireVisibility(row, canEditVisibility);
        this._renderDangerZone(row, state);
        this._renderPendingPhotos(row);
    }

    _wireTitleSave(row) {
        const input = this.container.querySelector('#gal-title-input');
        const btn = this.container.querySelector('#gal-title-save');
        if (!btn || btn.disabled) return;
        btn.addEventListener('click', async () => {
            const title = input.value.trim();
            if (!title) { this._showDetailError('The title must not be empty.'); return; }
            btn.disabled = true;
            try {
                await PublishedGalleryAPI.rename(row.channelSlug, row.postID, title);
                await this._load();
            } catch (err) {
                btn.disabled = false;
                this._showDetailError('Could not save the title: ' + err.message);
            }
        });
    }

    _wireVisibility(row, canEdit) {
        const wrap = this.container.querySelector('#gal-visibility');
        if (!canEdit) return;
        // Three visible labels, as every toggle carries (ADR-0019).
        const toggle = Toggle.create(wrap, {
            initial: !!row.unlisted,
            labelOn: 'Hidden',
            labelOff: 'Allowed',
            onChange: async (on) => {
                try {
                    await PublishedGalleryAPI.rename(row.channelSlug, row.postID, row.title, on);
                    row.unlisted = on;
                } catch (err) {
                    toggle.setState(!on);
                    this._showDetailError('Could not change the visibility: ' + err.message);
                }
            },
        });
    }

    // Destructive actions confirm in place, naming what goes and what stays.
    _renderDangerZone(row, state) {
        const wrap = this.container.querySelector('#gal-danger');
        const isDraft = state.key === 'draft';
        wrap.innerHTML = `<button class="btn btn-sm btn-danger" id="gal-remove">${isDraft ? 'Discard draft…' : 'Unpublish…'}</button>`;
        wrap.querySelector('#gal-remove').addEventListener('click', () => {
            const canDeleteRemote = !isDraft && row.channelHandler === 'rsync';
            wrap.innerHTML = `
                <p class="gal-danger-question">${isDraft
                    ? `Discard this draft? The ${row.pendingCount} collected photo${row.pendingCount !== 1 ? 's stay' : ' stays'} in your library.`
                    : `Unpublish "${escapeHtml(row.title || '(untitled)')}"? This removes the built gallery from the local output. The photos stay in your libraries.`}</p>
                ${canDeleteRemote ? `<label class="gal-danger-remote"><input type="checkbox" id="gal-remove-remote"> Also delete it on the remote host over SSH</label>` : ''}
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="gal-remove-cancel">Keep it</button>
                    <button class="btn btn-sm btn-danger" id="gal-remove-confirm">${isDraft ? 'Discard draft' : 'Unpublish'}</button>
                </div>
                <div class="gal-detail-error" id="gal-danger-error" hidden></div>`;
            wrap.querySelector('#gal-remove-cancel').addEventListener('click', () => this._renderDangerZone(row, state));
            wrap.querySelector('#gal-remove-confirm').addEventListener('click', async () => {
                const errEl = wrap.querySelector('#gal-danger-error');
                try {
                    if (isDraft) {
                        await ChannelAPI.deleteDraft(row.channelSlug, row.draftID);
                    } else {
                        const deleteRemote = !!wrap.querySelector('#gal-remove-remote')?.checked;
                        const result = await PublishedGalleryAPI.remove(row.channelSlug, row.postID, deleteRemote);
                        if (result.remoteDeleteError) {
                            errEl.textContent = 'Removed locally, but the remote copy is still there: ' + result.remoteDeleteError;
                            errEl.hidden = false;
                            return;
                        }
                    }
                    this._openRowKey = null;
                    this._checks.delete(row.rowKey);
                    await this._load();
                } catch (err) {
                    errEl.textContent = 'Could not remove it: ' + err.message;
                    errEl.hidden = false;
                }
            });
        });
    }

    // What will go online with the next publish, and what is already there.
    async _renderPendingPhotos(row) {
        const wrap = this.container.querySelector('#gal-pending');
        if (!row.draftID || row.pendingCount === 0) {
            wrap.innerHTML = '';
            return;
        }
        wrap.innerHTML = '<div class="gal-loading">Loading the collected photos…</div>';
        let draft = null;
        try {
            const drafts = await ChannelAPI.listDrafts(row.channelSlug);
            draft = drafts.find(d => d.id === row.draftID) || null;
        } catch (err) {
            wrap.innerHTML = `<div class="gal-error">Could not load the collected photos: ${escapeHtml(err.message)}</div>`;
            return;
        }
        const photos = draft?.photos || [];
        wrap.innerHTML = `
            <h2 class="gal-section-title">${galleryState(row, this._channelBySlug(row.channelSlug)).key === 'draft'
                ? `${photos.length} photo${photos.length !== 1 ? 's' : ''} collected`
                : `Not online yet: ${photos.length} added`}</h2>
            <div class="gal-pending-photos">
                ${photos.map(p => `
                    <div class="gal-pending-photo" data-lib="${escapeHtml(p.libraryID)}" data-photo="${escapeHtml(p.photoID)}">
                        <img src="/api/library/${encodeURIComponent(p.libraryID)}/thumb/${encodeURIComponent(p.photoID)}" alt="" loading="lazy">
                        <button class="gal-pending-remove" title="Leave this photo out">×</button>
                    </div>`).join('')}
            </div>`;
        for (const btn of wrap.querySelectorAll('.gal-pending-remove')) {
            btn.addEventListener('click', async () => {
                const card = btn.closest('.gal-pending-photo');
                try {
                    await ChannelAPI.removeDraftPhoto(row.channelSlug, draft.id, card.dataset.lib, card.dataset.photo);
                    await this._load();
                } catch (err) {
                    this._showDetailError('Could not remove the photo: ' + err.message);
                }
            });
        }
    }

    _showDetailError(message) {
        let el = this.container.querySelector('.gal-detail-error-banner');
        if (!el) {
            el = document.createElement('div');
            el.className = 'gal-detail-error-banner';
            this.container.querySelector('.gal-detail-main')?.prepend(el);
        }
        el.textContent = message;
    }
}
