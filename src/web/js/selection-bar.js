// selection-bar.js — one bar for everything you can do to a selection.
//
// Actions on a selection used to be spread over a Tools dropdown, header
// buttons and keyboard shortcuts, several of them greyed out with no reason
// given. They now live in one bar that appears when something is selected and
// goes away when nothing is (ADR-0029's screen rules; see the redesign doc).
//
// The bar is the same component everywhere; only the action list differs by
// context, because the contexts genuinely differ: "Add to gallery" needs
// library photos, so it exists in a library and not in a plain folder.

// `folders`: the action also works on selected folders. The others work on
// photos only and say so when only folders are selected.
const SELECTION_ACTIONS = {
    collect: { label: 'Add to gallery…', primary: true },
    export: { label: 'Export…' },
    download: { label: 'Download', folders: true },
    rename: { label: 'Rename…' },
    location: { label: 'Set location…' },
    organize: { label: 'Show in Organize', folders: true },
    mark: { label: 'Mark for deletion' },
};

const PHOTOS_ONLY = 'Works on photos. Select photos to use it.';

class SelectionBar {
    // container: the element the bar is appended to (it renders nothing until
    //            something is selected).
    // actions:   ordered list of keys from SELECTION_ACTIONS.
    // onAction:  (key) => void. "clear" is handled by the caller too, so Esc
    //            and the button end up in the same place.
    constructor(container, { actions, onAction }) {
        this.container = container;
        this.actions = actions;
        this.onAction = onAction;
        this._count = 0;
        this._el = null;
    }

    // count: how many items are selected. Zero removes the bar.
    // onlyFolders: nothing but folders is selected.
    update(count, { disabled = {}, onlyFolders = false } = {}) {
        if (onlyFolders) {
            disabled = { ...disabled };
            for (const key of this.actions) {
                if (!SELECTION_ACTIONS[key].folders) disabled[key] ??= PHOTOS_ONLY;
            }
        }
        this._count = count;
        if (!count) {
            this._el?.remove();
            this._el = null;
            return;
        }
        if (!this._el) {
            this._el = document.createElement('div');
            this._el.className = 'selection-bar';
            this._el.setAttribute('role', 'toolbar');
            this._el.setAttribute('aria-label', 'Selection');
            this.container.appendChild(this._el);
            this._el.addEventListener('click', (e) => {
                const btn = e.target.closest('[data-action]');
                if (btn) this.onAction(btn.dataset.action);
            });
        }
        this._el.innerHTML = `
            <span class="selection-bar-count">${count} selected</span>
            ${this.actions.map(key => {
                const a = SELECTION_ACTIONS[key];
                const why = disabled[key];
                return `<button class="btn btn-sm${a.primary ? ' btn-accent' : ''}" data-action="${key}"${why ? ` disabled title="${escapeHtml(why)}"` : ''}>${escapeHtml(a.label)}</button>`;
            }).join('')}
            <span class="selection-bar-spacer"></span>
            <button class="btn btn-sm" data-action="clear" title="Clear the selection (Esc)">Clear selection</button>`;
    }

    destroy() {
        this._el?.remove();
        this._el = null;
    }
}
