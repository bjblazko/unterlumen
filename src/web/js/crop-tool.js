// Interactive crop overlay for the fullscreen viewer.
// Coordinates are expressed as fractions [0,1] of the rendered image area.

class CropTool {
    constructor(imgEl) {
        this._img = imgEl;
        this._ratio = null; // null = free, number = w/h
        this._rect = null;  // { x, y, w, h } in image-fraction space
        this._overlay = null;
        this._dragState = null; // { mode, startX, startY, origRect }
        this._onMouseMove = this._handleMouseMove.bind(this);
        this._onMouseUp   = this._handleMouseUp.bind(this);
        this._build();
    }

    setAspectRatio(ratio) {
        this._ratio = ratio;
        if (this._rect && ratio !== null) {
            this._rect = this._constrainRect(this._rect);
        }
        this._draw();
    }

    // Returns { x, y, width, height } as fractions [0,1], or null if no rect drawn.
    getRect() {
        if (!this._rect) return null;
        const r = this._rect;
        return { x: r.x, y: r.y, width: r.w, height: r.h };
    }

    destroy() {
        document.removeEventListener('mousemove', this._onMouseMove);
        document.removeEventListener('mouseup', this._onMouseUp);
        if (this._overlay && this._overlay.parentNode) {
            this._overlay.parentNode.removeChild(this._overlay);
        }
        this._overlay = null;
    }

    // --- Private ---

    _build() {
        const overlay = document.createElement('div');
        overlay.className = 'crop-overlay';
        this._overlay = overlay;

        overlay.addEventListener('mousedown', (e) => this._handleMouseDown(e));
        document.addEventListener('mousemove', this._onMouseMove);
        document.addEventListener('mouseup', this._onMouseUp);

        // Insert overlay as sibling after the image element
        this._img.parentNode.appendChild(overlay);
        this._positionOverlay();
    }

    _positionOverlay() {
        const r = this._imgRect();
        const o = this._overlay;
        o.style.left   = r.left + 'px';
        o.style.top    = r.top  + 'px';
        o.style.width  = r.width  + 'px';
        o.style.height = r.height + 'px';
    }

    // Bounding rect of the visible image content in viewport coordinates.
    // The img element is sized by CSS (max-width/max-height: 100%) to exactly
    // the visual image dimensions, so getBoundingClientRect() == content area.
    // Using naturalWidth/naturalHeight is unreliable for EXIF-rotated images
    // across browsers, so we rely on the element bounds directly.
    _imgRect() {
        const r = this._img.getBoundingClientRect();
        return { left: r.left, top: r.top, width: r.width, height: r.height };
    }

    // Convert viewport coords to fraction coords relative to the image.
    _toFraction(vx, vy) {
        const r = this._imgRect();
        return {
            x: (vx - r.left) / r.width,
            y: (vy - r.top)  / r.height,
        };
    }

    _handleMouseDown(e) {
        if (e.button !== 0) return;
        e.preventDefault();

        const handle = e.target.closest('.crop-handle');
        const box    = e.target.closest('.crop-box');

        if (handle) {
            this._dragState = {
                mode:      handle.dataset.handle,
                startX:    e.clientX,
                startY:    e.clientY,
                origRect:  { ...this._rect },
            };
            return;
        }

        if (box) {
            this._dragState = {
                mode:     'move',
                startX:   e.clientX,
                startY:   e.clientY,
                origRect: { ...this._rect },
            };
            return;
        }

        // Start new rect
        const f = this._toFraction(e.clientX, e.clientY);
        this._rect = { x: f.x, y: f.y, w: 0, h: 0 };
        this._dragState = {
            mode:   'new',
            startX: e.clientX,
            startY: e.clientY,
            origFx: f.x,
            origFy: f.y,
        };
        this._draw();
    }

    _handleMouseMove(e) {
        if (!this._dragState) return;
        e.preventDefault();

        const r    = this._imgRect();
        const dx   = (e.clientX - this._dragState.startX) / r.width;
        const dy   = (e.clientY - this._dragState.startY) / r.height;
        const mode = this._dragState.mode;
        const orig = this._dragState.origRect;

        if (mode === 'new') {
            const fx   = this._dragState.origFx;
            const fy   = this._dragState.origFy;
            const curF = this._toFraction(e.clientX, e.clientY);
            let nx = Math.min(fx, curF.x);
            let ny = Math.min(fy, curF.y);
            let nw = Math.abs(curF.x - fx);
            let nh = Math.abs(curF.y - fy);
            this._rect = this._constrainRect({ x: nx, y: ny, w: nw, h: nh });
            this._draw();
            return;
        }

        if (mode === 'move') {
            let nx = orig.x + dx;
            let ny = orig.y + dy;
            nx = Math.max(0, Math.min(1 - orig.w, nx));
            ny = Math.max(0, Math.min(1 - orig.h, ny));
            this._rect = { x: nx, y: ny, w: orig.w, h: orig.h };
            this._draw();
            return;
        }

        // Resize handle
        let { x, y, w, h } = orig;

        if (mode.includes('e'))  { w = Math.max(0.02, orig.w + dx); }
        if (mode.includes('s'))  { h = Math.max(0.02, orig.h + dy); }
        if (mode.includes('w'))  { const nw = Math.max(0.02, orig.w - dx); x = orig.x + orig.w - nw; w = nw; }
        if (mode.includes('n'))  { const nh = Math.max(0.02, orig.h - dy); y = orig.y + orig.h - nh; h = nh; }

        this._rect = this._constrainRect({ x, y, w, h });
        this._draw();
    }

    _handleMouseUp() {
        this._dragState = null;
        if (this._rect && (this._rect.w < 0.01 || this._rect.h < 0.01)) {
            this._rect = null;
            this._draw();
        }
    }

    // Apply aspect ratio constraint and clamp to [0,1].
    _constrainRect(r) {
        let { x, y, w, h } = r;

        if (this._ratio !== null && w > 0 && h > 0) {
            // this._ratio is a VISUAL pixel ratio (w_px / h_px).
            // Fractions map to pixels as: w_px = w * imgW, h_px = h * imgH.
            // So the visual ratio in fraction space equals (w * imgW) / (h * imgH).
            // To enforce: w * imgW / (h * imgH) == ratio  →  w / h == ratio * imgH / imgW
            const ir = this._imgRect();
            const fracRatio = this._ratio * ir.height / ir.width;
            const curFracRatio = w / h;
            if (curFracRatio > fracRatio) {
                w = h * fracRatio;
            } else {
                h = w / fracRatio;
            }
        }

        // Clamp so rect stays inside [0,1]×[0,1]
        if (x < 0) x = 0;
        if (y < 0) y = 0;
        if (x + w > 1) w = 1 - x;
        if (y + h > 1) h = 1 - y;

        return { x, y, w, h };
    }

    _draw() {
        this._positionOverlay();
        const o = this._overlay;
        o.innerHTML = '';

        if (!this._rect || this._rect.w < 0.005 || this._rect.h < 0.005) {
            // Just the cursor-crosshair overlay, no box yet
            return;
        }

        const { x, y, w, h } = this._rect;
        const pw = o.offsetWidth;
        const ph = o.offsetHeight;

        const px = x * pw;
        const py = y * ph;
        const pw2 = w * pw;
        const ph2 = h * ph;

        // Shade outside regions using four divs (top, bottom, left, right)
        // that together cover the area outside the crop box.
        const mkShade = (l, t, rw, rh) => {
            const d = document.createElement('div');
            d.className = 'crop-shade';
            d.style.cssText = `left:${l}px;top:${t}px;width:${rw}px;height:${rh}px`;
            o.appendChild(d);
        };
        mkShade(0,         0,      pw,        py);           // top
        mkShade(0,         py+ph2, pw,        ph-py-ph2);   // bottom
        mkShade(0,         py,     px,        ph2);          // left
        mkShade(px+pw2,    py,     pw-px-pw2, ph2);          // right

        // Crop box (border)
        const box = document.createElement('div');
        box.className = 'crop-box';
        box.style.cssText = `left:${px}px;top:${py}px;width:${pw2}px;height:${ph2}px`;
        o.appendChild(box);

        // Rule-of-thirds grid
        const grid = document.createElement('div');
        grid.className = 'crop-grid';
        grid.innerHTML = `
            <div class="crop-grid-line crop-grid-v" style="left:33.33%"></div>
            <div class="crop-grid-line crop-grid-v" style="left:66.66%"></div>
            <div class="crop-grid-line crop-grid-h" style="top:33.33%"></div>
            <div class="crop-grid-line crop-grid-h" style="top:66.66%"></div>
        `;
        box.appendChild(grid);

        // Resize handles: nw, n, ne, e, se, s, sw, w
        const handles = [
            ['nw', 0,   0   ],
            ['n',  50,  0   ],
            ['ne', 100, 0   ],
            ['e',  100, 50  ],
            ['se', 100, 100 ],
            ['s',  50,  100 ],
            ['sw', 0,   100 ],
            ['w',  0,   50  ],
        ];
        for (const [dir, lp, tp] of handles) {
            const h = document.createElement('div');
            h.className = 'crop-handle';
            h.dataset.handle = dir;
            h.style.cssText = `left:${lp}%;top:${tp}%;transform:translate(-50%,-50%)`;
            box.appendChild(h);
        }
    }
}
