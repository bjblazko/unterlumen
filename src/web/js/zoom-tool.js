// Image zoom and pan for the viewer

class ZoomTool {
    constructor(imgEl, containerEl) {
        this._img = imgEl;
        this._container = containerEl;
        this._levels = ['fit', 5, 10, 15, 25, 50, 75, 100, 150, 200, 300, 400];
        this._idx = 0;
        this._naturalW = 0;
        this._naturalH = 0;
        this._onchange = null;
        this._drag = null;

        if (imgEl.complete && imgEl.naturalWidth) {
            this._naturalW = imgEl.naturalWidth;
            this._naturalH = imgEl.naturalHeight;
        } else {
            imgEl.addEventListener('load', () => {
                this._naturalW = imgEl.naturalWidth;
                this._naturalH = imgEl.naturalHeight;
                if (this._idx !== 0) this._apply();
            }, { once: true });
        }

        this._onMouseDown = this._onMouseDown.bind(this);
        this._onMouseMove = this._onMouseMove.bind(this);
        this._onMouseUp   = this._onMouseUp.bind(this);
        this._onWheel     = this._onWheel.bind(this);

        containerEl.addEventListener('mousedown', this._onMouseDown);
        containerEl.addEventListener('wheel', this._onWheel, { passive: false });
    }

    zoomIn() {
        if (this._idx === 0) {
            const fit = this._fitPct();
            const next = this._levels.findIndex((l, i) => i > 0 && l > fit);
            if (next >= 0) { this._idx = next; this._apply(); }
        } else if (this._idx < this._levels.length - 1) {
            this._idx++;
            this._apply();
        }
    }

    zoomOut() {
        if (this._idx === 0) {
            const fit = this._fitPct();
            let prev = -1;
            for (let i = 1; i < this._levels.length; i++) {
                if (this._levels[i] < fit) prev = i; else break;
            }
            if (prev >= 0) { this._idx = prev; this._apply(); }
        } else if (this._idx > 0) {
            this._idx--;
            this._apply();
        }
    }

    reset() { this._idx = 0; this._apply(); }

    // Percentage of native resolution that the current container shows in fit mode
    _fitPct() {
        if (!this._naturalW || !this._naturalH) return 100;
        const scale = Math.min(
            this._container.clientWidth  / this._naturalW,
            this._container.clientHeight / this._naturalH
        );
        return scale * 100;
    }

    setLevel(value) {
        const idx = this._levels.indexOf(value);
        if (idx >= 0) { this._idx = idx; this._apply(); }
    }

    getCurrentLevel() { return this._levels[this._idx]; }

    isAtMin() {
        if (this._idx > 0) return false;
        const fit = this._fitPct();
        return !this._levels.some((l, i) => i > 0 && l < fit);
    }

    isAtMax() { return this._idx === this._levels.length - 1; }

    _apply() {
        const level = this._levels[this._idx];
        if (level === 'fit') {
            this._resetStyles();
        } else {
            const w = Math.round(this._naturalW * level / 100);
            const h = Math.round(this._naturalH * level / 100);

            // Use inline styles — they beat all CSS class rules including !important on other selectors
            this._container.style.overflow       = 'auto';
            this._container.style.alignItems     = 'flex-start';
            this._container.style.justifyContent = 'flex-start';
            this._container.style.cursor         = 'grab';

            this._img.style.width      = w + 'px';
            this._img.style.height     = h + 'px';
            this._img.style.maxWidth   = 'none';
            this._img.style.maxHeight  = 'none';
            this._img.style.objectFit  = 'fill';
            this._img.style.flexShrink = '0';

            // Reading clientWidth forces a synchronous layout flush, ensuring
            // scroll is set after the browser has processed the new dimensions.
            requestAnimationFrame(() => {
                const cw = this._container.clientWidth;
                const ch = this._container.clientHeight;
                const mh = Math.max(0, Math.floor((ch - h) / 2));
                const mw = Math.max(0, Math.floor((cw - w) / 2));
                this._img.style.margin         = `${mh}px ${mw}px`;
                this._container.scrollLeft = Math.max(0, Math.round((w - cw) / 2));
                this._container.scrollTop  = Math.max(0, Math.round((h - ch) / 2));
            });
        }
        if (this._onchange) this._onchange();
    }

    _resetStyles() {
        this._container.style.overflow       = '';
        this._container.style.alignItems     = '';
        this._container.style.justifyContent = '';
        this._container.style.cursor         = '';
        this._img.style.width      = '';
        this._img.style.height     = '';
        this._img.style.maxWidth   = '';
        this._img.style.maxHeight  = '';
        this._img.style.objectFit  = '';
        this._img.style.flexShrink = '';
        this._img.style.margin     = '';
    }

    _onMouseDown(e) {
        if (this._idx === 0 || e.button !== 0) return;
        this._drag = { x: e.clientX, y: e.clientY, sl: this._container.scrollLeft, st: this._container.scrollTop };
        this._container.style.cursor = 'grabbing';
        document.addEventListener('mousemove', this._onMouseMove);
        document.addEventListener('mouseup',   this._onMouseUp);
        e.preventDefault();
    }

    _onMouseMove(e) {
        if (!this._drag) return;
        this._container.scrollLeft = this._drag.sl - (e.clientX - this._drag.x);
        this._container.scrollTop  = this._drag.st - (e.clientY - this._drag.y);
    }

    _onMouseUp() {
        this._drag = null;
        this._container.style.cursor = this._idx === 0 ? '' : 'grab';
        document.removeEventListener('mousemove', this._onMouseMove);
        document.removeEventListener('mouseup',   this._onMouseUp);
    }

    _onWheel(e) {
        e.preventDefault();
        if (e.deltaY < 0) this.zoomIn(); else this.zoomOut();
    }

    destroy() {
        this._idx = 0;
        this._resetStyles();
        this._container.removeEventListener('mousedown', this._onMouseDown);
        this._container.removeEventListener('wheel',     this._onWheel);
        document.removeEventListener('mousemove', this._onMouseMove);
        document.removeEventListener('mouseup',   this._onMouseUp);
    }
}
