// viewer-menu.js — the full view's ⋯ menu: the film strip, full screen and
// the original file, then what an overview does to a selection, done to the
// photo on screen. A library photo — opened from a
// library, the filter, the Map, Statistics or the Timeline — can be added to a
// gallery, exported, renamed, located, shown in Organize and shown in its
// library's folder. A photo opened from Folders or Organize has what Folders
// offers. Download and Delete stay buttons of their own.

class ViewerMenu {
    // libraryRef: (key) => { lib, id } for a library photo, or null for a
    // photo in Folders, whose key is its path under the browse boundary.
    constructor(viewer, libraryRef) {
        this._viewer = viewer;
        this._libraryRef = libraryRef || (() => null);
        this.menu = new Menu({ items: () => this._items(), label: t('More actions') });
    }

    _items() {
        const key = this._viewer.currentPath;
        const ref = this._libraryRef(key);
        return [...this._viewItems(), 'separator', ...(ref ? this._libraryItems(ref) : this._folderItems(key))];
    }

    // How the photo is shown, and its file: rare enough for the menu, and
    // each has its key.
    _viewItems() {
        const v = this._viewer;
        const items = [{ id: 'filmstrip', label: t('Film strip (F)'), switch: { on: v.filmStripVisible, labelOn: t('Shown'), labelOff: t('Hidden') }, onChange: (on) => v.setFilmStrip(on) }];
        if (Fullscreen.available()) items.push({ id: 'fullscreen', label: Fullscreen.active() ? t('Leave full screen') : t('Full screen'), onSelect: () => v.toggleFullscreen() });
        items.push({ id: 'download', label: t('Download original'), onSelect: () => v.downloadOriginal() });
        return items;
    }

    _libraryItems(ref) {
        const photoRef = { getLibraryMeta: () => ({ libID: ref.lib, photoID: ref.id }) };
        return [
            { id: 'collect', label: t('Add to gallery…'), onSelect: () => this._collect(ref) },
            { id: 'export', label: t('Export…'), onSelect: () => this._withPhoto(ref, p => App._openExport([p.path], null, photoRef)) },
            { id: 'rename', label: t('Rename…'), onSelect: () => this._withPhoto(ref, p => this._rename(p.path)) },
            { id: 'location', label: t('Set location…'), onSelect: () => this._withPhoto(ref, p => this._tool('set-location', p.path)) },
            { id: 'organize', label: t('Show in Organize'), onSelect: () => this._withPhoto(ref, p => this._leave(() => App.openCommanderAt(dirOf(p.path), [p.name]))) },
            'separator',
            { id: 'library', label: t('Show in library'), onSelect: () => showPhotoInLibrary(ref, () => this._viewer.close()) },
        ];
    }

    _folderItems(path) {
        const items = [
            { id: 'export', label: t('Export…'), onSelect: () => App.handleToolInvoke({ tool: 'export', files: [path] }) },
            { id: 'rename', label: t('Rename…'), onSelect: () => this._rename(path) },
            { id: 'location', label: t('Set location…'), onSelect: () => this._tool('set-location', path) },
        ];
        if (App.mode !== 'organize') {
            const boundary = (App.config?.boundary || '').replace(/\/$/, '');
            const dir = dirOf(path);
            items.push({ id: 'organize', label: t('Show in Organize'), onSelect: () => this._leave(() => App.openCommanderAt(dir ? `${boundary}/${dir}` : boundary, [path.split('/').pop()])) });
        }
        return items;
    }

    // A new location changes what the info panel shows.
    _tool(tool, file) {
        App.handleToolInvoke({ tool, files: [file], onDone: () => this._viewer.reloadInfo() });
    }

    // The photo is gone under its old name, so the viewer gives way to the
    // place it was opened from, which shows the new one.
    _rename(file) {
        App.handleToolInvoke({ tool: 'batch-rename', files: [file], onDone: () => this._viewer.close() });
    }

    _collect(ref) {
        new CollectDialog({
            count: 1,
            onCollect: (choice) => collectPhotoGroups([{ libID: ref.lib, photoIDs: [ref.id] }], choice),
        }).open();
    }

    // Going to another place closes the viewer first: it lies over the place
    // it was opened from.
    _leave(go) {
        this._viewer.close();
        go();
    }

    async _withPhoto(ref, fn) {
        const photo = await readLibraryPhoto(ref);
        if (photo) fn(photo);
    }
}

// readLibraryPhoto says where a library photo ({ lib, id }) is now: its
// absolute path and name, and its library's folder. It is read when an
// action is chosen, since a rename moves it; null, said in a toast, when the
// photo cannot be found.
async function readLibraryPhoto(ref) {
    try {
        const [info, libs] = await Promise.all([
            fetch(`/api/library/${ref.lib}/photo/${ref.id}/info`).then(async r => {
                if (!r.ok) throw new Error(await r.text() || `the server answered ${r.status}`);
                return r.json();
            }),
            LibraryAPI.list(),
        ]);
        const lib = libs.find(l => String(l.id) === String(ref.lib));
        return { path: info.path, name: info.name, sourcePath: lib?.sourcePath ?? '' };
    } catch (err) {
        App.showToast(t('This photo could not be found in its library: {err}', { err: err.message }));
        return null;
    }
}

// showPhotoInLibrary opens a library photo's library at its folder, the
// photo selected. leave runs just before, to close what lies over the place.
async function showPhotoInLibrary(ref, leave = () => {}) {
    const photo = await readLibraryPhoto(ref);
    if (!photo) return;
    const source = photo.sourcePath.replace(/\/$/, '');
    const dir = dirOf(photo.path);
    const relDir = dir === source ? '' : dir.startsWith(source + '/') ? dir.slice(source.length + 1) : null;
    if (relDir === null) {
        App.showToast(t('This photo is no longer inside its library\'s folder. Scan the library to find it again.'));
        return;
    }
    leave();
    App.showInLibrary(ref.lib, relDir, photo.name);
}

function dirOf(path) {
    const i = path.lastIndexOf('/');
    return i < 0 ? '' : path.slice(0, i);
}
