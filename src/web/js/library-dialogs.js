// LibraryDialogs — creating a library, and editing one: its name and
// description, maintenance jobs and deletion. Opened by the LibraryTab that
// owns it, whose list it refreshes.

class LibraryDialogs {
    constructor(tab) {
        this.tab = tab;
    }

    // Everything about the library itself: its name, its description, and —
    // behind a two-step confirmation naming what goes — deleting it.
    edit(lib, onSaved = null) {
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
                        <button class="btn btn-sm" data-maint="analyseAgain">Analyse photos again</button>
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

    // All six maintenance runs live here rather than behind a chevron on the
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
            analyseAgain: 'Analysing',
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
                    this.tab._cachedLibs = null;
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
                    this.tab._cachedLibs = null;
                    this.tab.currentLibrary = null;
                    closeDialog();
                    this.tab.render();
                    App.refreshLibraryVisibility();
                } catch (err) {
                    wrap.innerHTML = `<div class="build-error">Could not delete it: ${escapeHtml(err.message)}</div>`;
                }
            });
        });
    }

    create(prefillPath) {
        this._createDialog = new Dialog({
            title: 'New library',
            size: 'md',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="lib-dlg-name">Name</label>
                <input class="library-dialog-input" id="lib-dlg-name" type="text" placeholder="My Photos" autocomplete="off">
                <label class="library-dialog-label" for="lib-dlg-path">Source folder</label>
                <div class="library-dialog-path-row">
                    <input class="library-dialog-input" id="lib-dlg-path" type="text" placeholder="/Fotos/2024">
                    <button class="btn btn-sm" id="lib-dlg-choose" type="button">Choose…</button>
                </div>
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

        // The folder is chosen, not typed; the field stays for pasting a path.
        dlg.querySelector('#lib-dlg-choose').addEventListener('click', async () => {
            const current = stripQuotes(pathEl.value.trim()).replace(/^\//, '');
            const picked = await new FolderPicker().open(current, { title: 'Folder for the new library' });
            if (picked === null) return;
            pathEl.value = '/' + picked;
            if (!nameEl.value.trim()) nameEl.value = picked.split('/').filter(Boolean).pop() || '';
            nameEl.focus();
        });

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
                this.tab._cachedLibs = null;
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
                        onClick: () => { this._createDialog.close(null); App.refreshLibraryVisibility(); this.tab._openLibrary(lib); },
                    }]);
                    return;
                }
                this._createDialog.close(null);
                App.refreshLibraryVisibility();
                this.tab._openLibrary(lib);
            } catch (err) {
                activity.fail(`The library was not created: ${err.message}`);
                createBtn.disabled = false;
            }
        };
    }

    // Open the create dialog pre-filled with a known path (from Tools menu).
}
