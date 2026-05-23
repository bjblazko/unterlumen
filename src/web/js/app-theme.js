// ThemeManager — owns theme and thumbnail-quality preferences

class ThemeManager {
    constructor(app) {
        this._app = app;
    }

    init() {
        const saved = localStorage.getItem('theme') || 'auto';
        this._apply(saved);
        this._updateButtons(saved);

        window.matchMedia('(prefers-color-scheme: dark)')
            .addEventListener('change', () => {
                if ((localStorage.getItem('theme') || 'auto') === 'auto') {
                    this._apply('auto');
                }
            });
    }

    _apply(preference) {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        const resolved = preference === 'auto' ? (prefersDark ? 'dark' : 'light') : preference;
        document.documentElement.dataset.theme = resolved;
    }

    _updateButtons(preference) {
        document.querySelectorAll('[data-theme-set]').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.themeSet === preference);
        });
    }

    setQuality(quality) {
        localStorage.setItem('thumbnail-quality', quality);
        const app = this._app;
        if (app.browsePane) app.browsePane.reloadThumbnails();
        if (app.commander) {
            if (app.commander.leftPane) app.commander.leftPane.reloadThumbnails();
            if (app.commander.rightPane) app.commander.rightPane.reloadThumbnails();
        }
    }
}
