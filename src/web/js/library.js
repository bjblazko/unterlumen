// LibraryTab — Libraries mode: list, create, and browse photo libraries

/* --- Library API helpers --- */

const LibraryAPI = {
    async list() {
        const r = await fetch('/api/library/');
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async create(name, description, sourcePath) {
        const r = await fetch('/api/library/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description, sourcePath }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async get(id) {
        const r = await fetch(`/api/library/${id}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async delete(id) {
        const r = await fetch(`/api/library/${id}`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
    },
    async photos(id, { q = '', offset = 0, limit = 100, ...filters } = {}) {
        const params = new URLSearchParams({ offset, limit });
        if (q) params.set('q', q);
        for (const [k, v] of Object.entries(filters)) params.set(k, v);
        const r = await fetch(`/api/library/${id}/photos?${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    thumbURL(libID, photoID) {
        return `/api/library/${libID}/thumb/${photoID}`;
    },
    photoURL(libID, photoID) {
        return `/api/library/${libID}/photo/${photoID}`;
    },
    async photoIDByPath(libID, relPath) {
        const r = await fetch(`/api/library/${libID}/photo-id-by-path?path=${encodeURIComponent(relPath)}`);
        if (!r.ok) return null;
        const { photoID } = await r.json();
        return photoID;
    },
    async getMeta(libID, photoID) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}/meta`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async publish(libID, { photoIDs, channel, publishedAt }) {
        const r = await fetch(`/api/library/${libID}/publish`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, channel, publishedAt }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async upsertMeta(libID, photoID, key, value) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}/meta`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key, value }),
        });
        if (!r.ok) throw new Error(await r.text());
    },
    async deleteMeta(libID, photoID, key) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}/meta?key=${encodeURIComponent(key)}`, {
            method: 'DELETE',
        });
        if (!r.ok) throw new Error(await r.text());
    },
    reindex(id, onProgress) {
        return new Promise((resolve, reject) => {
            const es = new EventSource(`/api/library/${id}/reindex`);
            // EventSource only supports GET; use fetch for POST with SSE
            es.close();

            // Use fetch + ReadableStream for POST SSE
            const ctrl = new AbortController();
            fetch(`/api/library/${id}/reindex`, { method: 'POST', signal: ctrl.signal })
                .then(async r => {
                    if (!r.ok) { reject(new Error(await r.text())); return; }
                    const reader = r.body.getReader();
                    const dec = new TextDecoder();
                    let buf = '';
                    while (true) {
                        const { done, value } = await reader.read();
                        if (done) break;
                        buf += dec.decode(value, { stream: true });
                        const lines = buf.split('\n');
                        buf = lines.pop();
                        for (const line of lines) {
                            const t = line.trim();
                            if (t.startsWith('data:')) {
                                try {
                                    const p = JSON.parse(t.slice(5).trim());
                                    onProgress(p);
                                    if (p.finished) { resolve(); return; }
                                } catch {}
                            }
                        }
                    }
                    resolve();
                })
                .catch(err => { if (err.name !== 'AbortError') reject(err); });
        });
    },
};

/* --- LibraryTab --- */

class LibraryTab {
    constructor(container) {
        this.container = container;
        this.currentLibrary = null;
        this._pane = null;
        this._infoPanel = null;
    }

    getActivePaneForKeyboard() {
        return this._pane;
    }

    render() {
        this.container.innerHTML = '';
        this.container.className = 'library-root';
        if (this.currentLibrary) {
            this._renderDetail();
        } else {
            this._renderList();
        }
    }

    /* --- Library list --- */

    _renderList() {
        const el = document.createElement('div');
        el.className = 'library-list-view';
        el.innerHTML = `
            <div class="library-list-header">
                <h2 class="library-list-title">Libraries</h2>
                <button class="btn btn-accent" id="lib-create-btn">New library</button>
            </div>
            <div class="library-list-body" id="lib-list-body">
                <div class="library-loading">Loading…</div>
            </div>`;
        this.container.appendChild(el);

        el.querySelector('#lib-create-btn').addEventListener('click', () => this._showCreateDialog());
        this._loadList(el.querySelector('#lib-list-body'));
    }

    async _loadList(body) {
        try {
            const libs = await LibraryAPI.list();
            body.innerHTML = '';
            if (libs.length === 0) {
                body.innerHTML = '<div class="library-empty">No libraries yet. Create one to get started.</div>';
                return;
            }
            for (const lib of libs) {
                body.appendChild(this._libCard(lib));
            }
        } catch (err) {
            body.innerHTML = `<div class="library-error">Failed to load libraries: ${err.message}</div>`;
        }
    }

    _libCard(lib) {
        const card = document.createElement('div');
        card.className = 'library-card';
        const lastIdx = lib.lastIndexed
            ? new Date(lib.lastIndexed).toLocaleDateString()
            : 'Never';
        card.innerHTML = `
            <div class="library-card-info">
                <div class="library-card-name">${escapeHtml(lib.name)}</div>
                <div class="library-card-meta">${escapeHtml(lib.sourcePath)}</div>
                <div class="library-card-stats">${lib.photoCount} photos · Last indexed: ${lastIdx}</div>
                ${lib.description ? `<div class="library-card-desc">${escapeHtml(lib.description)}</div>` : ''}
            </div>
            <div class="library-card-actions">
                <button class="btn btn-sm btn-accent lib-open">Open</button>
                <button class="btn btn-sm lib-reindex">Re-index</button>
                <button class="btn btn-sm lib-delete">Delete</button>
            </div>`;

        card.querySelector('.lib-open').addEventListener('click', () => this._openLibrary(lib));
        card.querySelector('.lib-reindex').addEventListener('click', (e) => this._reindexCard(lib, card, e.target));
        card.querySelector('.lib-delete').addEventListener('click', () => this._deleteLibrary(lib, card));
        return card;
    }

    async _reindexCard(lib, card, btn) {
        const progress = card.querySelector('.library-card-progress') || (() => {
            const p = document.createElement('div');
            p.className = 'library-card-progress';
            card.appendChild(p);
            return p;
        })();
        btn.disabled = true;
        progress.textContent = 'Indexing…';
        try {
            await LibraryAPI.reindex(lib.id, (p) => {
                if (p.finished) {
                    progress.textContent = `Done — ${p.total} photos indexed.`;
                } else {
                    progress.textContent = `${p.done} / ${p.total}${p.current ? ' · ' + p.current : ''}`;
                }
            });
            // Refresh card stats.
            const updated = await LibraryAPI.get(lib.id);
            card.querySelector('.library-card-stats').textContent =
                `${updated.photoCount} photos · Last indexed: ${new Date(updated.lastIndexed).toLocaleDateString()}`;
        } catch (err) {
            progress.textContent = `Error: ${err.message}`;
        } finally {
            btn.disabled = false;
        }
    }

    async _deleteLibrary(lib, card) {
        if (!confirm(`Delete library "${lib.name}"?\n\nThis removes the index and thumbnails. Your original photos are not affected.`)) return;
        try {
            await LibraryAPI.delete(lib.id);
            card.remove();
        } catch (err) {
            alert('Delete failed: ' + err.message);
        }
    }

    _showCreateDialog(prefillPath) {
        const dlg = document.createElement('div');
        dlg.className = 'library-dialog-backdrop';
        dlg.innerHTML = `
            <div class="library-dialog">
                <h3 class="library-dialog-title">New Library</h3>
                <label class="library-dialog-label">Name</label>
                <input class="library-dialog-input" id="lib-dlg-name" type="text" placeholder="My Photos" autocomplete="off">
                <label class="library-dialog-label">Source folder</label>
                <input class="library-dialog-input" id="lib-dlg-path" type="text" placeholder="/path/to/photos">
                <label class="library-dialog-label">Description (optional)</label>
                <input class="library-dialog-input" id="lib-dlg-desc" type="text" placeholder="">
                <div class="library-dialog-note">The folder will be scanned when you click Create. Large folders may take a few minutes.</div>
                <div class="library-dialog-actions">
                    <button class="btn" id="lib-dlg-cancel">Cancel</button>
                    <button class="btn btn-accent" id="lib-dlg-create">Create &amp; index</button>
                </div>
                <div class="library-dialog-progress" id="lib-dlg-progress" style="display:none"></div>
            </div>`;
        document.body.appendChild(dlg);

        const nameEl = dlg.querySelector('#lib-dlg-name');
        const pathEl = dlg.querySelector('#lib-dlg-path');
        const descEl = dlg.querySelector('#lib-dlg-desc');
        const progressEl = dlg.querySelector('#lib-dlg-progress');
        const createBtn = dlg.querySelector('#lib-dlg-create');

        if (prefillPath) {
            pathEl.value = prefillPath;
            nameEl.value = prefillPath.split('/').filter(Boolean).pop() || '';
            nameEl.focus();
        } else {
            nameEl.focus();
        }

        dlg.querySelector('#lib-dlg-cancel').addEventListener('click', () => dlg.remove());

        createBtn.addEventListener('click', async () => {
            const name = nameEl.value.trim();
            const path = stripQuotes(pathEl.value.trim());
            pathEl.value = path; // show cleaned value
            if (!name || !path) { alert('Name and source folder are required.'); return; }

            createBtn.disabled = true;
            progressEl.style.display = '';
            progressEl.textContent = 'Creating library…';

            try {
                const lib = await LibraryAPI.create(name, descEl.value.trim(), path);
                progressEl.textContent = 'Indexing photos…';
                await LibraryAPI.reindex(lib.id, (p) => {
                    if (p.finished) {
                        progressEl.textContent = `Done — ${p.total} photos indexed.`;
                    } else {
                        progressEl.textContent = `${p.done} / ${p.total}${p.current ? ' · ' + p.current : ''}`;
                    }
                });
                dlg.remove();
                this._openLibrary(lib);
            } catch (err) {
                progressEl.style.color = 'var(--accent)';
                progressEl.textContent = 'Error: ' + err.message;
                createBtn.disabled = false;
            }
        });
    }

    // Open the create dialog pre-filled with a known path (from Tools menu).
    openCreateDialogForPath(sourcePath) {
        this._showCreateDialog(sourcePath);
    }

    /* --- Library detail --- */

    _openLibrary(lib) {
        this.currentLibrary = lib;
        this._pane = null;
        this._infoPanel = null;
        this.render();
    }

    _renderDetail() {
        const lib = this.currentLibrary;
        const el = document.createElement('div');
        el.className = 'library-detail';
        el.innerHTML = `
            <div class="library-detail-header">
                <button class="btn btn-sm library-back-btn" id="lib-back">
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7 2L3 6l4 4"/></svg>
                    Libraries
                </button>
                <div class="library-detail-title">
                    <span class="library-detail-name">${escapeHtml(lib.name)}</span>
                    <span class="library-detail-path">${escapeHtml(lib.sourcePath)}</span>
                </div>
                <div class="library-detail-controls">
                    <button class="btn btn-sm lib-publish-btn" id="lib-publish-btn" disabled>Publish…</button>
                    <button class="btn btn-sm" id="lib-reindex-btn">Re-index</button>
                    <button class="btn btn-sm" id="lib-channels-btn" title="Manage channels">Channels</button>
                </div>
            </div>
            <div class="library-detail-body">
                <div class="library-pane-wrap" id="lib-pane"></div>
                <div class="lib-info-panel-container" id="lib-info-panel"></div>
            </div>`;
        this.container.appendChild(el);

        el.querySelector('#lib-back').addEventListener('click', () => {
            this._pane = null;
            this._infoPanel = null;
            this.currentLibrary = null;
            this.render();
        });

        el.querySelector('#lib-reindex-btn').addEventListener('click', () => this._reindexDetail(el));
        el.querySelector('#lib-channels-btn').addEventListener('click', () => new ChannelSettingsModal().open());

        const publishBtn = el.querySelector('#lib-publish-btn');
        publishBtn.addEventListener('click', () => this._openPublishModal());

        const paneEl = el.querySelector('#lib-pane');
        const infoPanelEl = el.querySelector('#lib-info-panel');

        this._infoPanel = new InfoPanel(infoPanelEl);
        this._infoPanel.onToggle = () => {
            if (this._infoPanel.expanded && this._pane) {
                this._pane._notifyFocusChange();
            }
        };

        this._pane = new LibraryPane(paneEl, lib.id, {
            onImageClick: (path) => App.openViewer(path, this._pane),
            onFocusChange: (path) => this._onPhotoFocus(path),
            onToolInvoke: (params) => App.handleToolInvoke(params),
            onSlideshowInvoke: () => App.handleSlideshowInvoke(this._pane),
            onSelectionChange: (files) => {
                publishBtn.disabled = files.length === 0;
            },
        });

        this._pane.load(lib.relSourcePath || '');
    }

    async _openPublishModal() {
        const lib = this.currentLibrary;
        const pane = this._pane;
        if (!pane) return;

        const selectedPaths = pane.getSelectedFiles();
        if (selectedPaths.length === 0) return;

        let channels;
        try {
            channels = await ChannelAPI.list();
        } catch (err) {
            alert('Failed to load channels: ' + err.message);
            return;
        }
        if (channels.length === 0) {
            alert('No channels configured. Use the Channels button to add one.');
            return;
        }

        const now = new Date();
        const localISO = new Date(now - now.getTimezoneOffset() * 60000).toISOString().slice(0, 16);

        const dlg = document.createElement('div');
        dlg.className = 'modal-backdrop';
        dlg.innerHTML = `
            <div class="modal publish-modal">
                <div class="modal-header">
                    <span class="modal-title">Publish ${selectedPaths.length} photo${selectedPaths.length !== 1 ? 's' : ''}</span>
                    <button class="modal-close" id="pub-close">&times;</button>
                </div>
                <div class="modal-body">
                    <label class="form-label">Channel</label>
                    <select class="form-select" id="pub-channel">
                        ${channels.map(c => `<option value="${escapeHtml(c.slug)}">${escapeHtml(c.name)}</option>`).join('')}
                    </select>
                    <div id="pub-account-wrap" style="display:none">
                        <label class="form-label">Account</label>
                        <select class="form-select" id="pub-account"></select>
                    </div>
                    <label class="form-label">Date &amp; time</label>
                    <input class="form-input" id="pub-date" type="datetime-local" value="${localISO}">
                    <div class="publish-info" id="pub-info"></div>
                    ${selectedPaths.length > 1 ? `<div class="publish-group-note">${selectedPaths.length} photos will be grouped as one post (shared post ID in XMP).</div>` : ''}
                </div>
                <div class="modal-footer">
                    <div class="publish-error" id="pub-error" style="display:none"></div>
                    <button class="btn" id="pub-cancel">Cancel</button>
                    <button class="btn btn-accent" id="pub-confirm">Publish</button>
                </div>
            </div>`;
        document.body.appendChild(dlg);

        const updateChannel = () => {
            const slug = dlg.querySelector('#pub-channel').value;
            const ch = channels.find(c => c.slug === slug);
            if (!ch) return;

            // Account dropdown
            const accountWrap = dlg.querySelector('#pub-account-wrap');
            const accountSel  = dlg.querySelector('#pub-account');
            const accounts = ch.accounts || [];
            if (accounts.length > 0) {
                accountSel.innerHTML = accounts.map(a =>
                    `<option value="${escapeHtml(a.id)}">${escapeHtml(a.label || a.id)}</option>`
                ).join('');
                accountWrap.style.display = '';
            } else {
                accountWrap.style.display = 'none';
            }

            // Export summary
            const scaleDesc = _scaleDesc(ch.scale);
            const handlerNote = ch.handler ? ` · handler: ${ch.handler}` : '';
            dlg.querySelector('#pub-info').textContent =
                `Export: ${ch.format.toUpperCase()} · quality ${ch.quality}${scaleDesc ? ' · ' + scaleDesc : ''}${handlerNote}`;
        };
        dlg.querySelector('#pub-channel').addEventListener('change', updateChannel);
        updateChannel();

        dlg.querySelector('#pub-close').addEventListener('click', () => dlg.remove());
        dlg.querySelector('#pub-cancel').addEventListener('click', () => dlg.remove());
        dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });

        dlg.querySelector('#pub-confirm').addEventListener('click', async () => {
            const confirmBtn = dlg.querySelector('#pub-confirm');
            const errEl = dlg.querySelector('#pub-error');
            const channel = dlg.querySelector('#pub-channel').value;
            const dateVal = dlg.querySelector('#pub-date').value;
            const publishedAt = dateVal ? new Date(dateVal).toISOString() : new Date().toISOString();

            confirmBtn.disabled = true;
            confirmBtn.textContent = 'Publishing…';
            errEl.style.display = 'none';

            try {
                // Resolve filesystem paths → photo IDs
                const photoIDs = await Promise.all(
                    selectedPaths.map(p => LibraryAPI.photoIDByPath(lib.id, p))
                );
                const validIDs = photoIDs.filter(Boolean);
                if (validIDs.length === 0) throw new Error('No matching library photos found for selection.');

                const accountWrap = dlg.querySelector('#pub-account-wrap');
                const account = accountWrap.style.display !== 'none'
                    ? (dlg.querySelector('#pub-account').value || undefined)
                    : undefined;

                const { results } = await LibraryAPI.publish(lib.id, { photoIDs: validIDs, channel, account, publishedAt });
                const errors = (results || []).filter(r => r.error);
                dlg.remove();
                if (errors.length > 0) {
                    alert(`Published with ${errors.length} error(s):\n${errors.map(e => e.error).join('\n')}`);
                } else {
                    _showToast(`Published ${validIDs.length} photo${validIDs.length !== 1 ? 's' : ''} to ${channel}.`);
                }
            } catch (err) {
                errEl.textContent = err.message;
                errEl.style.display = '';
                confirmBtn.disabled = false;
                confirmBtn.textContent = 'Publish';
            }
        });
    }

    async _onPhotoFocus(path) {
        const lib = this.currentLibrary;
        const infoPanel = this._infoPanel;
        if (!infoPanel || !infoPanel.expanded) return;
        if (!path) { infoPanel.clear(); return; }

        infoPanel.loadInfo(path);

        try {
            const photoID = await LibraryAPI.photoIDByPath(lib.id, path);
            if (!photoID) { infoPanel.setMetaContext(null); return; }
            const entries = await LibraryAPI.getMeta(lib.id, photoID);
            infoPanel.setMetaContext({
                entries,
                onUpsert: (k, v) => LibraryAPI.upsertMeta(lib.id, photoID, k, v),
                onDelete: (k) => LibraryAPI.deleteMeta(lib.id, photoID, k),
                refresh: () => LibraryAPI.getMeta(lib.id, photoID),
            });
        } catch {
            infoPanel.setMetaContext(null);
        }
    }

    async _reindexDetail(detailEl) {
        const lib = this.currentLibrary;
        const btn = detailEl.querySelector('#lib-reindex-btn');
        btn.disabled = true;
        const statusEl = detailEl.querySelector('#lib-status');
        if (statusEl) statusEl.textContent = 'Indexing…';

        try {
            await LibraryAPI.reindex(lib.id, (p) => {
                if (!statusEl) return;
                if (p.finished) {
                    statusEl.textContent = `Done — ${p.total} photos`;
                } else {
                    statusEl.textContent = `Indexing ${p.done} / ${p.total}${p.current ? ' · ' + p.current : ''}`;
                }
            });
            this.currentLibrary = await LibraryAPI.get(lib.id);
            if (this._pane) this._pane.load(this._pane.path);
        } catch (err) {
            if (statusEl) statusEl.textContent = 'Error: ' + err.message;
        } finally {
            btn.disabled = false;
        }
    }
}

/* --- Utilities --- */

function stripQuotes(s) {
    if ((s.startsWith("'") && s.endsWith("'")) ||
        (s.startsWith('"') && s.endsWith('"'))) {
        return s.slice(1, -1);
    }
    return s;
}

function _showToast(msg) {
    const hint = document.getElementById('ui-hint');
    if (!hint) return;
    hint.textContent = msg;
    hint.classList.add('visible');
    clearTimeout(_showToast._timer);
    _showToast._timer = setTimeout(() => hint.classList.remove('visible'), 3000);
}

