// setup-place.js — the setup (#setup): where destinations are shared with
// another installation, and where this one keeps its own data. Libraries name
// their own folders and are added in Libraries (ADR-0047). Settings links
// here (ADR-0042).
//
// A place, not a dialog: it keeps its address, and Settings comes back to it.

class SetupPane {
    constructor(container) {
        this.container = container;
        this._sharedPath = '';   // as the folder picker names it; '' for not shared
        this._sharedLabel = '';  // as the disk names it, to show
    }

    async render() {
        Activity.in(this.container, 'Reading the settings…');
        let setup;
        try {
            setup = await API.setup();
        } catch (err) {
            this.container.innerHTML = `<div class="gal-detail-error">Could not read the settings: ${escapeHtml(err.message)}</div>`;
            return;
        }
        this._setup = setup;
        this._sharedPath = setup.sharedPath;
        this._sharedLabel = setup.sharedDir;
        this._draw();
    }

    _draw() {
        this.container.innerHTML = `
            <div class="settings-pane">
                <div class="gal-head">
                    <h1 class="gal-title">Sharing and data folder</h1>
                </div>
                <div class="gal-body settings-body">
                    <p class="setup-lede">Each library names its own folder; add them in ${placeLink('library', 'libraries', 'Libraries')}. ${placeLink('guide', 'guide', 'How Unterlumen works')} explains the rest.</p>

                    <div class="form-field">
                        <span class="form-label">Shared folder</span>
                        <span class="setup-path" id="setup-shared"></span>
                        <span class="form-hint" id="setup-shared-hint"></span>
                        <div class="setup-row">
                            <button class="btn btn-sm" id="setup-choose">Choose…</button>
                            <button class="btn btn-sm" id="setup-unshare">Do not share</button>
                        </div>
                    </div>

                    <details class="setup-advanced">
                        <summary>Data folder</summary>
                        <div class="form-field">
                            <label class="form-label" for="setup-libdir">Where Unterlumen keeps its libraries, thumbnails and what Publish builds</label>
                            <input class="form-input setup-path" id="setup-libdir" spellcheck="false" autocomplete="off">
                            <span class="form-hint">It stays on this computer. Leave it as it is unless the disk is too small.</span>
                        </div>
                    </details>

                    <div class="form-field">
                        <span class="form-label">Helper programs</span>
                        <span class="settings-tools" id="setup-tools">Checking…</span>
                        <div><button class="btn btn-sm" id="setup-deps">What these are for</button></div>
                        <div class="tools-install" id="setup-tools-install"></div>
                    </div>

                    <div id="setup-error"></div>
                    <div class="setup-actions">
                        <button class="btn" id="setup-cancel">Cancel</button>
                        <button class="btn btn-accent" id="setup-save">Save</button>
                    </div>
                </div>
            </div>`;

        this.container.querySelector('#setup-libdir').value = this._setup.libDir || this._setup.defaultLibDir;
        this.container.querySelector('#setup-choose').addEventListener('click', () => this._choose());
        this.container.querySelector('#setup-unshare').addEventListener('click', () => this._showShared('', ''));
        this.container.querySelector('#setup-save').addEventListener('click', (e) => this._save(e.currentTarget));
        this.container.querySelector('#setup-cancel').addEventListener('click', () => App.setMode('settings'));
        this.container.querySelector('#setup-deps').addEventListener('click', () => new DepsModal().open(App.toolsStatus));
        const showTools = (status) => {
            this.container.querySelector('#setup-tools').textContent = status ? toolsSummary(status) : 'The helper check did not answer.';
            mountToolsInstall(this.container.querySelector('#setup-tools-install'), status, showTools);
        };
        showTools(App.toolsStatus);
        this._showShared(this._sharedPath, this._sharedLabel, this._sharedPath ? 'kept' : '');
    }

    async _choose() {
        const picked = await new FolderPicker().open(this._sharedPath || this._setup.homePath, {
            title: 'Folder both installations see',
            disk: true,
            home: this._setup.homePath,
        });
        if (picked === null) return;
        const label = /^[A-Za-z]:/.test(picked) ? picked : '/' + picked;
        let found = '', canShare = true;
        try {
            ({ sharedDir: found, canShare } = await API.setupShared(picked));
        } catch { /* saving will say if the folder is gone */ }
        if (!found && !canShare) {
            this._showShared(this._sharedPath, this._sharedLabel, this._sharedPath ? 'kept' : '');
            this._hint('The top of a disk cannot be shared: no other installation sees it. Choose a folder on the disk or the NAS both installations use.', true);
            return;
        }
        this._showShared(picked, found || label, found ? 'found' : 'new');
    }

    // how: 'kept' (in use now), 'found' (another installation shares there),
    // 'new' (made on Save), '' (not shared).
    _showShared(path, label, how = '') {
        this._sharedPath = path;
        this._sharedLabel = label;
        const el = this.container.querySelector('#setup-shared');
        el.textContent = path ? label : 'Not shared';
        el.classList.toggle('setup-path--none', !path);
        this.container.querySelector('#setup-unshare').hidden = !path;
        const hints = {
            kept: 'Destinations and galleries are shared through this folder with every installation that uses it.',
            found: 'Another installation shares through this folder. Its destinations and galleries are used here too.',
            new: 'A folder .unterlumen-shared is made in it, and this installation’s destinations are copied there. Choose the same folder on the other installation.',
            '': 'Destinations and galleries stay on this computer. To use them on a second installation, for example a NAS, choose a folder both see.',
        };
        this._hint(hints[how], false);
    }

    _hint(text, isError) {
        const hint = this.container.querySelector('#setup-shared-hint');
        hint.textContent = text;
        hint.classList.toggle('form-hint--error', isError);
    }

    async _save(btn) {
        const errorEl = this.container.querySelector('#setup-error');
        errorEl.innerHTML = '';
        const libDir = this.container.querySelector('#setup-libdir').value.trim();
        const restore = Activity.button(btn, 'Saving…');
        try {
            await API.saveSetup({
                libDir: libDir === this._setup.defaultLibDir ? '' : libDir,
                sharedPath: this._sharedPath,
            });
        } catch (err) {
            restore();
            errorEl.innerHTML = `<div class="gal-detail-error">${escapeHtml(err.message)}</div>`;
            return;
        }
        // The libraries and destinations behind the app changed, so it starts again.
        location.hash = '#settings';
        location.reload();
    }
}
