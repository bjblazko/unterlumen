// library-list.js — the overview of libraries: a card each with its photos as
// a strip, when it was indexed and its scan, in the order chosen (recently
// added, by name, or an order of one's own). The tab draws the place around
// it; the libraries read last stay on the tab (_cachedLibs), which the
// dialogs empty when they change one.

class LibraryList {
    constructor(tab) {
        this.tab = tab;
        this.sortMode = localStorage.getItem('library.sortMode') || 'auto';
    }

    // Sorting is the browser's first, then the one saved on the server.
    wireSort(sortSelect, body) {
        sortSelect.value = this.sortMode;
        LibraryAPI.getSettings().then(s => {
            const serverMode = s.librarySortMode || 'auto';
            if (serverMode === this.sortMode) return;
            this._setMode(serverMode, sortSelect);
            this.load(body);
        }).catch(() => {});
        sortSelect.addEventListener('change', async () => {
            if (sortSelect.value === 'manual') await this._initManualOrder(this.tab._cachedLibs || []);
            this._setMode(sortSelect.value, sortSelect);
            LibraryAPI.patchSettings({ librarySortMode: sortSelect.value }); // fire-and-forget
            this.load(body);
        });
    }

    _setMode(mode, sortSelect) {
        this.sortMode = mode;
        localStorage.setItem('library.sortMode', mode);
        sortSelect.value = mode;
    }

    async load(body) {
        if (!body.firstChild) Activity.in(body, t('Reading the libraries…'), { area: true });
        try {
            const libs = await LibraryAPI.list();
            this.tab._cachedLibs = libs;
            const lastSeen = parseInt(localStorage.getItem('library.lastOverviewVisit') || '0', 10);
            localStorage.setItem('library.lastOverviewVisit', Date.now().toString());
            this._fill(body, libs, lastSeen);
        } catch (err) {
            body.innerHTML = `<div class="library-error">${t('Could not read the libraries: {error}. Reload the page to try again.', { error: escapeHtml(err.message) })}</div>`;
        }
    }

    _fill(body, libs, lastSeen) {
        body.innerHTML = '';
        const meta = body.closest('.library-list-view')?.querySelector('.place-meta');
        if (meta) meta.innerHTML = countHTML(libs.length, 'library', 'libraries');
        if (libs.length === 0) {
            body.insertAdjacentHTML('beforeend', `<div class="library-empty">${t('No libraries yet. Choose New library… and pick a folder; its photos are read once and stay where they are.')}</div>`);
            return;
        }
        for (const lib of this._sorted(libs)) {
            const card = this._card(lib, lastSeen);
            body.appendChild(card);
            if (lib.scanning) {
                const signal = this.tab._scanWatch.signal;
                this._runScan(lib, card, (id, cb) => LibraryAPI.scanNew(id, cb, undefined, signal), t('Scanning'), signal);
            }
            if (this.sortMode === 'manual') this._addMoveButtons(lib, card, body);
        }
    }

    // The whole row opens the library — five orange Open buttons on one
    // screen were five primary actions. Scanning stays here because the row
    // is also where you see when it was last indexed; editing and deleting
    // moved into the library itself.
    _card(lib, lastSeen) {
        const card = document.createElement('div');
        card.className = 'library-card';
        card.innerHTML = `<div class="library-card-top">${this._cardTopHTML(lib, lastSeen)}</div>`;
        if (lib.photoCount > 0) {
            const strip = document.createElement('div');
            strip.className = 'library-card-filmstrip';
            card.appendChild(strip);
            this._loadFilmstrip(strip, lib.id);
        }
        card.querySelector('.lib-open').addEventListener('click', () => this.tab._openLibrary(lib));
        card.querySelector('.lib-scan-new').addEventListener('click',
            () => this._runScan(lib, card, (id, cb) => LibraryAPI.scanNew(id, cb), t('Scanning')));
        return card;
    }

    _cardTopHTML(lib, lastSeen) {
        const hasNew = lib.lastNewPhotos && new Date(lib.lastNewPhotos).getTime() > lastSeen;
        return `
            <button class="library-card-open lib-open">
                <span class="library-card-name">${escapeHtml(lib.name)}${hasNew ? `<span class="library-card-new-dot" title="${t('New photos added')}"></span>` : ''}</span>
                <span class="library-card-count">${libraryCountText(lib)}</span>
                <span class="library-card-meta">${escapeHtml(lib.sourcePath)}${libraryStateWords(lib)}</span>
                ${lib.description ? `<span class="library-card-desc">${escapeHtml(lib.description)}</span>` : ''}
            </button>
            <div class="library-card-actions">
                <button class="btn btn-sm lib-scan-new">${t('Scan for new photos')}</button>
                <span class="library-card-indexed">${lib.lastIndexed ? t('Indexed {day}', { day: formatDay(lib.lastIndexed) }) : t('Never indexed')}</span>
            </div>`;
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

    async _runScan(lib, card, scanFn, label, signal) {
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
            if (signal?.aborted) return; // the card is gone
            const updated = await LibraryAPI.get(lib.id);
            card.querySelector('.library-card-count').textContent = libraryCountText(updated);
            card.querySelector('.library-card-indexed').textContent = t('Indexed {day}', { day: formatDay(updated.lastIndexed) });
        } catch (err) {
            activity.fail(t('It stopped: {error}', { error: err.message }));
        } finally {
            scanBtns.forEach(b => { b.disabled = false; });
        }
    }

    _sorted(libs) {
        if (this.sortMode === 'name') return [...libs].sort((a, b) => a.name.localeCompare(b.name));
        if (this.sortMode === 'manual') return [...libs].sort((a, b) => (a.sortPosition ?? Infinity) - (b.sortPosition ?? Infinity));
        return [...libs].sort(newestPhotosFirst);
    }

    // An order of one's own starts as the order they had.
    async _initManualOrder(libs) {
        if (!libs || !libs.length) return;
        if (libs.some(l => l.sortPosition != null)) return; // already seeded server-side
        await LibraryAPI.setOrder([...libs].sort(newestPhotosFirst).map(l => l.id));
    }

    _addMoveButtons(lib, card, body) {
        const actions = card.querySelector('.library-card-actions');
        for (const [cls, glyph, title, step] of [['lib-move-up', '↑', 'Move up', -1], ['lib-move-down', '↓', 'Move down', 1]]) {
            const btn = document.createElement('button');
            btn.className = `btn btn-sm ${cls}`;
            btn.textContent = glyph;
            btn.title = t(title);
            btn.addEventListener('click', () => this._move(lib, step, body));
            actions.appendChild(btn);
        }
    }

    async _move(lib, step, body) {
        const order = this._sorted(this.tab._cachedLibs || []).map(l => l.id);
        const i = order.indexOf(lib.id);
        const j = i + step;
        if (i === -1 || j < 0 || j >= order.length) return;
        [order[i], order[j]] = [order[j], order[i]];
        await LibraryAPI.setOrder(order);
        this.load(body);
    }
}

// Libraries with new photos first, the newest first; those without last.
function newestPhotosFirst(a, b) {
    if (!a.lastNewPhotos && !b.lastNewPhotos) return 0;
    if (!a.lastNewPhotos) return 1;
    if (!b.lastNewPhotos) return -1;
    return new Date(b.lastNewPhotos) - new Date(a.lastNewPhotos);
}

// libraryCountText is what a library holds: its photos, and its videos when
// it has any (ADR-0058).
function libraryCountText(lib) {
    const videos = lib.videoCount || 0;
    const photos = countText(lib.photoCount - videos, 'photo');
    return videos ? `${photos} · ${countText(videos, 'video')}` : photos;
}
