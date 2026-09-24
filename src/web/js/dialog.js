// dialog.js — one dialog for the whole app (ADR-0033).
//
// A dialog is for one decision about what is already on the screen: it fits on
// one screen, it needs the context behind it, and it ends with an action or a
// cancel. Anything you read, compare or work in for a while is a place, not a
// dialog.
//
// This owns the frame and the behaviour, never the content:
//   • scrim, panel, header, scrolling body, footer
//   • Escape and a click on the scrim cancel; the focus goes in on open,
//     stays inside while open, and returns to where it came from
//   • at most one accent action, always on the right
//   • on a phone it sits at the bottom as a sheet
//
// Usage:
//   const dlg = new Dialog({
//       title: 'Delete library',
//       body: '<p>…</p>',                       // string or Element
//       actions: [
//           { label: 'Cancel', value: null },
//           { label: 'Delete library', kind: 'danger', onClick: (d) => … },
//       ],
//   });
//   dlg.open();          // → the .dialog element, for your own listeners
//   dlg.close(result);   // resolves what open() handed to onClose

const DIALOG_SIZES = new Set(['sm', 'md', 'lg']);

class Dialog {
    constructor({
        title,
        subtitle = '',
        size = 'md',
        body = '',
        actions = [],
        note = '',
        dismissible = true,
        className = '',
        onClose = null,
    } = {}) {
        this.title = title;
        this.subtitle = subtitle;
        this.size = DIALOG_SIZES.has(size) ? size : 'md';
        this._bodyContent = body;
        this._actions = actions;
        this._note = note;
        this.dismissible = dismissible;
        this.className = className;
        this.onClose = onClose;
        this.scrim = null;
        this.el = null;
        this._returnFocus = null;
        this._onKeyDown = this._onKeyDown.bind(this);
    }

    static get openCount() { return document.querySelectorAll('.dialog-scrim').length; }

    open() {
        this._returnFocus = document.activeElement;
        this.scrim = document.createElement('div');
        this.scrim.className = 'dialog-scrim';
        // Dialogs can stack (a folder picker on top of an export dialog), so
        // each one sits above the one it was opened from.
        this.scrim.style.zIndex = String(1000 + Dialog.openCount * 10);

        const titleID = `dialog-title-${Math.random().toString(36).slice(2, 8)}`;
        this.scrim.innerHTML = `
            <div class="dialog dialog--${this.size}${this.className ? ' ' + this.className : ''}"
                 role="dialog" aria-modal="true" aria-labelledby="${titleID}">
                <div class="dialog-head">
                    <h2 class="dialog-title" id="${titleID}">${escapeHtml(this.title)}</h2>
                    ${this.subtitle ? `<span class="dialog-subtitle">${escapeHtml(this.subtitle)}</span>` : ''}
                </div>
                <div class="dialog-body"></div>
                <div class="dialog-foot">
                    <span class="dialog-note">${escapeHtml(this._note)}</span>
                </div>
            </div>`;

        this.el = this.scrim.querySelector('.dialog');
        this.body = this.scrim.querySelector('.dialog-body');
        this.foot = this.scrim.querySelector('.dialog-foot');
        this.setBody(this._bodyContent);
        this.setActions(this._actions);

        this.scrim.addEventListener('mousedown', (e) => {
            if (e.target === this.scrim && this.dismissible) this.close(null);
        });
        document.addEventListener('keydown', this._onKeyDown, true);
        document.body.appendChild(this.scrim);

        this._focusFirst();
        return this.el;
    }

    close(result = null) {
        if (!this.scrim) return;
        this.scrim.remove();
        this.scrim = null;
        this.el = null;
        document.removeEventListener('keydown', this._onKeyDown, true);
        if (this._returnFocus?.isConnected) this._returnFocus.focus();
        if (this.onClose) this.onClose(result);
    }

    /* --- Content --- */

    setBody(content) {
        this.body.innerHTML = '';
        if (content instanceof Element) this.body.appendChild(content);
        else this.body.innerHTML = content ?? '';
        return this.body;
    }

    // Actions read left to right in growing weight, so the one that does the
    // thing is last — and there is at most one of it.
    setActions(actions = []) {
        this._actions = actions;
        for (const el of [...this.foot.querySelectorAll('.dialog-action')]) el.remove();
        const kinds = { primary: 'btn btn-accent', danger: 'btn btn-danger', quiet: 'btn btn-sm', secondary: 'btn' };
        for (const action of actions) {
            const btn = document.createElement('button');
            btn.className = `${kinds[action.kind] ?? kinds.secondary} dialog-action`;
            btn.textContent = action.label;
            if (action.id) btn.id = action.id;
            if (action.disabled) btn.disabled = true;
            btn.addEventListener('click', () => {
                if (action.onClick) action.onClick(this);
                else this.close(action.value ?? null);
            });
            this.foot.appendChild(btn);
        }
    }

    // A sentence in the footer: what is selected, where it goes, what failed.
    setNote(text, kind = '') {
        const note = this.foot.querySelector('.dialog-note');
        note.textContent = text ?? '';
        note.className = `dialog-note${kind === 'error' ? ' dialog-note--problem' : ''}`;
    }

    setTitle(title, subtitle = null) {
        this.title = title;
        this.el.querySelector('.dialog-title').textContent = title;
        if (subtitle !== null) {
            const el = this.el.querySelector('.dialog-subtitle');
            if (el) el.textContent = subtitle;
        }
    }

    /* --- Focus and keys --- */

    _focusFirst() {
        const target = this.el.querySelector('[autofocus]') || this._tabbables()[0];
        target?.focus();
    }

    _tabbables() {
        return [...this.el.querySelectorAll(
            'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])',
        )].filter(el => el.offsetParent !== null || el === document.activeElement);
    }

    _onKeyDown(e) {
        // Only the topmost dialog listens; the ones below it wait.
        if (!this.scrim || this.scrim !== document.querySelector('.dialog-scrim:last-of-type')) return;

        if (e.key === 'Escape' && this.dismissible) {
            e.preventDefault();
            e.stopPropagation();
            this.close(null);
            return;
        }
        if (e.key === 'Tab') {
            const items = this._tabbables();
            if (!items.length) return;
            const first = items[0];
            const last = items[items.length - 1];
            if (e.shiftKey && document.activeElement === first) {
                e.preventDefault();
                last.focus();
            } else if (!e.shiftKey && document.activeElement === last) {
                e.preventDefault();
                first.focus();
            }
        }
    }
}
