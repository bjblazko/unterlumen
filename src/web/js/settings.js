// settings.js — Settings as a place, not a dropdown in a corner.
//
// The settings were a menu hanging off a header button: four sections and a
// dialog squeezed into a popover. They are few enough to show all at once on
// a page, where each one has room for the sentence that explains it.

class SettingsPane {
    constructor(container) {
        this.container = container;
    }

    render() {
        this.container.innerHTML = `
            <div class="settings-pane">
                <div class="gal-head">
                    <h1 class="gal-title">Settings</h1>
                    <span class="gal-group-spacer"></span>
                    <button class="btn btn-sm" id="settings-done">Done</button>
                </div>
                <div class="gal-body settings-body">
                    <p class="settings-guide">New to Unterlumen, or not sure what a library or a destination is? ${placeLink('guide', 'guide', 'How Unterlumen works')} explains it on one page.</p>
                    ${App.config?.canSetup ? `
                    <div class="form-field">
                        <span class="form-label">Photo folder</span>
                        <span class="settings-cache">${escapeHtml(App.config.boundary)}</span>
                        <span class="form-hint">${placeLink('setup', 'setup', 'Change the photo folder or sharing')}</span>
                    </div>` : ''}

                    <div class="form-field">
                        <span class="form-label">Theme</span>
                        <div class="seg" role="group" aria-label="Theme" id="settings-theme">
                            <button data-theme-set="light">Light</button>
                            <button data-theme-set="auto">System</button>
                            <button data-theme-set="dark">Dark</button>
                        </div>
                    </div>

                    <div class="form-field">
                        <span class="form-label">Thumbnail quality</span>
                        <div id="settings-thumb-quality-wrap"></div>
                        <span class="form-hint">High decodes the full image at the size your screen needs; Standard uses the thumbnail stored in the file, which is faster and smaller.</span>
                    </div>

                    <div class="form-field">
                        <span class="form-label">Interface</span>
                        <div id="settings-hide-ui-wrap"></div>
                        <span class="form-hint">Hiding the interface leaves only the photos. Press H to bring it back.</span>
                    </div>

                    <div class="form-field">
                        <span class="form-label">Cache</span>
                        <span class="settings-cache" id="settings-cache-size">…</span>
                        <span class="settings-cache settings-cache-path" id="settings-cache-path"></span>
                        <div><button class="btn btn-sm" id="settings-clear-cache">Clear cache…</button></div>
                        <div id="settings-cache-confirm"></div>
                    </div>

                    <div class="form-field">
                        <span class="form-label">Helper programs</span>
                        <span class="settings-tools" id="settings-tools">Checking…</span>
                        <div><button class="btn btn-sm" id="settings-check-deps">What these are for</button></div>
                        <div class="tools-install" id="settings-tools-install"></div>
                    </div>
                </div>
            </div>`;

        this._wireTheme();
        this._wireToggles();
        this._wireCache();
        this._wireTools();
        this.container.querySelector('#settings-done').addEventListener('click', () => App.leaveSettings());
    }

    _wireTheme() {
        const preference = localStorage.getItem('theme') || 'auto';
        const seg = this.container.querySelector('#settings-theme');
        const mark = (value) => {
            for (const btn of seg.querySelectorAll('[data-theme-set]')) {
                btn.setAttribute('aria-pressed', btn.dataset.themeSet === value ? 'true' : 'false');
            }
        };
        mark(preference);
        seg.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-theme-set]');
            if (!btn) return;
            const value = btn.dataset.themeSet;
            localStorage.setItem('theme', value);
            App.theme._apply(value);
            mark(value);
        });
    }

    _wireToggles() {
        const savedQuality = localStorage.getItem('thumbnail-quality') || 'standard';
        Toggle.create(this.container.querySelector('#settings-thumb-quality-wrap'), {
            initial: savedQuality === 'high',
            labelOn: 'High',
            labelOff: 'Standard',
            onChange: (on) => App.theme.setQuality(on ? 'high' : 'standard'),
        });

        App._hideUiToggle = Toggle.create(this.container.querySelector('#settings-hide-ui-wrap'), {
            initial: !App.uiHidden,
            labelOn: 'Shown',
            labelOff: 'Hidden',
            onChange: () => App.toggleUIVisibility(),
        });
    }

    _wireCache() {
        const sizeEl = this.container.querySelector('#settings-cache-size');
        const pathEl = this.container.querySelector('#settings-cache-path');
        // Thousands of megabytes are hard to read; say what a person would say.
        const readableSize = (bytes) => bytes >= 1073741824
            ? (bytes / 1073741824).toFixed(1) + ' GB'
            : (bytes / 1048576).toFixed(1) + ' MB';
        const load = () => API.cacheInfo().then(info => {
            sizeEl.textContent = readableSize(info.bytes);
            pathEl.textContent = info.path;
        }).catch(() => {
            sizeEl.textContent = 'Could not read the cache size.';
        });
        load();

        const confirmWrap = this.container.querySelector('#settings-cache-confirm');
        this.container.querySelector('#settings-clear-cache').addEventListener('click', () => {
            // Clearing costs nothing but time, so the question is short — but
            // it is still asked, and asked here rather than in a browser box.
            confirmWrap.innerHTML = `
                <p class="gal-danger-question">Clear the cached previews? They are rebuilt as you browse; the photos are untouched.</p>
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="settings-cache-cancel">Keep them</button>
                    <button class="btn btn-sm btn-danger" id="settings-cache-confirm-btn">Clear cache</button>
                </div>`;
            confirmWrap.querySelector('#settings-cache-cancel').addEventListener('click', () => { confirmWrap.innerHTML = ''; });
            confirmWrap.querySelector('#settings-cache-confirm-btn').addEventListener('click', async (e) => {
                confirmWrap.querySelectorAll('button').forEach(b => { b.disabled = true; });
                Activity.button(e.currentTarget, 'Clearing…');
                try {
                    await API.cacheClear();
                    confirmWrap.innerHTML = '<p class="form-hint">Cache cleared.</p>';
                } catch (err) {
                    confirmWrap.innerHTML = `<div class="gal-detail-error">Could not clear the cache: ${escapeHtml(err.message)}</div>`;
                }
                load();
            });
        });
    }

    async _wireTools() {
        const el = this.container.querySelector('#settings-tools');
        this.container.querySelector('#settings-check-deps')
            .addEventListener('click', () => new DepsModal().open(App.toolsStatus));

        let status = App.toolsStatus;
        if (!status) {
            try {
                status = await API.toolsCheck();
            } catch {
                el.textContent = 'The helper check did not answer.';
                return;
            }
        }
        const show = (s) => {
            el.textContent = toolsSummary(s);
            mountToolsInstall(this.container.querySelector('#settings-tools-install'), s, show);
        };
        show(status);
    }
}
