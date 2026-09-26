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

/* --- Library job progress --- */

// Library jobs (scan, index, previews, cleanup) all report
// {done, total, current, parent, finished, error}. An error also arrives with
// `finished`, so it is checked first.
function showLibraryProgress(activity, p, busyText) {
    if (p.error) return activity.fail(`It stopped: ${p.error}`);
    if (p.finished) return activity.done(`Finished. ${formatCount(p.total)} photo${p.total !== 1 ? 's' : ''}.`);
    if (!p.total) return activity.busy(busyText);
    const current = p.current ? (p.parent ? `${p.parent}/${p.current}` : p.current) : '';
    return activity.count(p.done, p.total, { noun: 'photos', current });
}

/* --- LibraryTab --- */

class LibraryTab {
    constructor(container) {
        this.container = container;
        this.currentLibrary = null;
        this._pane = null;
        this._infoPanel = null;
        this._filterPanel = null;
        this._resultsPane = null;
        this._cachedLibs = null;
    }

    // The filter's results while they are on screen, otherwise the open
    // library's folders — in the overview that is nothing.
    getActivePaneForKeyboard() {
        if (this._resultsPane && this._resultsPane.container.style.display !== 'none') {
            return this._resultsPane;
        }
        return this._pane;
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
        const el = document.createElement('div');
        el.className = 'library-list-view';
        el.innerHTML = `
            <div class="library-list-header">
                ${this._filterButtonHTML()}
                <h2 class="library-list-title">Libraries</h2>
                <div class="library-list-header-actions">
                    <select class="btn btn-sm select-btn lib-sort-select" aria-label="Sort libraries">
                        <option value="auto">Recently added</option>
                        <option value="name">Name</option>
                        <option value="manual">Custom order</option>
                    </select>
                    <div class="header-actions-sep"></div>
                    <button class="btn btn-sm" id="lib-stats-btn">Statistics</button>
                    <div class="header-actions-sep"></div>
                    <button class="btn btn-sm" id="lib-new-btn">New library…</button>
                </div>
            </div>
            ${this._filterBodyHTML(`
                <div class="library-list-body" id="lib-list-body"></div>`)}`;
        this.container.appendChild(el);

        el.querySelector('#lib-new-btn').addEventListener('click', () => this._showCreateDialog());
        el.querySelector('#lib-stats-btn').addEventListener('click', () => this._openStats());

        const sortSelect = el.querySelector('.lib-sort-select');
        const body = el.querySelector('#lib-list-body');

        // Render immediately from localStorage, then reconcile with the server.
        const cachedMode = localStorage.getItem('library.sortMode') || 'auto';
        this._sortMode = cachedMode;
        sortSelect.value = cachedMode;
        LibraryAPI.getSettings().then(s => {
            const serverMode = s.librarySortMode || 'auto';
            if (serverMode !== this._sortMode) {
                this._sortMode = serverMode;
                localStorage.setItem('library.sortMode', serverMode);
                sortSelect.value = serverMode;
                this._loadList(body);
            }
        }).catch(() => {});

        sortSelect.addEventListener('change', async () => {
            const newMode = sortSelect.value;
            if (newMode === 'manual') await this._initManualOrder(this._cachedLibs || []);
            this._sortMode = newMode;
            localStorage.setItem('library.sortMode', newMode);
            LibraryAPI.patchSettings({ librarySortMode: newMode }); // fire-and-forget
            this._loadList(body);
        });

        this._listSelectionBar = new SelectionBar(el, {
            actions: ['collect', 'export', 'rename', 'location', 'organize', 'mark'],
            onAction: (action) => this._runSelectionAction(action),
        });

        this._mountFilter(el, body, null);
        this._syncInfoPanel();
        this._loadList(body);
    }

    async _loadList(body) {
        if (!body.firstChild) Activity.in(body, 'Reading the libraries…', { area: true });
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
            const sortSelect = body.closest('.library-list-view')?.querySelector('.lib-sort-select');
            if (sortSelect) sortSelect.value = mode;
        } catch (err) {
            body.innerHTML = `<div class="library-error">Could not read the libraries: ${escapeHtml(err.message)}. Reload the page to try again.</div>`;
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
        // The whole row opens the library — five orange Open buttons on one
        // screen were five primary actions. Scanning stays here because the
        // row is also where you see when it was last indexed; editing and
        // deleting moved into the library itself.
        top.innerHTML = `
            <button class="library-card-open lib-open">
                <span class="library-card-name">${escapeHtml(lib.name)}${hasNew ? '<span class="library-card-new-dot" title="New photos added"></span>' : ''}</span>
                <span class="library-card-count">${lib.photoCount.toLocaleString()} photo${lib.photoCount !== 1 ? 's' : ''}</span>
                <span class="library-card-meta">${escapeHtml(lib.sourcePath)}</span>
                ${lib.description ? `<span class="library-card-desc">${escapeHtml(lib.description)}</span>` : ''}
            </button>
            <div class="library-card-actions">
                <button class="btn btn-sm lib-scan-new">Scan for new photos</button>
                <span class="library-card-indexed">Indexed ${lastIdx}</span>
            </div>`;
        card.appendChild(top);

        if (lib.photoCount > 0) {
            const strip = document.createElement('div');
            strip.className = 'library-card-filmstrip';
            card.appendChild(strip);
            this._loadFilmstrip(strip, lib.id);
        }

        top.querySelector('.lib-open').addEventListener('click', () => this._openLibrary(lib));
        card.querySelector('.lib-scan-new').addEventListener('click',
            () => this._runScanCard(lib, card, (id, cb) => LibraryAPI.scanNew(id, cb), 'Scanning'));
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
        let host = card.querySelector('.library-card-progress');
        if (!host) {
            host = document.createElement('div');
            host.className = 'library-card-progress';
            card.querySelector('.library-card-actions').appendChild(host);
        }
        const activity = Activity.in(host, `${label}…`);
        const scanBtns = card.querySelectorAll('.lib-scan-new');
        scanBtns.forEach(b => { b.disabled = true; });
        try {
            await scanFn(lib.id, (p) => showLibraryProgress(activity, p, `${label}…`));
            const updated = await LibraryAPI.get(lib.id);
            card.querySelector('.library-card-count').textContent =
                `${updated.photoCount.toLocaleString()} photo${updated.photoCount !== 1 ? 's' : ''}`;
            card.querySelector('.library-card-indexed').textContent =
                `Indexed ${new Date(updated.lastIndexed).toLocaleDateString()}`;
        } catch (err) {
            activity.fail(`It stopped: ${err.message}`);
        } finally {
            scanBtns.forEach(b => { b.disabled = false; });
        }
    }

    _sortLibs(libs) {
        const mode = this._sortMode || localStorage.getItem('library.sortMode') || 'auto';
        if (mode === 'name') {
            return [...libs].sort((a, b) => a.name.localeCompare(b.name));
        }
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

    // Everything about the library itself: its name, its description, and —
    // behind a two-step confirmation naming what goes — deleting it.
    _showEditDialog(lib, onSaved = null) {
        const self = this;
        this._editDialog = new Dialog({
            title: 'Edit library',
            subtitle: lib.name,
            size: 'md',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="lib-edit-name">Name</label>
                <input class="library-dialog-input" id="lib-edit-name" type="text" autocomplete="off">
                <label class="library-dialog-label" for="lib-edit-desc">Description (optional)</label>
                <input class="library-dialog-input" id="lib-edit-desc" type="text">
                <div class="library-dialog-field">
                    <span class="library-dialog-label">Folder</span>
                    <span class="library-dialog-path">${escapeHtml(lib.sourcePath)}</span>
                    <span class="form-hint">The folder a library reads is fixed; make a new library to read another one.</span>
                </div>
                <div class="library-dialog-field">
                    <span class="library-dialog-label">Maintenance</span>
                    <div class="lib-maint-actions" id="lib-edit-maint-actions">
                        <button class="btn btn-sm" data-maint="scanNew">Scan for new photos</button>
                        <button class="btn btn-sm" data-maint="reindex">Rebuild metadata &amp; previews</button>
                        <button class="btn btn-sm" data-maint="regenMissingPreviews">Generate missing previews</button>
                        <button class="btn btn-sm" data-maint="rebuildAllPreviews">Rebuild all previews</button>
                        <button class="btn btn-sm" data-maint="cleanup">Remove deleted photos</button>
                    </div>
                    <span class="form-hint">Rarely needed. Scanning finds new files; the others re-read what is already indexed.</span>
                    <span class="library-dialog-progress" id="lib-edit-maint-progress" hidden></span>
                </div>
                <div class="library-dialog-danger" id="lib-edit-danger"></div>
                <div class="build-error" id="lib-edit-error" hidden></div>`,
            actions: [
                { label: 'Cancel', id: 'lib-edit-cancel', onClick: () => this._editDialog.close(null) },
                { label: 'Save', kind: 'primary', id: 'lib-edit-save', onClick: () => save() },
            ],
        });
        const dlg = this._editDialog.open();

        const nameEl = dlg.querySelector('#lib-edit-name');
        const descEl = dlg.querySelector('#lib-edit-desc');
        const errEl = dlg.querySelector('#lib-edit-error');
        nameEl.value = lib.name;
        descEl.value = lib.description || '';
        nameEl.focus();

        const close = () => this._editDialog.close(null);

        async function save() {
            const name = nameEl.value.trim();
            if (!name) { nameEl.focus(); return; }
            try {
                const updated = await LibraryAPI.update(lib.id, name, descEl.value.trim());
                lib.name = updated.name;
                lib.description = updated.description;
                self._cachedLibs = null;
                close();
                if (onSaved) onSaved(updated);
            } catch (err) {
                errEl.textContent = err.message;
                errEl.hidden = false;
            }
        }

        this._wireMaintenance(dlg, lib);
        this._renderLibraryDanger(dlg, lib, close);
        return dlg;
    }

    // All five maintenance runs live here rather than behind a chevron on the
    // Libraries row: they are rare, they act on this one library, and the
    // dialog has room to say what each one does while it runs.
    _wireMaintenance(dlg, lib) {
        const actions = dlg.querySelector('#lib-edit-maint-actions');
        const progress = dlg.querySelector('#lib-edit-maint-progress');
        const buttons = [...actions.querySelectorAll('[data-maint]')];
        const labels = {
            scanNew: 'Scanning',
            reindex: 'Indexing',
            regenMissingPreviews: 'Generating',
            rebuildAllPreviews: 'Rebuilding',
            cleanup: 'Checking',
        };

        for (const btn of buttons) {
            btn.addEventListener('click', async () => {
                const run = btn.dataset.maint;
                buttons.forEach(b => { b.disabled = true; });
                progress.hidden = false;
                const activity = Activity.in(progress, `${labels[run]}…`);
                try {
                    await LibraryAPI[run](lib.id, (p) => showLibraryProgress(activity, p, `${labels[run]}…`));
                    this._cachedLibs = null;
                } catch (err) {
                    activity.fail(`It stopped: ${err.message}`);
                } finally {
                    buttons.forEach(b => { b.disabled = false; });
                }
            });
        }
    }

    _renderLibraryDanger(dlg, lib, closeDialog) {
        const wrap = dlg.querySelector('#lib-edit-danger');
        wrap.innerHTML = '<button class="btn btn-sm btn-danger" id="lib-delete-start">Delete library…</button>';
        wrap.querySelector('#lib-delete-start').addEventListener('click', () => {
            wrap.innerHTML = `
                <p class="gal-danger-question">Delete "${escapeHtml(lib.name)}"? This removes the index and its thumbnails. The photos in ${escapeHtml(lib.sourcePath)} are not touched.</p>
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="lib-delete-cancel">Keep it</button>
                    <button class="btn btn-sm btn-danger" id="lib-delete-confirm">Delete library</button>
                </div>`;
            wrap.querySelector('#lib-delete-cancel').addEventListener('click', () => this._renderLibraryDanger(dlg, lib, closeDialog));
            wrap.querySelector('#lib-delete-confirm').addEventListener('click', async (e) => {
                Activity.button(e.currentTarget, 'Deleting…');
                wrap.querySelectorAll('button').forEach(b => { b.disabled = true; });
                try {
                    await LibraryAPI.delete(lib.id);
                    this._cachedLibs = null;
                    this.currentLibrary = null;
                    closeDialog();
                    this.render();
                    App.refreshLibraryVisibility();
                } catch (err) {
                    wrap.innerHTML = `<div class="build-error">Could not delete it: ${escapeHtml(err.message)}</div>`;
                }
            });
        });
    }

    _showCreateDialog(prefillPath) {
        this._createDialog = new Dialog({
            title: 'New library',
            size: 'md',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="lib-dlg-name">Name</label>
                <input class="library-dialog-input" id="lib-dlg-name" type="text" placeholder="My Photos" autocomplete="off">
                <label class="library-dialog-label" for="lib-dlg-path">Source folder</label>
                <input class="library-dialog-input" id="lib-dlg-path" type="text" placeholder="/Fotos/2024">
                <label class="library-dialog-label" for="lib-dlg-desc">Description (optional)</label>
                <input class="library-dialog-input" id="lib-dlg-desc" type="text" placeholder="">
                <div class="library-dialog-note">The folder is scanned when you press Create. A large folder takes a few minutes.</div>
                <div class="library-dialog-progress" id="lib-dlg-progress" style="display:none"></div>`,
            actions: [
                { label: 'Cancel', id: 'lib-dlg-cancel', onClick: () => this._createDialog.close(null) },
                { label: 'Create & index', kind: 'primary', id: 'lib-dlg-create', onClick: () => create() },
            ],
        });
        const dlg = this._createDialog.open();

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

        const create = async () => {
            const name = nameEl.value.trim();
            const path = stripQuotes(pathEl.value.trim());
            pathEl.value = path; // show cleaned value
            if (!name || !path) {
                progressEl.style.display = '';
                progressEl.textContent = 'A library needs a name and a folder to read.';
                (name ? pathEl : nameEl).focus();
                return;
            }

            createBtn.disabled = true;
            progressEl.style.display = '';
            const activity = Activity.in(progressEl, 'Creating the library…');

            try {
                const lib = await LibraryAPI.create(name, descEl.value.trim(), path);
                this._cachedLibs = null;
                activity.busy('Looking for photos…');
                let failed = false;
                await LibraryAPI.reindex(lib.id, (p) => {
                    failed = failed || !!p.error;
                    showLibraryProgress(activity, p, 'Looking for photos…');
                });
                // The library exists even if indexing stopped, so creating it
                // again would make a second one. The reason stays readable and
                // the one thing left to do is open what was created.
                if (failed) {
                    this._createDialog.setActions([{
                        label: 'Open the library', kind: 'primary',
                        onClick: () => { this._createDialog.close(null); App.refreshLibraryVisibility(); this._openLibrary(lib); },
                    }]);
                    return;
                }
                this._createDialog.close(null);
                App.refreshLibraryVisibility();
                this._openLibrary(lib);
            } catch (err) {
                activity.fail(`The library was not created: ${err.message}`);
                createBtn.disabled = false;
            }
        };
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
        App.refreshLibraryVisibility();
    }

    // The overview with the filter already set — the way from a gallery or a
    // destination to its photos, across every library.
    async showFiltered(criteria) {
        this.currentLibrary = null;
        this._pane = null;
        this._infoPanel = null;
        this.render();
        App.refreshLibraryVisibility();
        await this._filterPanel.openWith(criteria);
    }

    // Open a library by id — used by the sidebar's per-library entries.
    async openLibraryById(libraryId) {
        if (String(this.currentLibrary?.id) === String(libraryId)) return;
        const libs = this._cachedLibs?.length ? this._cachedLibs : await LibraryAPI.list().catch(() => []);
        const lib = libs.find(l => String(l.id) === String(libraryId));
        if (lib) this._openLibrary(lib);
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
                ${this._filterButtonHTML()}
                <div class="library-detail-title">
                    <span class="library-detail-name">${escapeHtml(lib.name)}</span>
                    <span class="library-detail-path">${escapeHtml(lib.sourcePath)}</span>
                </div>
                <div class="library-detail-controls">
                    <button class="btn btn-sm" id="lib-detail-stats-btn">Statistics</button>
                    <button class="btn btn-sm desk-only" id="lib-edit-btn">Edit library…</button>
                </div>
            </div>
            ${this._filterBodyHTML('<div class="library-pane-wrap" id="lib-pane"></div>')}`;
        this.container.appendChild(el);
        // Actions on a selection live in the bar, not in the header, so they
        // are never shown greyed out with no reason given.
        this._detailSelectionBar = new SelectionBar(el, {
            actions: ['collect', 'export', 'rename', 'location', 'organize', 'mark'],
            onAction: (action) => this._runSelectionAction(action),
        });

        el.querySelector('#lib-back').addEventListener('click', () => {
            this._pane = null;
            this._infoPanel = null;
            this.currentLibrary = null;
            this.render();
        });

        el.querySelector('#lib-detail-stats-btn').addEventListener('click', () => this._openStats());
        el.querySelector('#lib-edit-btn').addEventListener('click', () => {
            this._showEditDialog(lib, (updated) => {
                this.currentLibrary = { ...lib, ...updated };
                this.render();
            });
        });

        const paneEl = el.querySelector('#lib-pane');
        this._mountFilter(el, paneEl, lib.id);
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
                this._updateSelectionBar();
            },
        });

        this._syncInfoPanel();
        this._pane.load('');
    }

    /* --- Filter: one column, the same in the overview and in a library --- */

    // The button stands at the left end of the head, directly above the
    // column it opens, with the same panel glyph as the sidebar's collapse
    // button and the number of criteria that are on.
    _filterButtonHTML() {
        return `
            <button class="btn btn-sm lib-filter-toggle" id="lib-filter-btn" aria-pressed="false" aria-expanded="false" data-state="off" aria-controls="lib-filter-panel">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                    <rect x="1.5" y="2.5" width="13" height="11"/><path d="M5.5 2.5v11"/>
                    <path class="collapse-chevron" d="M8.5 6l2 2-2 2"/>
                </svg>
                Filter
                <span class="lib-filter-btn-count" hidden></span>
            </button>`;
    }

    // `underHTML` is what the column sits beside — the list of libraries or
    // the library's own folders. The results take its place once a criterion
    // is set, and the × on them gives it back.
    _filterBodyHTML(underHTML) {
        return `
            <div class="lib-filter-body">
                <div class="lib-filter-panel" id="lib-filter-panel"></div>
                <div class="lib-filter-main">
                    ${underHTML}
                    <div class="library-pane-wrap" id="lib-results-pane" style="display:none"></div>
                    <div class="lib-info-panel-container" id="lib-info-panel"></div>
                </div>
            </div>`;
    }

    // scopeLibID: the library the filter starts with, or null for all of them.
    _mountFilter(el, underEl, scopeLibID) {
        this._filterEl = el;
        this._filterUnderEl = underEl;
        this._resultsPane = null;

        this._infoPanel = new InfoPanel(el.querySelector('#lib-info-panel'));
        this._infoPanel.onToggle = () => {
            if (this._infoPanel.expanded) this.getActivePaneForKeyboard()?._notifyFocusChange();
        };

        this._filterPanel = new LibraryFilterPanel(
            el.querySelector('#lib-filter-panel'),
            el.querySelector('#lib-filter-btn'),
            scopeLibID,
            {
                onResults: (photos, multiLib, pagination) => this._showFilterResults(photos, multiLib, pagination),
                // The panel takes its width from the photos, so the justified
                // layout has to re-pack when it comes and goes.
                onOpen: () => this._relayoutPhotos(),
                onClose: () => this._relayoutPhotos(),
                onActiveCount: (n) => this._updateFilterCount(n),
                onLoading: (on) => this._showFilterLoading(on),
                onError: () => this._resultsEl().classList.add('is-stale'),
            }
        );
    }

    _relayoutPhotos() {
        const pane = this.getActivePaneForKeyboard();
        if (pane && pane.view === 'justified') pane._justifiedRenderer.scheduleRelayout();
    }

    // The button says how many filters are on, so a closed panel never hides
    // the reason why fewer photos are shown.
    _updateFilterCount(count) {
        const badge = this._filterEl.querySelector('.lib-filter-btn-count');
        if (!badge) return;
        badge.textContent = count ? String(count) : '';
        badge.hidden = count === 0;
    }

    _resultsEl() {
        return this._filterEl.querySelector('#lib-results-pane');
    }

    // The first answer fills an empty area, so it says what it is doing. Later
    // answers replace results that are already there: those stay in view and
    // fade only if the answer is slow (the delay lives in the CSS).
    _showFilterLoading(isLoading) {
        const resultsEl = this._resultsEl();
        if (isLoading && resultsEl.style.display === 'none') {
            this._filterUnderEl.style.display = 'none';
            if (!this._resultsPane) {
                this._resultsActivity = Activity.in(resultsEl, 'Searching…', { area: true });
            }
            resultsEl.style.display = '';
        } else if (!isLoading) {
            this._resultsActivity?.el.remove();
            this._resultsActivity = null;
        }
        resultsEl.classList.toggle('is-stale', isLoading && !!this._resultsPane);
    }

    _showFilterResults(photos, multiLib, pagination) {
        const resultsEl = this._resultsEl();
        if (!this._resultsPane) {
            this._resultsPane = new SearchResultPane(resultsEl, {
                onImageClick: (path) => App.openViewer(path, this._resultsPane),
                onFocusChange: (path) => this._onResultFocus(path),
                onSlideshowInvoke: () => App.handleSlideshowInvoke(this._resultsPane),
                onToolInvoke: (params) => App.handleToolInvoke({ ...params, sourcePath: this.currentLibrary?.sourcePath ?? null }),
                onSelectionChange: () => this._updateSelectionBar(),
                // × drops the criteria and gives back what the results replaced.
                onClose: () => {
                    this._filterPanel.clearAndHide();
                    this._hideFilterResults();
                },
            });
        }
        this._resultsPane.loadResults(photos, multiLib, pagination);
        this._filterUnderEl.style.display = 'none';
        resultsEl.style.display = '';
        this._syncInfoPanel();
        this._updateSelectionBar();
    }

    // The results pane is kept for reuse; hidden, it no longer takes keys.
    _hideFilterResults() {
        this._resultsEl().style.display = 'none';
        this._filterUnderEl.style.display = '';
        if (this._pane) this._pane._notifyFocusChange();
        else this._infoPanel?.clear();
        this._syncInfoPanel();
        this._updateSelectionBar();
    }

    // The info panel describes photos; beside the list of libraries there
    // is nothing for it to describe, so it is only there when a pane is.
    _syncInfoPanel() {
        const el = this._filterEl.querySelector('#lib-info-panel');
        el.style.display = this.getActivePaneForKeyboard() ? '' : 'none';
    }

    async _onResultFocus(path) {
        const infoPanel = this._infoPanel;
        if (!infoPanel || !infoPanel.expanded) return;
        if (!path) { infoPanel.clear(); return; }

        const info = this._resultsPane ? this._resultsPane.getPhotoInfo(path) : null;
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

    // The active pane's selection drives the bar; "Show in Organize" needs a
    // folder or file that exists on disk under the browse root, which not
    // every selection has, so it says why when it cannot be used.
    _updateSelectionBar() {
        const pane = this.getActivePaneForKeyboard();
        const count = (pane?.getSelectedFiles().length ?? 0) + (pane?.selectedDirs?.size ?? 0);
        // The list view and an opened library each have their own bar; only
        // the one belonging to the visible screen may show anything.
        const active = this.currentLibrary ? this._detailSelectionBar : this._listSelectionBar;
        const other = this.currentLibrary ? this._listSelectionBar : this._detailSelectionBar;
        other?.update(0);
        const target = pane ? pane.getOpenInCommanderTarget() : null;
        active?.update(count, {
            disabled: {
                organize: target ? '' : (pane ? pane.organizeBtnHint() : 'Select photos to show them in Organize'),
            },
        });
    }

    _runSelectionAction(action) {
        const pane = this.getActivePaneForKeyboard();
        if (!pane) return;
        if (action === 'clear') {
            pane.selection.clear();
            pane.selectedDirs?.clear();
            pane.updateSelectionClasses();
            this._updateSelectionBar();
            return;
        }
        if (action === 'collect') { this._openCollectModal(); return; }
        if (action === 'organize') {
            const target = pane.getOpenInCommanderTarget();
            if (target) App.openCommanderAt(target.dir, target.names);
            return;
        }
        const files = pane.getSelectedFiles();
        if (!files.length) return;
        if (action === 'mark') {
            App.markForDeletion(files, pane.entries, pane.path);
            pane.updateMarkedForDeletion();
            return;
        }
        const tool = action === 'export' ? 'export'
            : action === 'location' ? 'set-location'
            // Always the batch dialog, even for one photo: it previews the
            // result, and the single-file path was a browser prompt().
            : action === 'rename' ? 'batch-rename'
            : null;
        if (!tool) return;
        App.handleToolInvoke({ tool, files, path: pane.path, sourcePath: this.currentLibrary?.sourcePath ?? null });
    }

    // Resolves the current selection to library photo IDs and hands the
    // picking to CollectDialog. A selection can span several libraries (the
    // cross-library search), and it is still one gallery.
    async _openCollectModal() {
        const active = this.getActivePaneForKeyboard();
        const searchPane = active && active === this._resultsPane && active.selection.selected.size > 0
            ? active : null;

        const lib = this.currentLibrary;
        let photoGroups = null;
        let selectedPaths = [];
        let selectedDirs = [];

        if (searchPane) {
            const byLib = new Map();
            for (const hint of searchPane.getSelectedFiles()) {
                const info = searchPane.getPhotoInfo(hint);
                if (!info) continue;
                if (!byLib.has(info.libID)) byLib.set(info.libID, []);
                byLib.get(info.libID).push(info.photoID);
            }
            photoGroups = Array.from(byLib.entries()).map(([libID, photoIDs]) => ({ libID, photoIDs }));
        } else {
            const pane = this._pane;
            if (!pane) return;
            selectedPaths = pane.getSelectedFiles();
            selectedDirs = Array.from(pane.selectedDirs || []);
            if (selectedPaths.length === 0 && selectedDirs.length === 0) return;
        }

        // A selected folder means every photo in it, which takes a round trip
        // to resolve — do it before the dialog so its count is the real one.
        if (selectedDirs.length > 0) {
            const n = selectedDirs.length;
            const arrays = await App.whileSlow(`Collecting the photos of ${n} folder${n !== 1 ? 's' : ''}…`,
                () => Promise.all(selectedDirs.map(d => this._pane.fetchRecursivePhotoPaths(d))));
            selectedPaths = selectedPaths.concat(arrays.flat());
        }

        const count = photoGroups
            ? photoGroups.reduce((n, g) => n + g.photoIDs.length, 0)
            : selectedPaths.length;
        if (count === 0) return;

        new CollectDialog({
            count,
            onCollect: async ({ slug, draftID, postID, title, unlisted, account }) => {
                let groups = photoGroups;
                if (!groups) {
                    const photoIDs = await Promise.all(selectedPaths.map(p => LibraryAPI.photoIDByPath(lib.id, p)));
                    groups = [{ libID: lib.id, photoIDs: photoIDs.filter(Boolean) }];
                }
                groups = groups.filter(g => g.photoIDs.length > 0);
                if (groups.length === 0) throw new Error('No matching library photos found for this selection.');

                // The first call creates the draft, the rest append to it.
                // Passing the title to every call would instead create one
                // gallery per library, all with the same name.
                let total = 0;
                let currentDraft = draftID;
                for (const g of groups) {
                    const draft = await LibraryAPI.collect(g.libID, slug, {
                        photoIDs: g.photoIDs,
                        draftID: currentDraft,
                        postID: !currentDraft ? postID : undefined,
                        title: !currentDraft && !postID ? title : undefined,
                        unlisted: currentDraft ? undefined : unlisted,
                        account: currentDraft ? undefined : account,
                    });
                    currentDraft = draft?.id || currentDraft;
                    total += g.photoIDs.length;
                }
                return total;
            },
        }).open();
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


