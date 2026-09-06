// LibraryTab — Libraries mode: list, create, and browse photo libraries

/* --- Gallery date helpers --- */

// Returns a human-readable date range for a galleryListItem.
// "January 2026" · "January – March 2026" · "December 2025 – January 2026"
function _galleryDateRange(g) {
    const pub = new Date(g.publishedAt);
    const fmt = (d, opts) => d.toLocaleDateString('en', opts);
    const MY = { month: 'long', year: 'numeric' };
    const MO = { month: 'long' };
    if (!g.updatedAt || g.updatedAt.startsWith('0001-')) return fmt(pub, MY);
    const upd = new Date(g.updatedAt);
    if (upd.getFullYear() === pub.getFullYear() && upd.getMonth() === pub.getMonth()) return fmt(pub, MY);
    if (upd.getFullYear() === pub.getFullYear()) return fmt(pub, MO) + ' – ' + fmt(upd, MY);
    return fmt(pub, MY) + ' – ' + fmt(upd, MY);
}

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
    async update(id, name, description) {
        const r = await fetch(`/api/library/${id}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description })
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async setOrder(ids) {
        const r = await fetch('/api/library-order', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ order: ids })
        });
        if (!r.ok) throw new Error(await r.text());
    },
    async getSettings() {
        const r = await fetch('/api/settings');
        if (!r.ok) return {};
        return r.json();
    },
    async patchSettings(patch) {
        const r = await fetch('/api/settings', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(patch)
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async photos(id, { q = '', offset = 0, limit = 100, ...filters } = {}) {
        const params = new URLSearchParams({ offset, limit });
        if (q) params.set('q', q);
        for (const [k, v] of Object.entries(filters)) params.set(k, v);
        const r = await fetch(`/api/library/${id}/photos?${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async exifRanges(id) {
        const r = await fetch(`/api/library/${id}/exif-ranges`);
        if (!r.ok) return {};
        return r.json();
    },
    async globalExifRanges(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/exif-ranges${params}`);
        if (!r.ok) return {};
        return r.json();
    },
    async exifValues(field, ids) {
        const params = new URLSearchParams({ field });
        if (ids) params.set('ids', ids);
        const r = await fetch(`/api/library/exif-values?${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async metaKeys(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/meta-keys${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async metaValues(key, ids) {
        const params = new URLSearchParams({ key });
        if (ids) params.set('ids', ids);
        const r = await fetch(`/api/library/meta-values?${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async albumTitles(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/album-titles${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async exifFields(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/exif-fields${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async search({ ids, limit = 100, offset = 0, ...rest } = {}) {
        const params = new URLSearchParams({ limit, offset });
        if (ids) params.set('ids', ids);
        for (const [k, v] of Object.entries(rest)) params.set(k, v);
        const r = await fetch(`/api/library/search?${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async statistics(ids) {
        const params = ids?.length ? `?ids=${ids.join(',')}` : '';
        const r = await fetch(`/api/library/statistics${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async deletePhoto(libID, photoID) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}`, { method: 'DELETE' });
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
    async build(libID, { photoIDs, channel, account, publishedAt, recordXMP, outputPath }) {
        const r = await fetch(`/api/library/${libID}/build`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, channel, account, publishedAt, recordXMP, outputPath }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async buildStream(libID, { photoIDs, channel, account, publishedAt, galleryTitle, targetPostID, unlisted, recordXMP, outputPath }, onProgress) {
        const r = await fetch(`/api/library/${libID}/build`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, channel, account, publishedAt, galleryTitle, targetPostID, unlisted, recordXMP, outputPath }),
        });
        if (!r.ok) throw new Error(await r.text());

        const reader = r.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let finalEvt = null;

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
                    if (evt.complete) finalEvt = evt;
                    else if (onProgress) onProgress(evt);
                } catch { /* skip malformed */ }
            }
        }
        if (!finalEvt) throw new Error('Gallery stream ended without completion event');
        return finalEvt;
    },
    async buildDownload(libID, { photoIDs, channel, recordXMP }) {
        const r = await fetch(`/api/library/${libID}/build-download`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, channel, recordXMP }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.blob();
    },
    async collect(libID, slug, { photoIDs, draftID, postID, title, unlisted, account }) {
        const r = await fetch(`/api/library/${libID}/channels/${slug}/drafts`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, draftID, postID, title, unlisted, account }),
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
    reindex(id, onProgress, subfolder) {
        return new Promise((resolve, reject) => {
            const url = subfolder
                ? `/api/library/${id}/reindex?subfolder=${encodeURIComponent(subfolder)}`
                : `/api/library/${id}/reindex`;
            fetch(url, { method: 'POST' })
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
    cleanup(id, onProgress, subfolder) {
        return new Promise((resolve, reject) => {
            const url = subfolder
                ? `/api/library/${id}/cleanup?subfolder=${encodeURIComponent(subfolder)}`
                : `/api/library/${id}/cleanup`;
            fetch(url, { method: 'POST' })
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
    regenMissingPreviews(id, onProgress, subfolder) {
        return new Promise((resolve, reject) => {
            const url = subfolder
                ? `/api/library/${id}/regen-previews-missing?subfolder=${encodeURIComponent(subfolder)}`
                : `/api/library/${id}/regen-previews-missing`;
            fetch(url, { method: 'POST' })
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
    rebuildAllPreviews(id, onProgress, subfolder) {
        return new Promise((resolve, reject) => {
            const url = subfolder
                ? `/api/library/${id}/regen-previews-all?subfolder=${encodeURIComponent(subfolder)}`
                : `/api/library/${id}/regen-previews-all`;
            fetch(url, { method: 'POST' })
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
    scanNew(id, onProgress, subfolder) {
        return new Promise((resolve, reject) => {
            const url = subfolder
                ? `/api/library/${id}/scan-new?subfolder=${encodeURIComponent(subfolder)}`
                : `/api/library/${id}/scan-new`;
            fetch(url, { method: 'POST' })
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
        this._searchPane = null;
        this._listInfoPanel = null;
        this._listSearchPanel = null;
        this._cachedLibs = null;
        this._detailCollectBtn = null;
        this._listCollectBtn = null;
        this._detailEl = null;
    }

    getActivePaneForKeyboard() {
        if (this._searchPane && this._searchPane.container.style.display !== 'none') {
            return this._searchPane;
        }
        if (this._listSearchPanel?._searchPane) {
            return this._listSearchPanel._searchPane;
        }
        return this._pane;
    }

    _updateCommanderBtn() {
        const btn = this._detailEl && this._detailEl.querySelector('#lib-commander-btn');
        if (!btn) return;
        const pane = this.getActivePaneForKeyboard();
        const target = pane ? pane.getOpenInCommanderTarget() : null;
        if (!target) {
            btn.disabled = true;
            btn.title = pane ? pane.commanderBtnHint() : 'Select a folder to open in Organise view';
            return;
        }
        const boundary = (App.config && App.config.boundary)
            ? App.config.boundary.replace(/\/$/, '') : '';
        const sp = target.dir.replace(/\/$/, '');
        const withinBoundary = !boundary || sp === boundary || sp.startsWith(boundary + '/');
        btn.disabled = !withinBoundary;
        btn.title = withinBoundary
            ? 'Open selected photos in Commander'
            : 'Library folder is outside the server root — cannot open in Commander';
    }

    async _openStats() {
        const lib = this.currentLibrary;
        const folderPath = this._pane?.path || '';
        const libs = this._cachedLibs ?? await LibraryAPI.list();
        if (!lib) {
            new StatsModal().open(libs);
        } else {
            const pathPrefix = folderPath
                ? lib.sourcePath.replace(/\/$/, '') + '/' + folderPath
                : lib.sourcePath;
            new StatsModal().open(libs, {
                pathPrefix,
                libraryId: lib.id,
                fixedScope: true,
                scopeLabel: folderPath ? `${lib.name} / ${folderPath}` : lib.name,
            });
        }
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
        this._listInfoPanel = null;
        this._listSearchPanel = null;

        const el = document.createElement('div');
        el.className = 'library-list-view';
        el.innerHTML = `
            <div class="library-list-header">
                <h2 class="library-list-title">Libraries</h2>
                <div class="library-list-header-actions">
                    <button class="toggle lib-sort-toggle" role="switch" aria-checked="true" data-state="on" title="Sort order">
                        <span class="toggle-label">Sort by</span>
                        <span class="toggle-label toggle-label-on">recent additions</span>
                        <span class="toggle-track"><span class="toggle-thumb"></span></span>
                        <span class="toggle-label toggle-label-off">custom</span>
                    </button>
                    <div class="header-actions-sep"></div>
                    <button class="btn btn-sm" aria-pressed="false" data-state="off" id="lib-search-btn" title="Filter by EXIF values">Filter</button>
                    <div class="header-actions-sep"></div>
                    <button class="btn" id="lib-stats-btn">Statistics</button>
                    <div class="header-actions-sep"></div>
                    <button class="btn lib-collect-btn" id="lib-list-collect-btn" disabled>Add to channel…</button>
                    <div class="header-actions-sep"></div>
                    <button class="btn" id="lib-channels-btn">Channels ›</button>
                </div>
            </div>
            <div class="lib-search-body">
                <div class="lib-search-panel" id="lib-search-panel"></div>
                <div class="lib-search-content" id="lib-search-content">
                    <div class="lib-search-results-area" id="lib-search-results-area"></div>
                    <div class="lib-info-panel-container" id="lib-search-info-panel"></div>
                </div>
            </div>
            <div class="library-list-body" id="lib-list-body">
                <div class="library-loading">Loading…</div>
            </div>`;
        this.container.appendChild(el);

        el.querySelector('#lib-channels-btn').addEventListener('click', () => new ChannelSettingsModal().open(null));
        el.querySelector('#lib-stats-btn').addEventListener('click', () => this._openStats());

        const sortToggle = el.querySelector('.lib-sort-toggle');
        const body = el.querySelector('#lib-list-body');

        // Render immediately from localStorage, then reconcile with server
        const cachedMode = localStorage.getItem('library.sortMode') || 'auto';
        this._sortMode = cachedMode;
        sortToggle.dataset.state = cachedMode === 'auto' ? 'on' : 'off';
        sortToggle.setAttribute('aria-checked', cachedMode === 'auto' ? 'true' : 'false');
        LibraryAPI.getSettings().then(s => {
            const serverMode = s.librarySortMode || 'auto';
            if (serverMode !== this._sortMode) {
                this._sortMode = serverMode;
                localStorage.setItem('library.sortMode', serverMode);
                sortToggle.dataset.state = serverMode === 'auto' ? 'on' : 'off';
                sortToggle.setAttribute('aria-checked', serverMode === 'auto' ? 'true' : 'false');
                this._loadList(body);
            }
        }).catch(() => {});

        sortToggle.addEventListener('click', async () => {
            const newMode = sortToggle.dataset.state === 'on' ? 'manual' : 'auto';
            if (newMode === 'manual') await this._initManualOrder(this._cachedLibs || []);
            this._sortMode = newMode;
            localStorage.setItem('library.sortMode', newMode);
            LibraryAPI.patchSettings({ librarySortMode: newMode }); // fire-and-forget
            sortToggle.dataset.state = newMode === 'auto' ? 'on' : 'off';
            sortToggle.setAttribute('aria-checked', newMode === 'auto' ? 'true' : 'false');
            this._loadList(body);
        });

        this._listCollectBtn = el.querySelector('#lib-list-collect-btn');
        this._listCollectBtn.addEventListener('click', () => this._openCollectModal());

        const infoPanelEl = el.querySelector('#lib-search-info-panel');
        this._listInfoPanel = new InfoPanel(infoPanelEl);
        this._listInfoPanel.onToggle = () => {
            if (this._listInfoPanel.expanded && this._listSearchPanel?._searchPane) {
                this._listSearchPanel._searchPane._notifyFocusChange();
            }
        };

        this._listSearchPanel = new LibrarySearchPanel(
            el.querySelector('#lib-search-panel'),
            el.querySelector('#lib-search-btn'),
            null,
            {
                resultsContainer: el.querySelector('#lib-search-results-area'),
                onFocusChange: (path) => this._onListSearchFocus(path),
                onToolInvoke: (params) => App.handleToolInvoke({ ...params, sourcePath: null }),
                onSelectionChange: () => {
                    if (this._listCollectBtn) {
                        this._listCollectBtn.disabled =
                            (this._listSearchPanel?._searchPane?.selection.selected.size ?? 0) === 0;
                    }
                },
                onClose: () => { if (this._listInfoPanel) this._listInfoPanel.clear(); },
            }
        );

        this._loadList(el.querySelector('#lib-list-body'));
    }

    async _onListSearchFocus(path) {
        const infoPanel = this._listInfoPanel;
        if (!infoPanel || !infoPanel.expanded) return;
        if (!path) { infoPanel.clear(); return; }

        const pane = this._listSearchPanel?._searchPane;
        const info = pane ? pane.getPhotoInfo(path) : null;
        if (info) {
            infoPanel.loadFromURL(`/api/library/${info.libID}/photo/${info.photoID}/info`, `lib:${info.libID}:${info.photoID}`);
        } else {
            infoPanel.loadInfo(path);
        }
        if (!info) { infoPanel.setMetaContext(null); return; }

        try {
            const entries = await LibraryAPI.getMeta(info.libID, info.photoID);
            infoPanel.setMetaContext({
                entries,
                onUpsert: (k, v) => LibraryAPI.upsertMeta(info.libID, info.photoID, k, v),
                onDelete: (k) => LibraryAPI.deleteMeta(info.libID, info.photoID, k),
                refresh: () => LibraryAPI.getMeta(info.libID, info.photoID),
            });
        } catch {
            infoPanel.setMetaContext(null);
        }
    }

    async _loadList(body) {
        try {
            const libs = await LibraryAPI.list();
            this._cachedLibs = libs;
            const prevLastSeen = parseInt(localStorage.getItem('library.lastOverviewVisit') || '0', 10);
            localStorage.setItem('library.lastOverviewVisit', Date.now().toString());
            body.innerHTML = '';
            if (libs.length === 0) {
                body.innerHTML = '<div class="library-empty">No libraries yet. Create one to get started.</div>';
                return;
            }
            const sorted = this._sortLibs(libs);
            const mode = this._sortMode || localStorage.getItem('library.sortMode') || 'auto';
            for (const lib of sorted) {
                const card = this._libCard(lib, prevLastSeen);
                body.appendChild(card);
                if (lib.scanning) {
                    this._runScanCard(lib, card, (id, cb) => LibraryAPI.scanNew(id, cb), 'Scanning');
                }
                if (mode === 'manual') this._addManualSortButtons(lib, card, body);
            }
            // Update sort toggle active state
            const sortToggle = body.closest('.library-list-view')?.querySelector('.lib-sort-toggle');
            if (sortToggle) {
                sortToggle.dataset.state = mode === 'auto' ? 'on' : 'off';
                sortToggle.setAttribute('aria-checked', mode === 'auto' ? 'true' : 'false');
            }
        } catch (err) {
            body.innerHTML = `<div class="library-error">Failed to load libraries: ${err.message}</div>`;
        }
    }

    _libCard(lib, lastSeen = parseInt(localStorage.getItem('library.lastOverviewVisit') || '0', 10)) {
        const hasNew = lib.lastNewPhotos && new Date(lib.lastNewPhotos).getTime() > lastSeen;

        const card = document.createElement('div');
        card.className = 'library-card';
        const lastIdx = lib.lastIndexed
            ? new Date(lib.lastIndexed).toLocaleDateString()
            : 'Never';

        const top = document.createElement('div');
        top.className = 'library-card-top';
        top.innerHTML = `
            <div class="library-card-info">
                <div class="library-card-name">${escapeHtml(lib.name)}${hasNew ? '<span class="library-card-new-dot" title="New photos added"></span>' : ''}</div>
                <div class="library-card-meta">${escapeHtml(lib.sourcePath)}</div>
                <div class="library-card-stats">${lib.photoCount} photos · Last indexed: ${lastIdx}</div>
                ${lib.description ? `<div class="library-card-desc">${escapeHtml(lib.description)}</div>` : ''}
            </div>
            <div class="library-card-actions">
                <button class="btn btn-sm btn-accent lib-open">Open</button>
                <button class="btn btn-sm lib-edit" aria-label="Edit library">Edit</button>
                <div class="dropdown-wrap lib-scan-wrap">
                    <div class="dropdown-toggle">
                        <button class="btn btn-sm lib-scan-new">Scan for new photos</button>
                        <button class="btn btn-sm lib-scan-toggle" aria-label="More scan options"><svg width="8" height="8" viewBox="0 0 8 8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3l2 2 2-2"/></svg></button>
                    </div>
                    <div class="dropdown-menu lib-scan-menu dropdown-menu-right" style="display:none">
                        <button class="btn dropdown-item lib-reindex">Rebuild metadata &amp; previews</button>
                        <button class="btn dropdown-item lib-regen-missing">Generate missing previews</button>
                        <button class="btn dropdown-item lib-rebuild-all">Rebuild all previews</button>
                        <button class="btn dropdown-item lib-cleanup">Remove deleted photos</button>
                    </div>
                </div>
                <button class="btn btn-sm lib-delete">Delete</button>
            </div>`;
        card.appendChild(top);

        if (lib.photoCount > 0) {
            const strip = document.createElement('div');
            strip.className = 'library-card-filmstrip';
            card.appendChild(strip);
            this._loadFilmstrip(strip, lib.id);
        }

        top.querySelector('.lib-open').addEventListener('click', () => this._openLibrary(lib));
        top.querySelector('.lib-edit').addEventListener('click', () => this._showEditDialog(lib, card));
        card.querySelector('.lib-delete').addEventListener('click', () => this._deleteLibrary(lib, card));
        const scanWrap = card.querySelector('.lib-scan-wrap');
        const scanMenu = scanWrap.querySelector('.lib-scan-menu');
        const { close: closeScanMenu } = Dropdown.init(scanWrap.querySelector('.lib-scan-toggle'), scanMenu);
        scanWrap.querySelector('.lib-scan-new').addEventListener('click', () => this._runScanCard(lib, card, (id, cb) => LibraryAPI.scanNew(id, cb), 'Scanning'));
        scanWrap.querySelector('.lib-reindex').addEventListener('click', () => {
            closeScanMenu();
            this._runScanCard(lib, card, (id, cb) => LibraryAPI.reindex(id, cb), 'Indexing');
        });
        scanWrap.querySelector('.lib-regen-missing').addEventListener('click', () => {
            closeScanMenu();
            this._runScanCard(lib, card, (id, cb) => LibraryAPI.regenMissingPreviews(id, cb), 'Generating');
        });
        scanWrap.querySelector('.lib-rebuild-all').addEventListener('click', () => {
            closeScanMenu();
            this._runScanCard(lib, card, (id, cb) => LibraryAPI.rebuildAllPreviews(id, cb), 'Rebuilding');
        });
        scanWrap.querySelector('.lib-cleanup').addEventListener('click', () => {
            closeScanMenu();
            this._runScanCard(lib, card, (id, cb) => LibraryAPI.cleanup(id, cb), 'Checking');
        });
        return card;
    }

    async _loadFilmstrip(strip, libID) {
        const result = await LibraryAPI.photos(libID, { limit: 50 });
        for (const photo of result.photos) {
            const img = document.createElement('img');
            img.src = LibraryAPI.thumbURL(libID, photo.id);
            img.loading = 'lazy';
            strip.appendChild(img);
        }
    }

    async _runScanCard(lib, card, scanFn, label) {
        const progressEl = card.querySelector('.library-card-progress') || (() => {
            const p = document.createElement('div');
            p.className = 'library-card-progress';
            card.querySelector('.library-card-actions').appendChild(p);
            return p;
        })();
        const scanBtns = card.querySelectorAll('.lib-scan-new, .lib-scan-toggle');
        scanBtns.forEach(b => { b.disabled = true; });
        progressEl.textContent = `${label}…`;
        try {
            await scanFn(lib.id, (p) => {
                if (p.finished) {
                    progressEl.textContent = `Done — ${p.total} photos.`;
                } else {
                    const loc = p.current
                        ? p.current + (p.parent ? ` in "${p.parent}"` : '') + ' · '
                        : '';
                    progressEl.textContent = `${loc}${p.done}/${p.total}`;
                }
            });
            const updated = await LibraryAPI.get(lib.id);
            card.querySelector('.library-card-stats').textContent =
                `${updated.photoCount} photos · Last indexed: ${new Date(updated.lastIndexed).toLocaleDateString()}`;
        } catch (err) {
            progressEl.textContent = `Error: ${err.message}`;
        } finally {
            scanBtns.forEach(b => { b.disabled = false; });
        }
    }

    async _deleteLibrary(lib, card) {
        if (!confirm(`Delete library "${lib.name}"?\n\nThis removes the index and thumbnails. Your original photos are not affected.`)) return;
        try {
            await LibraryAPI.delete(lib.id);
            this._cachedLibs = null;
            card.remove();
            App.refreshLibraryVisibility();
        } catch (err) {
            alert('Delete failed: ' + err.message);
        }
    }

    _sortLibs(libs) {
        const mode = this._sortMode || localStorage.getItem('library.sortMode') || 'auto';
        if (mode === 'manual') {
            return [...libs].sort((a, b) => {
                const ai = a.sortPosition ?? Infinity;
                const bi = b.sortPosition ?? Infinity;
                return ai - bi;
            });
        }
        // auto: sort by lastNewPhotos descending, nulls last
        return [...libs].sort((a, b) => {
            if (!a.lastNewPhotos && !b.lastNewPhotos) return 0;
            if (!a.lastNewPhotos) return 1;
            if (!b.lastNewPhotos) return -1;
            return new Date(b.lastNewPhotos) - new Date(a.lastNewPhotos);
        });
    }

    async _initManualOrder(libs) {
        if (!libs || !libs.length) return;
        if (libs.some(l => l.sortPosition != null)) return; // already seeded server-side
        // Seed from current auto order
        const sorted = [...libs].sort((a, b) => {
            if (!a.lastNewPhotos && !b.lastNewPhotos) return 0;
            if (!a.lastNewPhotos) return 1;
            if (!b.lastNewPhotos) return -1;
            return new Date(b.lastNewPhotos) - new Date(a.lastNewPhotos);
        });
        await LibraryAPI.setOrder(sorted.map(l => l.id));
    }

    _addManualSortButtons(lib, card, body) {
        const up = document.createElement('button');
        up.className = 'btn btn-sm lib-move-up';
        up.textContent = '↑';
        up.title = 'Move up';
        const down = document.createElement('button');
        down.className = 'btn btn-sm lib-move-down';
        down.textContent = '↓';
        down.title = 'Move down';
        card.querySelector('.library-card-actions').appendChild(up);
        card.querySelector('.library-card-actions').appendChild(down);

        up.addEventListener('click', async () => {
            const order = this._sortLibs(this._cachedLibs || []).map(l => l.id);
            const i = order.indexOf(lib.id);
            if (i > 0) {
                [order[i - 1], order[i]] = [order[i], order[i - 1]];
                await LibraryAPI.setOrder(order);
                this._loadList(body);
            }
        });
        down.addEventListener('click', async () => {
            const order = this._sortLibs(this._cachedLibs || []).map(l => l.id);
            const i = order.indexOf(lib.id);
            if (i !== -1 && i < order.length - 1) {
                [order[i], order[i + 1]] = [order[i + 1], order[i]];
                await LibraryAPI.setOrder(order);
                this._loadList(body);
            }
        });
    }

    _showEditDialog(lib, card) {
        const dlg = document.createElement('div');
        dlg.className = 'library-dialog-backdrop';
        dlg.innerHTML = `
            <div class="library-dialog">
                <h3 class="library-dialog-title">Edit Library</h3>
                <label class="library-dialog-label">Name</label>
                <input class="library-dialog-input" id="lib-edit-name" type="text" autocomplete="off">
                <label class="library-dialog-label">Description (optional)</label>
                <input class="library-dialog-input" id="lib-edit-desc" type="text">
                <div class="library-dialog-actions">
                    <button class="btn" id="lib-edit-cancel">Cancel</button>
                    <button class="btn btn-accent" id="lib-edit-save">Save</button>
                </div>
            </div>`;
        document.body.appendChild(dlg);

        const nameEl = dlg.querySelector('#lib-edit-name');
        const descEl = dlg.querySelector('#lib-edit-desc');
        nameEl.value = lib.name;
        descEl.value = lib.description || '';
        nameEl.focus();

        const close = () => dlg.remove();
        dlg.querySelector('#lib-edit-cancel').addEventListener('click', close);
        dlg.querySelector('#lib-edit-save').addEventListener('click', async () => {
            const name = nameEl.value.trim();
            if (!name) { nameEl.focus(); return; }
            try {
                const updated = await LibraryAPI.update(lib.id, name, descEl.value.trim());
                lib.name = updated.name;
                lib.description = updated.description;
                card.querySelector('.library-card-name').firstChild.textContent = updated.name;
                if (updated.description) {
                    let descEl2 = card.querySelector('.library-card-desc');
                    if (!descEl2) {
                        descEl2 = document.createElement('div');
                        descEl2.className = 'library-card-desc';
                        card.querySelector('.library-card-stats').after(descEl2);
                    }
                    descEl2.textContent = updated.description;
                } else {
                    card.querySelector('.library-card-desc')?.remove();
                }
                close();
            } catch (err) {
                alert('Save failed: ' + err.message);
            }
        });
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
                <input class="library-dialog-input" id="lib-dlg-path" type="text" placeholder="/Fotos/2024">
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
                this._cachedLibs = null;
                progressEl.textContent = 'Indexing photos…';
                await LibraryAPI.reindex(lib.id, (p) => {
                    if (p.finished) {
                        progressEl.textContent = `Done — ${p.total} photos indexed.`;
                    } else {
                        progressEl.textContent = `${p.done} / ${p.total}${p.current ? ' · ' + p.current : ''}`;
                    }
                });
                dlg.remove();
                App.refreshLibraryVisibility();
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
        this._searchPane = null;

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
                    <button class="btn btn-sm lib-commander-btn" id="lib-commander-btn" disabled title="Select photos to open in Commander">Organise: jump to folder</button>
                    <button class="btn btn-sm" aria-pressed="false" data-state="off" id="lib-filter-btn" title="Filter by EXIF values">Filter</button>
                    <button class="btn btn-sm" id="lib-detail-stats-btn">Statistics</button>
                    <button class="btn btn-sm lib-collect-btn" id="lib-collect-btn" disabled>Add to channel…</button>
                    <button class="btn btn-sm" id="lib-channels-btn" title="Manage channels">Channels ›</button>
                </div>
            </div>
            <div class="lib-search-body">
                <div class="lib-search-panel" id="lib-search-panel"></div>
                <div class="library-detail-body">
                    <div class="library-pane-wrap" id="lib-pane"></div>
                    <div class="library-pane-wrap" id="lib-search-pane" style="display:none"></div>
                    <div class="lib-info-panel-container" id="lib-info-panel"></div>
                </div>
            </div>`;
        this.container.appendChild(el);
        this._detailEl = el;
        this._detailCollectBtn = el.querySelector('#lib-collect-btn');

        el.querySelector('#lib-back').addEventListener('click', () => {
            this._pane = null;
            this._infoPanel = null;
            this._searchPane = null;
            this.currentLibrary = null;
            this.render();
        });

        el.querySelector('#lib-channels-btn').addEventListener('click', () => new ChannelSettingsModal().open(lib.id));
        el.querySelector('#lib-detail-stats-btn').addEventListener('click', () => this._openStats());

        const collectBtn = el.querySelector('#lib-collect-btn');
        collectBtn.addEventListener('click', () => this._openCollectModal());

        const commanderBtn = el.querySelector('#lib-commander-btn');
        commanderBtn.addEventListener('click', () => {
            const pane = this.getActivePaneForKeyboard();
            const target = pane ? pane.getOpenInCommanderTarget() : null;
            if (!target) return;
            App.openCommanderAt(target.dir, target.names);
        });

        this._filterPanel = new LibrarySearchPanel(
            el.querySelector('#lib-search-panel'),
            el.querySelector('#lib-filter-btn'),
            lib.id,
            {
                onResults: (photos, multiLib, paginationOpts) => this._showSearchResults(el, photos, multiLib, paginationOpts),
                onClose: () => this._showLibraryPane(el),
                onLoading: (isLoading) => {
                    const paneEl = el.querySelector('#lib-pane');
                    const searchPaneEl = el.querySelector('#lib-search-pane');
                    if (isLoading && paneEl.style.display !== 'none') {
                        paneEl.style.display = 'none';
                        if (!this._searchPane) {
                            searchPaneEl.innerHTML = '<div class="lib-results-spinner-wrap"><div class="lib-results-spinner"></div></div>';
                        }
                        searchPaneEl.style.display = '';
                    } else if (!isLoading) {
                        searchPaneEl.querySelector('.lib-results-spinner-wrap')?.remove();
                    }
                },
            }
        );

        const paneEl = el.querySelector('#lib-pane');
        const infoPanelEl = el.querySelector('#lib-info-panel');

        this._infoPanel = new InfoPanel(infoPanelEl);
        this._infoPanel.onToggle = () => {
            if (this._infoPanel.expanded) {
                const activePane = this._searchPane || this._pane;
                if (activePane) activePane._notifyFocusChange();
            }
        };
        this._infoPanel.onDirNavigate = (subPath) => {
            if (this._pane) this._pane.load(subPath);
        };

        this._pane = new LibraryPane(paneEl, lib.id, {
            sourcePath: lib.sourcePath,
            onImageClick: (path) => App.openViewer(path, this._pane),
            onFocusChange: (path, type) => {
                if (type === 'dir') this._onDirFocus(path);
                else this._onPhotoFocus(path);
            },
            onToolInvoke: (params) => App.handleToolInvoke({ ...params, sourcePath: lib.sourcePath }),
            onSlideshowInvoke: () => App.handleSlideshowInvoke(this._pane),
            onSelectionChange: () => {
                this._updateDetailCollectBtn();
                this._updateCommanderBtn();
            },
        });

        this._pane.load('');
    }

    _showSearchResults(detailEl, photos, multiLib, paginationOpts) {
        const paneEl = detailEl.querySelector('#lib-pane');
        const searchPaneEl = detailEl.querySelector('#lib-search-pane');

        if (!this._searchPane) {
            this._searchPane = new SearchResultPane(searchPaneEl, {
                onImageClick: (path) => App.openViewer(path, this._searchPane),
                onFocusChange: (path) => this._onPhotoFocusFromSearch(path),
                onSlideshowInvoke: () => App.handleSlideshowInvoke(this._searchPane),
                onToolInvoke: (params) => App.handleToolInvoke({ ...params, sourcePath: this.currentLibrary?.sourcePath || null }),
                onSelectionChange: () => { this._updateDetailCollectBtn(); this._updateCommanderBtn(); },
                onClose: () => this._filterPanel.close(),
            });
        }

        this._searchPane.loadResults(photos, multiLib, paginationOpts);
        paneEl.style.display = 'none';
        searchPaneEl.style.display = '';
        this._updateDetailCollectBtn();
        this._updateCommanderBtn();
    }

    _showLibraryPane(detailEl) {
        const paneEl = detailEl.querySelector('#lib-pane');
        const searchPaneEl = detailEl.querySelector('#lib-search-pane');

        searchPaneEl.style.display = 'none';
        paneEl.style.display = '';
        // Keep _searchPane alive so it can be reused if the filter is reopened.

        this._updateDetailCollectBtn();
        this._updateCommanderBtn();
    }

    async _onPhotoFocusFromSearch(path) {
        const infoPanel = this._infoPanel;
        if (!infoPanel || !infoPanel.expanded) return;
        if (!path) { infoPanel.clear(); return; }

        const info = this._searchPane ? this._searchPane.getPhotoInfo(path) : null;
        if (info) {
            infoPanel.loadFromURL(`/api/library/${info.libID}/photo/${info.photoID}/info`, `lib:${info.libID}:${info.photoID}`);
        } else {
            infoPanel.loadInfo(path);
        }
        if (!info) { infoPanel.setMetaContext(null); return; }

        try {
            const entries = await LibraryAPI.getMeta(info.libID, info.photoID);
            infoPanel.setMetaContext({
                entries,
                onUpsert: (k, v) => LibraryAPI.upsertMeta(info.libID, info.photoID, k, v),
                onDelete: (k) => LibraryAPI.deleteMeta(info.libID, info.photoID, k),
                refresh: () => LibraryAPI.getMeta(info.libID, info.photoID),
            });
        } catch {
            infoPanel.setMetaContext(null);
        }
    }

    _updateDetailCollectBtn() {
        if (!this._detailCollectBtn) return;
        const searchPaneEl = this._detailEl?.querySelector('#lib-search-pane');
        const searchActive = searchPaneEl && searchPaneEl.style.display !== 'none';
        if (searchActive) {
            this._detailCollectBtn.disabled = (this._searchPane?.selection.selected.size ?? 0) === 0;
        } else {
            this._detailCollectBtn.disabled =
                (this._pane?.getSelectedFiles().length ?? 0) === 0 &&
                !(this._pane?.selectedDirs?.size);
        }
    }

    async _openCollectModal() {
        const searchPane = (() => {
            if (this._searchPane && this._detailEl?.querySelector('#lib-search-pane')?.style.display !== 'none'
                && this._searchPane.selection.selected.size > 0) return this._searchPane;
            if (this._listSearchPanel?._searchPane?.selection.selected.size > 0)
                return this._listSearchPanel._searchPane;
            return null;
        })();

        let lib = this.currentLibrary;
        let selectedPaths, hasDirs, selectedDirs = [];
        let photoGroups = null;

        if (searchPane) {
            const hints = searchPane.getSelectedFiles();
            const byLib = new Map();
            for (const h of hints) {
                const info = searchPane.getPhotoInfo(h);
                if (!info) continue;
                if (!byLib.has(info.libID)) byLib.set(info.libID, []);
                byLib.get(info.libID).push(info.photoID);
            }
            photoGroups = Array.from(byLib.entries()).map(([libID, photoIDs]) => ({ libID, photoIDs }));
            selectedPaths = hints;
            hasDirs = false;
        } else {
            const pane = this._pane;
            if (!pane) return;
            const selectedFiles = pane.getSelectedFiles();
            selectedDirs = Array.from(pane.selectedDirs || []);
            hasDirs = selectedDirs.length > 0;
            if (selectedFiles.length === 0 && !hasDirs) return;
            selectedPaths = hasDirs ? [] : selectedFiles;
        }

        let channelList;
        try {
            channelList = await ChannelAPI.list();
        } catch (err) {
            alert('Failed to load channels: ' + err.message);
            return;
        }
        if (channelList.length === 0) {
            alert('No channels configured. Use the Channels button to add one.');
            return;
        }

        const initialTitle = hasDirs ? 'Counting photos…'
            : `Add ${selectedPaths.length} photo${selectedPaths.length !== 1 ? 's' : ''} to a channel`;

        const dlg = document.createElement('div');
        dlg.className = 'modal-backdrop';
        dlg.innerHTML = `
            <div class="modal collect-modal">
                <div class="modal-header">
                    <span class="modal-title">${initialTitle}</span>
                    <button class="modal-close" id="collect-close">&times;</button>
                </div>
                <div class="modal-body">
                    <label class="form-label">Channel</label>
                    <select class="form-select" id="collect-channel">
                        ${channelList.map(c => `<option value="${escapeHtml(c.slug)}">${escapeHtml(c.name)}</option>`).join('')}
                    </select>
                    <div id="collect-account-wrap" style="display:none">
                        <label class="form-label">Account</label>
                        <select class="form-select" id="collect-account"></select>
                    </div>
                    <div id="collect-gallery-wrap" style="display:none">
                        <label class="form-label">Add to</label>
                        <select class="form-select" id="collect-target">
                            <option value="">New…</option>
                        </select>
                        <div id="collect-title-wrap">
                            <label class="form-label" id="collect-gallery-label">Gallery title</label>
                            <input class="form-input" id="collect-gallery-title" placeholder="e.g. Summer 2026" autocomplete="off">
                            <div id="collect-unlisted-wrap" style="display:none">
                                <label class="export-radio-row">
                                    <input type="checkbox" id="collect-unlisted">
                                    Unlisted (not listed on the site index or sitemap — reachable only via direct link)
                                </label>
                            </div>
                        </div>
                    </div>
                    <div class="build-info" id="collect-info"></div>
                </div>
                <div class="modal-footer">
                    <div class="build-error" id="collect-error" style="display:none"></div>
                    <button class="btn" id="collect-cancel">Cancel</button>
                    <button class="btn btn-accent" id="collect-confirm"${hasDirs ? ' disabled' : ''}>Add to channel</button>
                </div>
            </div>`;
        document.body.appendChild(dlg);

        if (hasDirs) {
            Promise.all(selectedDirs.map(d => this._pane.fetchRecursivePhotoPaths(d)))
                .then(arrays => {
                    selectedPaths = arrays.flat();
                    if (dlg.isConnected) {
                        dlg.querySelector('.modal-title').textContent =
                            `Add ${selectedPaths.length} photo${selectedPaths.length !== 1 ? 's' : ''} to a channel`;
                        dlg.querySelector('#collect-confirm').disabled = selectedPaths.length === 0;
                    }
                });
        }

        // draftsBySlug caches ChannelAPI.listDrafts() results per channel so the
        // "Add to" dropdown can offer in-progress drafts alongside already-generated
        // galleries without refetching on every channel switch.
        const draftsBySlug = new Map();

        const updateChannel = async () => {
            const slug = dlg.querySelector('#collect-channel').value;
            const ch = channelList.find(c => c.slug === slug);
            if (!ch) return;

            const accountWrap = dlg.querySelector('#collect-account-wrap');
            const accountSel = dlg.querySelector('#collect-account');
            const accounts = ch.accounts || [];
            if (accounts.length > 0) {
                accountSel.innerHTML = accounts.map(a => `<option value="${escapeHtml(a.id)}">${escapeHtml(a.label || a.id)}</option>`).join('');
                accountWrap.style.display = '';
            } else {
                accountWrap.style.display = 'none';
            }

            const galleryWrap = dlg.querySelector('#collect-gallery-wrap');
            const targetSel = dlg.querySelector('#collect-target');
            const titleWrap = dlg.querySelector('#collect-title-wrap');
            const titleLabel = dlg.querySelector('#collect-gallery-label');
            titleLabel.textContent = ch.siteExport ? 'Album title' : 'Gallery title';

            if (ch.galleryExport || ch.siteExport) {
                galleryWrap.style.display = '';
                let generated = [];
                let drafts = draftsBySlug.get(slug);
                try {
                    generated = await ChannelAPI.galleries(slug);
                    if (!drafts) {
                        drafts = await ChannelAPI.listDrafts(slug);
                        draftsBySlug.set(slug, drafts);
                    }
                } catch { /* fall back to New-only */ }
                const generatedOpts = generated.map(g =>
                    `<option value="postID:${escapeHtml(g.postID)}">${escapeHtml(g.title)} (${g.photoCount}) · ${_galleryDateRange(g)}</option>`
                );
                const draftOpts = drafts.filter(d => !d.target.postID).map(d =>
                    `<option value="draftID:${escapeHtml(d.id)}">${escapeHtml(d.target.title)} (draft, ${d.photos.length} pending)</option>`
                );
                targetSel.innerHTML = `<option value="">New ${ch.siteExport ? 'album' : 'gallery'}</option>` + generatedOpts.join('') + draftOpts.join('');
                targetSel.onchange = () => {
                    titleWrap.style.display = targetSel.value === '' ? '' : 'none';
                };
                titleWrap.style.display = targetSel.value === '' ? '' : 'none';
            } else {
                galleryWrap.style.display = 'none';
            }

            const unlistedWrap = dlg.querySelector('#collect-unlisted-wrap');
            unlistedWrap.style.display = ch.siteExport ? '' : 'none';
            dlg.querySelector('#collect-unlisted').checked = false;

            const scaleDesc = _scaleDesc(ch.scale);
            const handlerNote = ch.handler ? ` · handler: ${ch.handler}` : '';
            dlg.querySelector('#collect-info').textContent =
                `Export: ${ch.format.toUpperCase()} · quality ${ch.quality}${scaleDesc ? ' · ' + scaleDesc : ''}${handlerNote}`;
        };

        dlg.querySelector('#collect-channel').addEventListener('change', updateChannel);
        updateChannel();

        dlg.querySelector('#collect-close').addEventListener('click', () => dlg.remove());
        dlg.querySelector('#collect-cancel').addEventListener('click', () => dlg.remove());
        dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });

        dlg.querySelector('#collect-confirm').addEventListener('click', async () => {
            const confirmBtn = dlg.querySelector('#collect-confirm');
            const errEl = dlg.querySelector('#collect-error');
            const slug = dlg.querySelector('#collect-channel').value;
            const ch = channelList.find(c => c.slug === slug);
            const targetVal = dlg.querySelector('#collect-target')?.value || '';
            const [targetKind, targetID] = targetVal.includes(':') ? targetVal.split(':') : [null, null];
            const titleWrap = dlg.querySelector('#collect-title-wrap');
            const galleryTitle = (titleWrap && titleWrap.style.display !== 'none')
                ? dlg.querySelector('#collect-gallery-title').value.trim()
                : undefined;
            const isGalleryMode = ch?.galleryExport || ch?.siteExport;
            if (isGalleryMode && !targetKind && !galleryTitle) {
                errEl.textContent = ch.siteExport ? 'Album title is required for a new album.' : 'Gallery title is required for a new gallery.';
                errEl.style.display = '';
                return;
            }
            const accountWrap = dlg.querySelector('#collect-account-wrap');
            const account = accountWrap.style.display !== 'none' ? (dlg.querySelector('#collect-account').value || undefined) : undefined;
            const unlisted = (ch?.siteExport && !targetKind) ? dlg.querySelector('#collect-unlisted').checked : undefined;

            confirmBtn.disabled = true;
            confirmBtn.textContent = 'Adding…';
            errEl.style.display = 'none';

            try {
                let validGroups;
                if (photoGroups) {
                    validGroups = photoGroups.filter(g => g.photoIDs.length > 0);
                    if (validGroups.length === 0) throw new Error('No matching library photos found.');
                } else {
                    const photoIDs = await Promise.all(selectedPaths.map(p => LibraryAPI.photoIDByPath(lib.id, p)));
                    validGroups = [{ libID: lib.id, photoIDs: photoIDs.filter(Boolean) }];
                    if (validGroups[0].photoIDs.length === 0) throw new Error('No matching library photos found for selection.');
                }

                let total = 0;
                for (const g of validGroups) {
                    await LibraryAPI.collect(g.libID, slug, {
                        photoIDs: g.photoIDs,
                        draftID: targetKind === 'draftID' ? targetID : undefined,
                        postID: targetKind === 'postID' ? targetID : undefined,
                        title: !targetKind ? galleryTitle : undefined,
                        unlisted, account,
                    });
                    total += g.photoIDs.length;
                }
                dlg.remove();
                App.showToast(`Added ${total} photo${total !== 1 ? 's' : ''} to "${galleryTitle || ch.name}" — not yet published.`);
            } catch (err) {
                errEl.textContent = err.message;
                errEl.style.display = '';
                confirmBtn.disabled = false;
                confirmBtn.textContent = 'Add to channel';
            }
        });
    }

    async _onPhotoFocus(path) {
        const lib = this.currentLibrary;
        const infoPanel = this._infoPanel;
        if (!infoPanel || !infoPanel.expanded) return;
        if (!path) { infoPanel.clear(); return; }

        let info = this._pane?._photoMap?.get(path);
        if (!info) {
            console.warn('[library] _photoMap miss for', path, 'mapSize:', this._pane?._photoMap?.size);
            const photoID = await LibraryAPI.photoIDByPath(lib.id, path);
            if (!photoID) { infoPanel.clear(); return; }
            info = { photoID };
        }

        infoPanel.loadFromURL(`/api/library/${lib.id}/photo/${info.photoID}/info`, `lib:${lib.id}:${info.photoID}`);

        try {
            const entries = await LibraryAPI.getMeta(lib.id, info.photoID);
            infoPanel.setMetaContext({
                entries,
                onUpsert: (k, v) => LibraryAPI.upsertMeta(lib.id, info.photoID, k, v),
                onDelete: (k) => LibraryAPI.deleteMeta(lib.id, info.photoID, k),
                refresh: () => LibraryAPI.getMeta(lib.id, info.photoID),
            });
        } catch {
            infoPanel.setMetaContext(null);
        }
    }

    _onDirFocus(path) {
        const lib = this.currentLibrary;
        const infoPanel = this._infoPanel;
        if (!infoPanel || !infoPanel.expanded) return;
        if (!path) { infoPanel.clear(); return; }
        const sourcePath = lib.sourcePath.replace(/\/$/, '');
        const pathPrefix = path ? sourcePath + '/' + path : sourcePath;
        infoPanel.loadFolderInfo(path, { libId: lib.id, pathPrefix });
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


