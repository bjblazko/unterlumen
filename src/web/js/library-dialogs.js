// LibraryDialogs — creating a library, and editing one: its name and
// description, whether it is shared, maintenance jobs and deletion. Opened by the LibraryTab that
// owns it, whose list it refreshes.

// keywordLanguageOptions offers the languages things are written in as
// keywords (ADR-0056), each in its own words, current chosen.
function keywordLanguageOptions(current) {
    return Object.entries(I18n.languages).map(([code, name]) =>
        `<option value="${code}" lang="${code}"${code === current ? ' selected' : ''}>${name}</option>`).join('');
}

class LibraryDialogs {
    constructor(tab) {
        this.tab = tab;
    }

    // Everything about the library itself: its name, its description, and —
    // behind a two-step confirmation naming what goes — deleting it.
    edit(lib, onSaved = null) {
        const self = this;
        this._editDialog = new Dialog({
            title: t('Edit library'),
            subtitle: lib.name,
            size: 'md',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="lib-edit-name">${t('Name')}</label>
                <input class="library-dialog-input" id="lib-edit-name" type="text" autocomplete="off">
                <label class="library-dialog-label" for="lib-edit-desc">${t('Description (optional)')}</label>
                <input class="library-dialog-input" id="lib-edit-desc" type="text">
                <label class="library-dialog-label" for="lib-edit-lang">${t('Keywords in the sidecars')}</label>
                <select class="btn btn-sm select-btn" id="lib-edit-lang">${keywordLanguageOptions(lib.keywordLanguage)}</select>
                <span class="form-hint">${t('Things found in the photos are written as keywords into the .xmp files beside them, which Lightroom, Apple Photos and other programs read too. This is their language. It can be changed later: Unterlumen then rewrites its own keywords, never yours.')}</span>
                <div class="library-dialog-field">
                    <span class="library-dialog-label">${t('Folder')}</span>
                    <span class="library-dialog-path">${escapeHtml(lib.sourcePath)}</span>
                    <span class="form-hint">${t('The folder a library reads is fixed; make a new library to read another one.')}</span>
                </div>
                <div class="library-dialog-field">
                    <span class="library-dialog-label">${t('Share with other installations')}</span>
                    <div id="lib-edit-share"></div>
                    <span class="form-hint" id="lib-edit-share-hint"></span>
                </div>
                <div class="library-dialog-field">
                    <span class="library-dialog-label">${t('Maintenance')}</span>
                    <div class="lib-maint-actions" id="lib-edit-maint-actions">
                        <button class="btn btn-sm" data-maint="scan">${t('Scan for new photos')}</button>
                        <button class="btn btn-sm" data-maint="again">${t('Read everything again…')}</button>
                    </div>
                    <span class="form-hint">${t('Scanning adds new photos, drops those whose files are gone and makes missing previews. Reading everything again reads every photo anew — details, previews, colours and faces — for when they look wrong; it takes as long as the first scan.')}</span>
                    <p class="lib-maint-confirm" hidden>${t('Read every photo of {name} again? This takes as long as its first scan; the library stays usable meanwhile.', { name: escapeHtml(lib.name) })}
                        <button class="btn btn-sm" data-maint-confirm="no">${t('Cancel')}</button>
                        <button class="btn btn-sm" data-maint-confirm="yes">${t('Read everything again')}</button></p>
                    <span class="library-dialog-progress" id="lib-edit-maint-progress" hidden></span>
                </div>
                <div class="library-dialog-danger" id="lib-edit-danger"></div>
                <div class="build-error" id="lib-edit-error" hidden></div>`,
            actions: [
                { label: t('Cancel'), id: 'lib-edit-cancel', onClick: () => this._editDialog.close(null) },
                { label: t('Save'), kind: 'primary', id: 'lib-edit-save', onClick: () => save() },
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
                const updated = await LibraryAPI.update(lib.id, name, descEl.value.trim(), dlg.querySelector('#lib-edit-lang').value);
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

        this._wireSharing(dlg, lib, onSaved);
        this._wireMaintenance(dlg, lib);
        this._renderLibraryDanger(dlg, lib, close);
        return dlg;
    }

    // Sharing takes effect at once, like maintenance: it writes or removes the
    // marker in the library's folder, which Save has nothing to do with.
    _wireSharing(dlg, lib, onSaved) {
        const wrap = dlg.querySelector('#lib-edit-share');
        const hint = dlg.querySelector('#lib-edit-share-hint');
        const explain = t('Shared, the folder carries a small file, .unterlumen-library.json, with this library\u2019s name. Another installation that adds the folder gets the same library; each one keeps its own index.');
        if (lib.missing) {
            hint.textContent = `${lib.sourcePath} is not connected. Connect its disk or NAS to change this.`;
            return;
        }
        if (lib.joinOffer) {
            hint.textContent = t('Another installation shares this folder as “{name}”. Join it in the library.', { name: lib.joinOffer.name });
            return;
        }
        hint.textContent = explain;
        const toggle = Toggle.create(wrap, {
            initial: Boolean(lib.shared),
            labelOn: t('Shared'),
            labelOff: t('This installation only'),
            onChange: async (on) => {
                hint.textContent = on ? 'Sharing\u2026' : t('Stopping\u2026');
                hint.classList.remove('form-hint--error');
                try {
                    const updated = await LibraryAPI.setShared(lib.id, on);
                    lib.shared = updated.shared;
                    this.tab._cachedLibs = null;
                    hint.textContent = explain;
                    if (onSaved) onSaved(updated);
                } catch (err) {
                    toggle.setState(!on);
                    hint.textContent = err.message;
                    hint.classList.add('form-hint--error');
                }
            },
        });
    }

    // All six maintenance runs live here rather than behind a chevron on the
    // Libraries row: they are rare, they act on this one library, and the
    // dialog has room to say what each one does while it runs.
    _wireMaintenance(dlg, lib) {
        const actions = dlg.querySelector('#lib-edit-maint-actions');
        const progress = dlg.querySelector('#lib-edit-maint-progress');
        const confirm = dlg.querySelector('.lib-maint-confirm');
        const buttons = [...actions.querySelectorAll('[data-maint]')];
        // Each button is a few library jobs, one after the other.
        const steps = {
            scan: [['scanNew', t('Scanning')], ['cleanup', t('Checking for photos that are gone')], ['regenMissingPreviews', t('Making missing previews')]],
            again: [['reindex', t('Reading every photo')], ['analyseAgain', t('Analysing')]],
        };
        const run = async (which) => {
            buttons.forEach(b => { b.disabled = true; });
            progress.hidden = false;
            const activity = Activity.in(progress, `${steps[which][0][1]}…`);
            try {
                // Only the last job's end ends the line; a job that fails
                // ends it and the rest.
                for (const [i, [job, words]] of steps[which].entries()) {
                    const last = i === steps[which].length - 1;
                    let failed = false;
                    activity.busy(`${words}…`);
                    await LibraryAPI[job](lib.id, (p) => {
                        if (p.error) failed = true;
                        if (p.finished && !p.error && !last) return;
                        showLibraryProgress(activity, p, `${words}…`);
                    });
                    if (failed) break;
                }
                this.tab._cachedLibs = null;
            } catch (err) {
                activity.fail(t('It stopped: {error}', { error: err.message }));
            } finally {
                buttons.forEach(b => { b.disabled = false; });
            }
        };
        actions.querySelector('[data-maint="scan"]').addEventListener('click', () => run('scan'));
        actions.querySelector('[data-maint="again"]').addEventListener('click', () => { confirm.hidden = false; });
        confirm.addEventListener('click', (e) => {
            const answer = e.target.closest('[data-maint-confirm]')?.dataset.maintConfirm;
            if (!answer) return;
            confirm.hidden = true;
            if (answer === 'yes') run('again');
        });
    }

    _renderLibraryDanger(dlg, lib, closeDialog) {
        const wrap = dlg.querySelector('#lib-edit-danger');
        wrap.innerHTML = '<button class="btn btn-sm btn-danger" id="lib-delete-start">' + t('Delete library…') + '</button>';
        wrap.querySelector('#lib-delete-start').addEventListener('click', () => {
            wrap.innerHTML = `
                <p class="gal-danger-question">${t('Delete “{name}”? This removes the index and its thumbnails. The photos in {folder} are not touched.', { name: escapeHtml(lib.name), folder: escapeHtml(lib.sourcePath) })}</p>
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="lib-delete-cancel">${t('Keep it')}</button>
                    <button class="btn btn-sm btn-danger" id="lib-delete-confirm">${t('Delete library')}</button>
                </div>`;
            wrap.querySelector('#lib-delete-cancel').addEventListener('click', () => this._renderLibraryDanger(dlg, lib, closeDialog));
            wrap.querySelector('#lib-delete-confirm').addEventListener('click', async (e) => {
                Activity.button(e.currentTarget, t('Deleting…'));
                wrap.querySelectorAll('button').forEach(b => { b.disabled = true; });
                try {
                    await LibraryAPI.delete(lib.id);
                    this.tab._cachedLibs = null;
                    this.tab.currentLibrary = null;
                    closeDialog();
                    this.tab.render();
                    App.refreshLibraryVisibility();
                } catch (err) {
                    wrap.innerHTML = `<div class="build-error">${t('Could not delete it: {error}', { error: escapeHtml(err.message) })}</div>`;
                }
            });
        });
    }

    create(prefillPath) {
        this._createDialog = new Dialog({
            title: t('New library'),
            size: 'md',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="lib-dlg-name">${t('Name')}</label>
                <input class="library-dialog-input" id="lib-dlg-name" type="text" placeholder="${t('My Photos')}" autocomplete="off">
                <label class="library-dialog-label" for="lib-dlg-path">${t('Source folder')}</label>
                <div class="library-dialog-path-row">
                    <input class="library-dialog-input" id="lib-dlg-path" type="text" placeholder="/Fotos/2024">
                    <button class="btn btn-sm" id="lib-dlg-choose" type="button">${t('Choose…')}</button>
                </div>
                <label class="library-dialog-label" for="lib-dlg-desc">${t('Description (optional)')}</label>
                <input class="library-dialog-input" id="lib-dlg-desc" type="text" placeholder="">
                <label class="library-dialog-label" for="lib-dlg-lang">${t('Keywords in the sidecars')}</label>
                <select class="btn btn-sm select-btn" id="lib-dlg-lang">${keywordLanguageOptions(I18n.lang)}</select>
                <span class="form-hint">${t('Things found in the photos are written as keywords into the .xmp files beside them, which Lightroom, Apple Photos and other programs read too. This is their language. It can be changed later: Unterlumen then rewrites its own keywords, never yours.')}</span>
                <div class="library-dialog-field">
                    <span class="library-dialog-label">${t('What a library does with the folder')}</span>
                    <ul class="library-dialog-facts">
                        <li>${t('Your photos stay where they are. Nothing is copied or uploaded.')}</li>
                        <li>${t('Every photo is read once. A large folder takes a few minutes.')}</li>
                        <li>${t('When you give a photo a title or a field, or publish it, a small file of the same name ending in .xmp is put beside it. Other photo programs do the same and read it.')}</li>
                        <li>${t('Set location and Rename change the photo files themselves.')}</li>
                        <li>${t('Removing the library later leaves the photos and these files as they are.')}</li>
                    </ul>
                </div>
                <div class="library-dialog-note" hidden></div>
                <div class="library-dialog-progress" id="lib-dlg-progress" style="display:none"></div>`,
            actions: [
                { label: t('Cancel'), id: 'lib-dlg-cancel', onClick: () => this._createDialog.close(null) },
                { label: t('Create & index'), kind: 'primary', id: 'lib-dlg-create', onClick: () => create() },
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
            const picked = await new FolderPicker().open(current, { title: t('Folder for the new library') });
            if (picked === null) return;
            pathEl.value = '/' + picked;
            if (!nameEl.value.trim()) nameEl.value = picked.split('/').filter(Boolean).pop() || '';
            nameEl.focus();
            showShared();
        });
        pathEl.addEventListener('change', () => showShared());

        // A folder another installation shares is added as that library, under
        // its name, so name and description come from there.
        const noteEl = dlg.querySelector('.library-dialog-note');
        const showShared = async () => {
            const { marker } = await LibraryAPI.marker(stripQuotes(pathEl.value.trim()));
            nameEl.readOnly = descEl.readOnly = Boolean(marker);
            if (!marker) {
                noteEl.hidden = true;
                return;
            }
            nameEl.value = marker.name;
            descEl.value = marker.description || '';
            noteEl.hidden = false;
            noteEl.textContent = t('Another installation shares this folder as “{name}”. It is added as that library; this installation reads the photos once for its own index.', { name: marker.name });
        };

        if (prefillPath) {
            pathEl.value = prefillPath;
            nameEl.value = prefillPath.split('/').filter(Boolean).pop() || '';
            nameEl.focus();
            showShared();
        } else {
            nameEl.focus();
        }

        const create = async () => {
            const name = nameEl.value.trim();
            const path = stripQuotes(pathEl.value.trim());
            pathEl.value = path; // show cleaned value
            if (!name || !path) {
                progressEl.style.display = '';
                progressEl.textContent = t('A library needs a name and a folder to read.');
                (name ? pathEl : nameEl).focus();
                return;
            }

            createBtn.disabled = true;
            progressEl.style.display = '';
            const activity = Activity.in(progressEl, t('Creating the library…'));

            try {
                const lib = await LibraryAPI.create(name, descEl.value.trim(), path, dlg.querySelector('#lib-dlg-lang').value);
                this.tab._cachedLibs = null;
                activity.busy(t('Looking for photos…'));
                let failed = false;
                await LibraryAPI.reindex(lib.id, (p) => {
                    failed = failed || !!p.error;
                    showLibraryProgress(activity, p, t('Looking for photos…'));
                });
                // The library exists even if indexing stopped, so creating it
                // again would make a second one. The reason stays readable and
                // the one thing left to do is open what was created.
                if (failed) {
                    this._createDialog.setActions([{
                        label: t('Open the library'), kind: 'primary',
                        onClick: () => { this._createDialog.close(null); App.refreshLibraryVisibility(); this.tab._openLibrary(lib); },
                    }]);
                    return;
                }
                this._createDialog.close(null);
                App.refreshLibraryVisibility();
                this.tab._openLibrary(lib);
            } catch (err) {
                activity.fail(t('The library was not created: {error}', { error: err.message }));
                createBtn.disabled = false;
            }
        };
    }

    // Open the create dialog pre-filled with a known path (from Tools menu).
}
