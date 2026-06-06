// ChipInput — two-phase autocomplete chip filter
// Phase 1: namespace suggestions (camera:, lens:, film:, …)
// Phase 2: value suggestions for the selected namespace
//
// Usage:
//   new ChipInput(container, { onFetchNamespaces, onFetchValues, onChange })
//   onFetchNamespaces(): Promise<[{ns, label, hint}]>
//   onFetchValues(ns): Promise<string[]>
//   onChange(chips): void   chips = [{ns, label, value}]

// Palette for chip colours — cycles per chip added, not per namespace.
const CHIP_HUES = [20, 170, 260, 100, 300, 45, 220, 140];

class ChipInput {
    constructor(container, { onFetchNamespaces, onFetchValues, onChange } = {}) {
        this._onFetchNamespaces = onFetchNamespaces || (() => Promise.resolve([]));
        this._onFetchValues     = onFetchValues     || (() => Promise.resolve([]));
        this._onChange          = onChange           || (() => {});
        this._chips      = [];      // [{ns, label, value, hue}]
        this._phase      = 'ns';   // 'ns' | 'value'
        this._selectedNS = null;   // {ns, label} during value phase
        this._suggestions = [];
        this._activeIdx   = -1;
        this._nsCache     = null;
        this._valueCache  = {};
        this._nextHue     = 0;     // cycles through CHIP_HUES

        this._build(container);
    }

    getChips() { return this._chips; }

    reset() {
        this._chips = [];
        this._renderChips();
        this._onChange([]);
    }

    // Clears cached namespace and value lists so the next open re-fetches from the server.
    clearCache() {
        this._nsCache = null;
        this._valueCache = {};
    }

    _build(container) {
        container.innerHTML = '';
        container.className = 'chip-input';

        const wrap = document.createElement('div');
        wrap.className = 'chip-input-wrap';
        container.appendChild(wrap);
        this._wrapEl = wrap;

        // Fixed namespace-label shown during value phase (replaces placeholder approach).
        this._nsLabel = document.createElement('span');
        this._nsLabel.className = 'chip-ns-label';
        this._nsLabel.style.display = 'none';
        wrap.appendChild(this._nsLabel);

        this._input = document.createElement('input');
        this._input.type = 'text';
        this._input.className = 'chip-input-field';
        this._input.placeholder = 'Add filter…';
        this._input.setAttribute('autocomplete', 'off');
        wrap.appendChild(this._input);

        this._dropdown = document.createElement('div');
        this._dropdown.className = 'chip-autocomplete';
        this._dropdown.style.display = 'none';
        container.appendChild(this._dropdown);

        this._input.addEventListener('focus', () => this._open());
        this._input.addEventListener('input',  () => this._onInput());
        this._input.addEventListener('keydown', e  => this._onKey(e));
        this._input.addEventListener('blur', () => setTimeout(() => this._close(), 150));

        // Clicking the wrap focuses the input.
        wrap.addEventListener('click', e => {
            if (e.target !== this._input) this._input.focus();
        });
    }

    _renderChips() {
        for (const el of [...this._wrapEl.querySelectorAll('.chip')]) el.remove();
        // Insert chips before the ns-label.
        for (const chip of this._chips) {
            const el = document.createElement('span');
            el.className = 'chip';
            el.style.setProperty('--chip-hue', String(chip.hue));
            el.innerHTML =
                `<span class="chip-label">${escapeHtml(chip.label)}: ${escapeHtml(chip.displayValue || chip.value)}</span>` +
                `<button class="chip-remove" title="Remove filter">×</button>`;
            el.querySelector('.chip-remove').addEventListener('click', () => {
                this._chips = this._chips.filter(c => c !== chip);
                this._renderChips();
                this._onChange(this._chips);
            });
            this._wrapEl.insertBefore(el, this._nsLabel);
        }
    }

    async _open() {
        // If already in value phase (re-focus after e.g. scrolling), stay there.
        if (this._phase === 'value') {
            this._renderSuggestions(this._filterVals(this._input.value));
            return;
        }
        await this._loadNamespaces();
        this._renderSuggestions(this._filterNS(this._input.value));
    }

    async _loadNamespaces() {
        if (!this._nsCache) {
            this._nsCache = await this._onFetchNamespaces().catch(() => []);
        }
    }

    _enterValuePhase(ns) {
        this._phase = 'value';
        this._selectedNS = ns;
        this._input.value = '';
        this._input.placeholder = '';
        this._nsLabel.textContent = ns.label + ': ';
        this._nsLabel.style.display = '';
    }

    _exitValuePhase() {
        this._phase = 'ns';
        this._selectedNS = null;
        this._nsLabel.style.display = 'none';
        this._input.placeholder = 'Add filter…';
    }

    _close() {
        this._dropdown.style.display = 'none';
        this._activeIdx = -1;
        if (this._phase === 'value') {
            this._exitValuePhase();
            this._input.value = '';
        }
    }

    async _onInput() {
        if (this._phase === 'ns') {
            await this._loadNamespaces();
            this._renderSuggestions(this._filterNS(this._input.value));
        } else {
            this._renderSuggestions(this._filterVals(this._input.value));
        }
    }

    _filterNS(text) {
        const q = text.trim().toLowerCase();
        if (!q) return this._nsCache || [];
        return (this._nsCache || []).filter(n =>
            n.ns.toLowerCase().startsWith(q) || n.label.toLowerCase().startsWith(q)
        );
    }

    _filterVals(text) {
        const q = text.trim().toLowerCase();
        const vals = this._valueCache[this._selectedNS?.ns] || [];
        if (!q) return vals;
        return vals.filter(v => {
            const s = typeof v === 'string' ? v : v.label;
            return s.toLowerCase().includes(q);
        });
    }

    // Position the dropdown with fixed coordinates so it escapes overflow clipping.
    _positionDropdown() {
        const rect  = this._wrapEl.getBoundingClientRect();
        const dd    = this._dropdown;
        const width = Math.max(rect.width, 280);
        dd.style.left  = rect.left + 'px';
        dd.style.width = width + 'px';

        const spaceBelow = window.innerHeight - rect.bottom - 4;
        const spaceAbove = rect.top - 4;

        if (spaceBelow < 80 && spaceAbove > spaceBelow) {
            dd.style.top       = 'auto';
            dd.style.bottom    = (window.innerHeight - rect.top + 3) + 'px';
            dd.style.maxHeight = Math.min(260, spaceAbove - 4) + 'px';
        } else {
            dd.style.top       = (rect.bottom + 3) + 'px';
            dd.style.bottom    = 'auto';
            dd.style.maxHeight = Math.min(260, Math.max(80, spaceBelow - 4)) + 'px';
        }
    }

    _renderSuggestions(items) {
        this._suggestions = items;
        this._activeIdx   = -1;
        this._dropdown.innerHTML = '';

        if (items.length === 0) {
            if (this._phase === 'value') {
                const hint = document.createElement('div');
                hint.className = 'chip-ac-item chip-ac-no-values';
                hint.textContent = 'Type a value and press Enter';
                this._dropdown.appendChild(hint);
                this._positionDropdown();
                this._dropdown.style.display = '';
            } else {
                this._dropdown.style.display = 'none';
            }
            return;
        }

        for (const item of items) {
            const el = document.createElement('div');
            el.className = 'chip-ac-item';
            if (this._phase === 'value') {
                // Value items are either plain strings or {label, value} pairs (decoded EXIF).
                const display = typeof item === 'string' ? item : item.label;
                const raw     = typeof item === 'string' ? item : item.value;
                el.textContent = display;
                el.addEventListener('mousedown', e => { e.preventDefault(); this._selectValue(raw, display); });
            } else {
                el.innerHTML = `<span class="chip-ac-ns">${escapeHtml(item.ns)}:</span>` +
                    (item.hint ? `<span class="chip-ac-hint">${escapeHtml(item.hint)}</span>` : '');
                el.addEventListener('mousedown', e => { e.preventDefault(); this._selectNS(item); });
            }
            this._dropdown.appendChild(el);
        }
        this._positionDropdown();
        this._dropdown.style.display = '';
    }

    async _selectNS(ns) {
        this._enterValuePhase(ns);
        if (!this._valueCache[ns.ns]) {
            const vals = await this._onFetchValues(ns.ns).catch(() => []);
            this._valueCache[ns.ns] = vals;
        }
        this._renderSuggestions(this._filterVals(''));
    }

    _selectValue(value, displayValue) {
        const hue = CHIP_HUES[this._nextHue % CHIP_HUES.length];
        this._nextHue++;
        this._chips.push({ ns: this._selectedNS.ns, label: this._selectedNS.label, nsInfo: { ...this._selectedNS }, value, displayValue: displayValue || value, hue });
        this._renderChips();
        this._onChange(this._chips);
        this._exitValuePhase();
        this._input.value = '';
        this._dropdown.style.display = 'none';
    }

    _onKey(e) {
        const items = this._suggestions;
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            this._activeIdx = Math.min(this._activeIdx + 1, items.length - 1);
            this._highlightActive();
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            this._activeIdx = Math.max(this._activeIdx - 1, -1);
            this._highlightActive();
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (this._activeIdx >= 0 && this._activeIdx < items.length) {
                const item = items[this._activeIdx];
                if (this._phase === 'ns') {
                    this._selectNS(item);
                } else {
                    const display = typeof item === 'string' ? item : item.label;
                    const raw     = typeof item === 'string' ? item : item.value;
                    this._selectValue(raw, display);
                }
            } else if (this._phase === 'value' && this._input.value.trim()) {
                // Freehand: create chip from typed text with no matching suggestion.
                this._selectValue(this._input.value.trim());
            }
        } else if (e.key === 'Escape') {
            this._close();
        } else if (e.key === 'Backspace' && this._input.value === '') {
            if (this._phase === 'value') {
                this._exitValuePhase();
                this._loadNamespaces().then(() => this._renderSuggestions(this._filterNS('')));
            } else if (this._chips.length > 0) {
                this._chips.pop();
                this._renderChips();
                this._onChange(this._chips);
            }
        }
    }

    _highlightActive() {
        const els = this._dropdown.querySelectorAll('.chip-ac-item');
        els.forEach((el, i) => el.classList.toggle('active', i === this._activeIdx));
        if (this._activeIdx >= 0 && this._activeIdx < els.length) {
            els[this._activeIdx].scrollIntoView({ block: 'nearest' });
        }
    }
}
