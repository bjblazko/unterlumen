// setup-place.js — the setup (#setup): which folder holds the photos, and
// whether destinations are shared with another installation that shows the
// same photos. The installed app opens here on its first start; afterwards
// Settings links here to change it (ADR-0042).
//
// A place, not a dialog: on a first start there is nothing else to show, and
// it keeps its address for later.

class SetupPane {
    constructor(container) {
        this.container = container;
        this._photosPath = '';   // as the folder picker names it
        this._photosDir = '';    // as the disk names it, to show
        this._shareToggle = null;
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
        this._photosPath = setup.photosPath;
        this._photosDir = setup.photosDir;
        this._draw();
    }

    _draw() {
        const first = !this._setup.photosDir;
        this.container.innerHTML = `
            <div class="settings-pane">
                <div class="gal-head">
                    <h1 class="gal-title">${first ? 'Set up Unterlumen' : 'Photo folder and sharing'}</h1>
                </div>
                <div class="gal-body settings-body">
                    ${this._setup.problem ? `<div class="gal-detail-error">${escapeHtml(this._setup.problem)}</div>` : ''}
                    <p class="setup-lede">Unterlumen shows the photos in one folder and in every folder inside it. Choose the folder that holds your photos. ${placeLink('guide', 'guide', 'How Unterlumen works')} explains the rest.</p>

                    <div class="form-field">
                        <span class="form-label">Photo folder</span>
                        <span class="setup-path" id="setup-photos"></span>
                        <div><button class="btn btn-sm" id="setup-choose">Choose…</button></div>
                    </div>

                    <div class="form-field" id="setup-share-field" hidden>
                        <span class="form-label">Share with another installation</span>
                        <div id="setup-share-toggle"></div>
                        <span class="form-hint" id="setup-share-hint"></span>
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
                        ${first ? '' : '<button class="btn" id="setup-cancel">Cancel</button>'}
                        <button class="btn btn-accent" id="setup-save">${first ? 'Set up Unterlumen' : 'Save'}</button>
                    </div>
                </div>
            </div>`;

        this.container.querySelector('#setup-libdir').value = this._setup.libDir || this._setup.defaultLibDir;
        this.container.querySelector('#setup-choose').addEventListener('click', () => this._choose());
        this.container.querySelector('#setup-save').addEventListener('click', (e) => this._save(e.currentTarget));
        this.container.querySelector('#setup-cancel')?.addEventListener('click', () => App.setMode('settings'));
        this.container.querySelector('#setup-deps').addEventListener('click', () => new DepsModal().open(App.toolsStatus));
        const showTools = (status) => {
            this.container.querySelector('#setup-tools').textContent = status ? toolsSummary(status) : 'The helper check did not answer.';
            mountToolsInstall(this.container.querySelector('#setup-tools-install'), status, showTools);
        };
        showTools(App.toolsStatus);
        this._showPhotos();
    }

    async _choose() {
        const picked = await new FolderPicker().open(this._photosPath || this._setup.homePath, {
            title: 'Folder that holds your photos',
            disk: true,
            home: this._setup.homePath,
        });
        if (picked === null) return;
        this._photosPath = picked;
        this._photosDir = null; // named by the picker path until saved
        this._showPhotos();
    }

    _photosLabel() {
        if (this._photosDir) return this._photosDir;
        if (!this._photosPath && this._photosDir === '') return null;
        return /^[A-Za-z]:/.test(this._photosPath) ? this._photosPath : '/' + this._photosPath;
    }

    // The photo folder decides what sharing means: a folder another
    // installation already shares is joined, any other one can start sharing.
    async _showPhotos() {
        const label = this._photosLabel();
        const el = this.container.querySelector('#setup-photos');
        el.textContent = label ?? 'Not chosen yet';
        el.classList.toggle('setup-path--none', label === null);
        const field = this.container.querySelector('#setup-share-field');
        field.hidden = label === null;
        if (label === null) return;

        let found = '';
        try {
            found = (await API.setupShared(this._photosPath)).sharedDir;
        } catch { /* said as "not shared yet"; saving will tell if the folder is gone */ }
        this._drawShare(found);
    }

    _drawShare(found) {
        const elsewhere = !found && this._setup.sharedDir && !this._setup.sharedDir.startsWith(this._photosLabel());
        const hint = this.container.querySelector('#setup-share-hint');
        if (found) {
            hint.innerHTML = 'Another Unterlumen installation works in this folder, for example on a NAS. Shared, its destinations and galleries are used here too.';
        } else if (elsewhere) {
            hint.innerHTML = `Destinations and galleries are shared through <span class="setup-path">${escapeHtml(this._setup.sharedDir)}</span>.`;
        } else {
            hint.innerHTML = `If another installation shows the same photos, for example on a NAS, both can use the same destinations and galleries. They are then kept in the photo folder, in <span class="setup-path">.unterlumen-shared</span>, where the other one finds them.`;
        }
        const wrap = this.container.querySelector('#setup-share-toggle');
        wrap.innerHTML = '';
        this._shareToggle = Toggle.create(wrap, {
            initial: Boolean(found || elsewhere || this._setup.sharedDir),
            labelOn: 'Shared',
            labelOff: 'This installation only',
        });
    }

    async _save(btn) {
        const errorEl = this.container.querySelector('#setup-error');
        errorEl.innerHTML = '';
        if (this._photosLabel() === null) {
            errorEl.innerHTML = '<div class="gal-detail-error">Choose the folder that holds your photos first.</div>';
            return;
        }
        const libDir = this.container.querySelector('#setup-libdir').value.trim();
        const restore = Activity.button(btn, 'Setting up…');
        try {
            await API.saveSetup({
                photosPath: this._photosPath,
                libDir: libDir === this._setup.defaultLibDir ? '' : libDir,
                share: this._shareToggle?.state() ?? false,
            });
        } catch (err) {
            restore();
            errorEl.innerHTML = `<div class="gal-detail-error">${escapeHtml(err.message)}</div>`;
            return;
        }
        // Everything behind the app changed — the browse root, the libraries,
        // the destinations — so it starts again, in Folders.
        location.hash = '#folders';
        location.reload();
    }
}
