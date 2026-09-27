// RangeSlider — two handles on one track: a from–to choice over a span.
//
// Positions are fractions 0…1; the caller maps them to its values. Each
// handle is a role="slider" that takes the pointer (mouse, pen, touch) and
// the arrow, Page and Home/End keys. While a handle moves it carries
// .is-adjusting — the current value of something adjusted right now.

class RangeSlider {
    // label: what the range chooses, for screen readers ("Date taken").
    // step: how far one arrow key moves a handle, as a fraction.
    // valueText(pos): the handle's value in words, for screen readers.
    // onInput(minPos, maxPos): called on every move.
    constructor({ label, step = 0.01, valueText = (pos) => `${Math.round(pos * 100)} %`, onInput }) {
        this._step = step;
        this._valueText = valueText;
        this._onInput = onInput;
        this.min = 0;
        this.max = 1;

        this.el = document.createElement('div');
        this.el.className = 'range-slider';
        this.el.innerHTML =
            '<div class="range-track"><div class="range-fill"></div></div>' +
            '<div class="range-handle range-handle--min" role="slider" tabindex="0"></div>' +
            '<div class="range-handle range-handle--max" role="slider" tabindex="0"></div>';
        this._fill = this.el.querySelector('.range-fill');
        this._minHandle = this.el.querySelector('.range-handle--min');
        this._maxHandle = this.el.querySelector('.range-handle--max');
        this._minHandle.setAttribute('aria-label', `${label}, from`);
        this._maxHandle.setAttribute('aria-label', `${label}, until`);

        for (const [handle, isMin] of [[this._minHandle, true], [this._maxHandle, false]]) {
            this._attachPointer(handle, isMin);
            this._attachKeys(handle, isMin);
        }
        this._draw();
    }

    // Moves both handles without calling onInput.
    setPositions(min, max) {
        this.min = min;
        this.max = max;
        this._draw();
    }

    _move(isMin, pos) {
        pos = Math.max(0, Math.min(1, pos));
        if (isMin) this.min = Math.min(pos, this.max);
        else this.max = Math.max(pos, this.min);
        this._draw();
        this._onInput?.(this.min, this.max);
    }

    _draw() {
        this._minHandle.style.left = `${this.min * 100}%`;
        this._maxHandle.style.left = `${this.max * 100}%`;
        this._fill.style.left = `${this.min * 100}%`;
        this._fill.style.width = `${(this.max - this.min) * 100}%`;
        for (const [handle, pos] of [[this._minHandle, this.min], [this._maxHandle, this.max]]) {
            handle.setAttribute('aria-valuemin', '0');
            handle.setAttribute('aria-valuemax', '100');
            handle.setAttribute('aria-valuenow', String(Math.round(pos * 100)));
            handle.setAttribute('aria-valuetext', this._valueText(pos));
        }
    }

    _attachPointer(handle, isMin) {
        handle.addEventListener('pointerdown', (e) => {
            if (e.button !== 0) return;
            e.preventDefault();
            // Focus for the arrow keys afterwards, without the ring: the
            // pointer already shows which handle this is.
            handle.focus({ focusVisible: false });
            handle.setPointerCapture(e.pointerId);
            handle.classList.add('is-adjusting');
            const onMove = (ev) => {
                const rect = this.el.getBoundingClientRect();
                this._move(isMin, (ev.clientX - rect.left) / rect.width);
            };
            const onUp = () => {
                handle.classList.remove('is-adjusting');
                handle.removeEventListener('pointermove', onMove);
                handle.removeEventListener('pointerup', onUp);
                handle.removeEventListener('pointercancel', onUp);
            };
            handle.addEventListener('pointermove', onMove);
            handle.addEventListener('pointerup', onUp);
            handle.addEventListener('pointercancel', onUp);
        });
    }

    // The keys a slider owns stop here, so the place's own shortcuts
    // (arrows move the focus in a grid) do not act on them as well.
    _attachKeys(handle, isMin) {
        handle.addEventListener('keydown', (e) => {
            const pos = isMin ? this.min : this.max;
            const target = this._keyTarget(e.key, pos);
            if (target === null) return;
            e.preventDefault();
            e.stopPropagation();
            this._move(isMin, target);
        });
    }

    _keyTarget(key, pos) {
        switch (key) {
            case 'ArrowLeft': case 'ArrowDown': return pos - this._step;
            case 'ArrowRight': case 'ArrowUp': return pos + this._step;
            case 'PageDown': return pos - this._step * 10;
            case 'PageUp': return pos + this._step * 10;
            case 'Home': return 0;
            case 'End': return 1;
            default: return null;
        }
    }
}
