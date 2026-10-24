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
                ${placeHeaderHTML({ title: t('Settings') })}
                <div class="gal-body settings-body">
                    <p class="settings-guide">${t('New to Unterlumen, or not sure what a library or a destination is? {link} explains it on one page.', { link: placeLink('guide', 'guide', t('How Unterlumen works')) })}</p>
                    ${App.config?.canSetup ? `
                    <div class="form-field">
                        <span class="form-label">${t('Shared folder')}</span>
                        <span class="settings-cache" id="settings-shared-dir">…</span>
                        <span class="form-hint">${placeLink('setup', 'setup', t('Change the shared folder or the data folder'))}</span>
                    </div>` : ''}

                    ${App.config?.serverRole ? `
                    <div class="form-field">
                        <span class="form-label">${t('Sharing')}</span>
                        <div class="tools-install" id="settings-sharing">…</div>
                    </div>` : ''}

                    <div class="form-field">
                        <span class="form-label">${t('Theme')}</span>
                        <div class="seg" role="group" aria-label="${t('Theme')}" id="settings-theme">
                            <button data-theme-set="light">${t('Light')}</button>
                            <button data-theme-set="auto">${t('System')}</button>
                            <button data-theme-set="dark">${t('Dark')}</button>
                        </div>
                    </div>

                    <div class="form-field">
                        <span class="form-label">${t('Language')}</span>
                        <div class="seg" role="group" aria-label="${t('Language')}" id="settings-language">
                            <button data-language="auto">${t('Browser')}</button>
                            ${Object.entries(I18n.languages).map(([code, name]) => `<button data-language="${code}" lang="${code}">${name}</button>`).join('')}
                        </div>
                        <span class="form-hint">${t('The language of this browser\'s Unterlumen. Browser follows the language the browser is set to. The keywords written into the sidecars have a language of their own, chosen per library.')}</span>
                    </div>

                    <div class="form-field">
                        <span class="form-label">${t('Folder previews')}</span>
                        <div id="settings-thumb-quality-wrap"></div>
                        <span class="form-hint">${t('Only for Folders. Fast shows a photo\'s library thumbnail when it is in a library, otherwise the small preview stored in the file, JPEG and HEIF alike: quick, even over the network. Sharp decodes every photo at the size your screen needs: crisper, slower, and the cache grows faster.')}</span>
                    </div>

                    <div class="form-field" id="settings-storage">
                        <span class="form-label">${t('Storage')}</span>
                        <span class="form-hint">${t('Measuring…')}</span>
                    </div>

                    <div class="form-field">
                        <span class="form-label">${t('Helper programs')}</span>
                        <span class="settings-tools" id="settings-tools">${t('Checking…')}</span>
                        <div><button class="btn btn-sm" id="settings-check-deps">${t('What these are for')}</button></div>
                        <div class="tools-install" id="settings-tools-install"></div>
                    </div>
                </div>
            </div>`;

        this._wireTheme();
        this._wireLanguage();
        this._wireToggles();
        new SettingsStorage(this.container.querySelector('#settings-storage')).load();
        this._wireTools();
        if (App.config?.serverRole) this._wireSharing();
        if (App.config?.canSetup) this._showSharedDir();
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

    // The language is this browser's: a choice loads the page again in it.
    _wireLanguage() {
        const seg = this.container.querySelector('#settings-language');
        const chosen = I18n.chosen();
        for (const btn of seg.querySelectorAll('[data-language]')) {
            btn.setAttribute('aria-pressed', btn.dataset.language === chosen ? 'true' : 'false');
        }
        seg.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-language]');
            if (btn && btn.dataset.language !== chosen) I18n.choose(btn.dataset.language);
        });
    }

    _wireToggles() {
        const savedQuality = localStorage.getItem('thumbnail-quality') || 'standard';
        Toggle.create(this.container.querySelector('#settings-thumb-quality-wrap'), {
            initial: savedQuality === 'high',
            labelOn: t('Sharp'),
            labelOff: t('Fast'),
            onChange: (on) => App.theme.setQuality(on ? 'high' : 'standard'),
        });

    }

    // The installed app's shared folder, shown here and changed in the setup.
    async _showSharedDir() {
        const el = this.container.querySelector('#settings-shared-dir');
        try {
            const { sharedDir } = await API.setup();
            el.textContent = sharedDir || t('Not shared');
        } catch (err) {
            el.textContent = t('Could not read the settings: {error}', { error: err.message });
        }
    }

    // A server's folder is fixed, so sharing is the one thing to set up
    // there. A NAS that shares first is found by the Mac's setup (ADR-0042).
    async _wireSharing() {
        const el = this.container.querySelector('#settings-sharing');
        let sharedDir;
        try {
            ({ sharedDir } = await API.sharing());
        } catch (err) {
            el.innerHTML = `<span class="gal-detail-error">${t('Could not read the sharing: {error}', { error: escapeHtml(err.message) })}</span>`;
            return;
        }
        if (sharedDir) {
            el.innerHTML = `<span class="form-hint">${t('Destinations and galleries are shared through {folder}. Another installation — Unterlumen on a Mac, say — uses them when it chooses this folder as its shared folder.', { folder: `<span class="mono">${escapeHtml(sharedDir)}</span>` })}</span>`;
            return;
        }
        el.innerHTML = `<span class="form-hint">${t('If another installation shows the same photos — Unterlumen on a Mac with this folder opened over the network, say — both can use the same destinations and galleries. They are then kept in this installation’s folder, in {folder}.', { folder: '<span class="mono">.unterlumen-shared</span>' })}</span>
            <div><button class="btn btn-sm" id="settings-share">${t('Share with another installation')}</button></div>
            <div id="settings-share-result"></div>`;
        el.querySelector('#settings-share').addEventListener('click', async (e) => {
            const restore = Activity.button(e.currentTarget, t('Sharing…'));
            try {
                await API.share();
            } catch (err) {
                restore();
                el.querySelector('#settings-share-result').innerHTML = `<div class="gal-detail-error">${escapeHtml(err.message)}</div>`;
                return;
            }
            this._wireSharing();
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
                el.textContent = t('The helper check did not answer.');
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
