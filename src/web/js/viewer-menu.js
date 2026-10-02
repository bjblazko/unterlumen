// viewer-menu.js — the full view's ⋯ menu: what an overview does to a
// selection, done to the photo on screen. A library photo — opened from a
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
        this.menu = new Menu({ items: () => this._items(), label: 'More actions' });
    }

    _items() {
        const key = this._viewer.currentPath;
        const ref = this._libraryRef(key);
        return ref ? this._libraryItems(ref) : this._folderItems(key);
    }

    _libraryItems(ref) {
        const photoRef = { getLibraryMeta: () => ({ libID: ref.lib, photoID: ref.id }) };
        return [
            { id: 'collect', label: 'Add to gallery…', onSelect: () => this._collect(ref) },
            { id: 'export', label: 'Export…', onSelect: () => this._withPhoto(ref, p => App._openExport([p.path], null, photoRef)) },
            { id: 'rename', label: 'Rename…', onSelect: () => this._withPhoto(ref, p => this._rename(p.path)) },
            { id: 'location', label: 'Set location…', onSelect: () => this._withPhoto(ref, p => this._tool('set-location', p.path)) },
            { id: 'organize', label: 'Show in Organize', onSelect: () => this._withPhoto(ref, p => this._leave(() => App.openCommanderAt(dirOf(p.path), [p.name]))) },
            'separator',
            { id: 'library', label: 'Show in library', onSelect: () => this._withPhoto(ref, p => this._showInLibrary(ref, p)) },
        ];
    }

    _folderItems(path) {
        const items = [
            { id: 'export', label: 'Export…', onSelect: () => App.handleToolInvoke({ tool: 'export', files: [path] }) },
            { id: 'rename', label: 'Rename…', onSelect: () => this._rename(path) },
            { id: 'location', label: 'Set location…', onSelect: () => this._tool('set-location', path) },
        ];
        if (App.mode !== 'organize') {
            const boundary = (App.config?.boundary || '').replace(/\/$/, '');
            const dir = dirOf(path);
            items.push({ id: 'organize', label: 'Show in Organize', onSelect: () => this._leave(() => App.openCommanderAt(dir ? `${boundary}/${dir}` : boundary, [path.split('/').pop()])) });
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

    _showInLibrary(ref, photo) {
        const source = (photo.sourcePath || '').replace(/\/$/, '');
        const dir = dirOf(photo.path);
        const relDir = dir === source ? '' : dir.startsWith(source + '/') ? dir.slice(source.length + 1) : null;
        if (relDir === null) {
            App.showToast('This photo is no longer inside its library\'s folder. Scan the library to find it again.');
            return;
        }
        this._leave(() => App.showInLibrary(ref.lib, relDir, photo.name));
    }

    // Going to another place closes the viewer first: it lies over the place
    // it was opened from.
    _leave(go) {
        this._viewer.close();
        go();
    }

    // Where a library photo is now: its absolute path and name, and its
    // library's folder. Read when an action is chosen, since a rename moves it.
    async _withPhoto(ref, fn) {
        let photo;
        try {
            const [info, libs] = await Promise.all([
                fetch(`/api/library/${ref.lib}/photo/${ref.id}/info`).then(async r => {
                    if (!r.ok) throw new Error(await r.text() || `the server answered ${r.status}`);
                    return r.json();
                }),
                LibraryAPI.list(),
            ]);
            const lib = libs.find(l => String(l.id) === String(ref.lib));
            photo = { path: info.path, name: info.name, sourcePath: lib?.sourcePath ?? '' };
        } catch (err) {
            App.showToast(`This photo could not be found in its library: ${err.message}`);
            return;
        }
        fn(photo);
    }
}

function dirOf(path) {
    const i = path.lastIndexOf('/');
    return i < 0 ? '' : path.slice(0, i);
}
