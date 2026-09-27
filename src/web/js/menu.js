// Menu — the one "⋯" menu: view options and actions that need not be on
// screen all the time. It is a floating layer (the one place a shadow is
// allowed), opened from a framed button with an accessible name. Choosing an
// action, Escape, Tab or a click outside closes it; a switch flips in place
// and leaves the menu open, so the change shows behind it. The arrow keys move
// through its items. While open it owns the keyboard (.keyboard-owner), so the global
// shortcuts wait.
//
//   const menu = new Menu({ items: () => [
//       { label: 'Names', switch: { on: true, labelOn: 'Shown', labelOff: 'Hidden' },
//         onChange: (on) => … },
//       'separator',
//       { label: 'Clear cache', disabled: true, title: 'Select photos first', onSelect: () => … },
//   ] });
//   container.appendChild(menu.button);
//
// items is a function, read each time the menu opens, so labels and states
// are those of that moment. A switch is drawn by Toggle, so it has the same
// three labels as every other switch in the app.

class Menu {
    constructor({ items, label = 'More' }) {
        this._items = items;
        this._el = null;
        this.button = document.createElement('button');
        this.button.type = 'button';
        this.button.className = 'btn btn-sm menu-btn';
        this.button.setAttribute('aria-haspopup', 'menu');
        this.button.setAttribute('aria-expanded', 'false');
        this.button.setAttribute('aria-label', label);
        this.button.title = label;
        // Three dots drawn at the size of the icons beside it; the text
        // character was set larger than every other control in the row.
        this.button.innerHTML = '<svg width="13" height="13" viewBox="0 0 13 13" fill="currentColor" aria-hidden="true">'
            + '<circle cx="2.5" cy="6.5" r="1.25"/><circle cx="6.5" cy="6.5" r="1.25"/><circle cx="10.5" cy="6.5" r="1.25"/></svg>';
        this.button.addEventListener('click', () => (this._el ? this.close() : this.open()));
        this._onOutside = (e) => {
            if (!this._el || this._el.contains(e.target) || this.button.contains(e.target)) return;
            this.close();
        };
    }

    open() {
        Menu._current?.close();
        Menu._current = this;
        const el = document.createElement('div');
        el.className = 'menu keyboard-owner';
        el.setAttribute('role', 'menu');
        for (const item of this._items()) el.appendChild(this._itemEl(item));
        document.body.appendChild(el);
        this._el = el;
        this._place();
        this.button.setAttribute('aria-expanded', 'true');
        el.addEventListener('keydown', (e) => this._onKey(e));
        document.addEventListener('pointerdown', this._onOutside, true);
        this._focusable()[0]?.focus();
    }

    close({ refocus = true } = {}) {
        if (!this._el) return;
        this._el.remove();
        this._el = null;
        if (Menu._current === this) Menu._current = null;
        this.button.setAttribute('aria-expanded', 'false');
        document.removeEventListener('pointerdown', this._onOutside, true);
        if (refocus && this.button.isConnected) this.button.focus();
    }

    _itemEl(item) {
        if (item === 'separator') {
            const sep = document.createElement('div');
            sep.className = 'menu-sep';
            sep.setAttribute('role', 'separator');
            return sep;
        }
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'menu-item';
        if (item.id) btn.dataset.id = item.id;
        if (item.title) btn.title = item.title;
        btn.disabled = !!item.disabled;
        btn.innerHTML = '<span class="menu-label"></span>';
        btn.querySelector('.menu-label').textContent = item.label;
        if (item.switch) return this._switchEl(btn, item);
        btn.setAttribute('role', 'menuitem');
        btn.addEventListener('click', () => {
            // Closed first: the action often re-renders what holds the button.
            this.close({ refocus: false });
            item.onSelect?.();
        });
        return btn;
    }

    // The row is the control; the Toggle inside only shows its state.
    _switchEl(btn, item) {
        btn.classList.add('menu-item--switch');
        btn.setAttribute('role', 'menuitemcheckbox');
        btn.setAttribute('aria-checked', String(item.switch.on));
        const toggle = Toggle.create(btn, {
            initial: item.switch.on, labelOn: item.switch.labelOn, labelOff: item.switch.labelOff,
        });
        toggle.el.removeAttribute('role');
        toggle.el.setAttribute('aria-hidden', 'true');
        btn.addEventListener('click', () => {
            const on = !toggle.state();
            toggle.setState(on);
            btn.setAttribute('aria-checked', String(on));
            item.onChange(on);
        });
        return btn;
    }

    // Below the button, right-aligned with it; above it when there is no room.
    _place() {
        const r = this.button.getBoundingClientRect();
        const el = this._el;
        el.style.right = Math.max(8, window.innerWidth - r.right) + 'px';
        const below = r.bottom + 4;
        const height = el.offsetHeight;
        el.style.top = (below + height > window.innerHeight - 8 && r.top - 4 - height > 8)
            ? (r.top - 4 - height) + 'px'
            : below + 'px';
    }

    _focusable() {
        return [...this._el.querySelectorAll('.menu-item:not(:disabled)')];
    }

    _onKey(e) {
        const items = this._focusable();
        const at = items.indexOf(document.activeElement);
        if (e.key === 'Escape') {
            e.preventDefault(); e.stopPropagation();
            this.close();
        } else if (e.key === 'Tab') {
            this.close({ refocus: false });
        } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault();
            if (items.length === 0) return;
            const next = e.key === 'ArrowDown' ? (at + 1) % items.length : (at - 1 + items.length) % items.length;
            items[next].focus();
        } else if (e.key === 'Home' || e.key === 'End') {
            e.preventDefault();
            items[e.key === 'Home' ? 0 : items.length - 1]?.focus();
        }
    }
}

Menu._current = null;
