// MultiSelect — a button naming a choice ("Cameras: all") that opens a list
// of checkboxes, every one on at first (ADR-0050). Unchecking narrows; "All"
// turns every one on again. The choice is null while everything is on, so a
// value that appears later (a new camera) is in it too.
//
// A floating layer, the only kind that casts a shadow (ADR-0030). Escape or a
// click outside closes it.

class MultiSelect {
    // label: the button's name ("Cameras"); options: [{ value, label, count }];
    // selected: an array of values, or null for all; onChange(selected).
    constructor({ label, options = [], selected = null, onChange }) {
        this._label = label;
        this._options = options;
        this._selected = selected;
        this._onChange = onChange;
        this.el = document.createElement('div');
        this.el.className = 'multi-select';
        this.el.innerHTML = `<button type="button" class="btn btn-sm select-btn multi-select-btn" aria-haspopup="true" aria-expanded="false"></button>
            <div class="multi-select-list" role="group" hidden></div>`;
        this._btn = this.el.querySelector('.multi-select-btn');
        this._list = this.el.querySelector('.multi-select-list');
        this._list.setAttribute('aria-label', label);
        this._btn.addEventListener('click', () => this._setOpen(this._list.hidden));
        this._outside = (e) => { if (!this.el.contains(e.target)) this._setOpen(false); };
        this._keys = (e) => {
            if (e.key !== 'Escape') return;
            e.stopPropagation();
            this._setOpen(false);
            this._btn.focus();
        };
        this._draw();
    }

    // New options (the lenses of other cameras): a chosen value that is gone
    // is dropped; returns whether that changed the choice.
    setOptions(options) {
        this._options = options;
        let changed = false;
        // A choice of none stays none; a choice whose values are all gone
        // falls back to all.
        if (this._selected?.length) {
            const kept = this._selected.filter(v => options.some(o => o.value === v));
            changed = kept.length !== this._selected.length;
            this._selected = kept.length ? kept : null;
        }
        this._draw();
        return changed;
    }

    get selected() { return this._selected; }

    set selected(values) {
        this._selected = values;
        this._draw();
    }

    _isOn(value) {
        return this._selected === null || this._selected.includes(value);
    }

    _summary() {
        if (this._selected === null) return `${this._label}: all`;
        if (this._selected.length === 0) return `${this._label}: none`;
        if (this._selected.length === 1) {
            const one = this._options.find(o => o.value === this._selected[0]);
            return `${this._label}: ${one ? one.label : this._selected[0]}`;
        }
        return `${this._label}: ${this._selected.length} of ${this._options.length}`;
    }

    _draw() {
        this._btn.textContent = this._summary();
        this._btn.classList.toggle('multi-select-btn--narrowed', this._selected !== null);
        const all = this._selected === null;
        this._list.innerHTML = `<label class="multi-select-row multi-select-all"><input type="checkbox"${all ? ' checked' : ''}><span>All</span></label>`
            + this._options.map((o, i) => `<label class="multi-select-row"><input type="checkbox" data-i="${i}"${this._isOn(o.value) ? ' checked' : ''}><span class="multi-select-name">${escapeHtml(o.label)}</span><span class="multi-select-count">${o.count != null ? formatCount(o.count) : ''}</span></label>`).join('');
        this._list.querySelector('.multi-select-all input').addEventListener('change', (e) => {
            this._change(e.target.checked ? null : []);
        });
        this._list.querySelectorAll('input[data-i]').forEach(input => input.addEventListener('change', () => {
            const value = this._options[Number(input.dataset.i)].value;
            const on = new Set(this._selected ?? this._options.map(o => o.value));
            if (input.checked) on.add(value); else on.delete(value);
            this._change(on.size === this._options.length ? null : [...on]);
        }));
    }

    _change(selected) {
        this._selected = selected;
        const focused = document.activeElement?.closest?.('.multi-select-row');
        const index = focused ? [...this._list.children].indexOf(focused) : -1;
        this._draw();
        if (index >= 0) this._list.children[index]?.querySelector('input')?.focus();
        this._onChange(selected);
    }

    _setOpen(open) {
        this._list.hidden = !open;
        this._btn.setAttribute('aria-expanded', String(open));
        if (open) {
            document.addEventListener('pointerdown', this._outside, true);
            this.el.addEventListener('keydown', this._keys);
            this._list.querySelector('input')?.focus();
        } else {
            document.removeEventListener('pointerdown', this._outside, true);
            this.el.removeEventListener('keydown', this._keys);
        }
    }
}
