// point-stage.js — a 3D stage of photos as points of light (ADR-0046): the
// Colour space, the Exposure space and Daylight are each one of these with
// their own mapping of the data. The stage turns by dragging, the wheel or
// the arrow keys; a light is read by hovering and its photos shown by a
// click. What is drawn on the GPU is in point-scene.js.

const POINT_STAGE_NO_GPU = 'This view needs WebGPU, which this browser does not offer. Current versions of Chrome, Edge and Safari have it.';

// renderPointStage draws into el the stage that build(stageEl) describes:
// {
//   summary,    what is drawn, in a sentence (the canvas's name and help),
//   legend,     HTML of the legend at the foot of the stage,
//   sprites,    the photos, SPRITE_FLOATS each (point-scene.js),
//   steps,      the periods of the trail, the same,
//   trail,      SEGMENT_FLOATS per segment; lines, LINE_FLOATS per vertex,
//   labels,     [{ text, pos: [x, y, z], kind }],
//   tip(kind, i), the tooltip HTML of photo or step i,
//   pick(kind, i), what onPick gets for it,
//   distance,   how far the camera stands from the middle, if not the default,
//   trailWidth, the trail's width in CSS pixels, if not 5.
// }
// build gets the stage element, so a view can read tokens from it. A
// preview turns by itself, takes no input and has smaller lights.
function renderPointStage(el, build, onPick, { preview = false } = {}) {
    el.innerHTML = `<div class="point-stage${preview ? ' point-stage--preview' : ''}" data-theme="dark"></div>`;
    const stageEl = el.firstElementChild;
    gpuDevice().then(device => {
        if (!device) {
            stageEl.innerHTML = `<p class="point-stage-nogpu">${POINT_STAGE_NO_GPU}</p>`;
            return;
        }
        const spec = build(stageEl);
        stageEl.innerHTML = `
            <canvas class="point-stage-canvas"${preview ? '' : ` tabindex="0" role="img" aria-label="${escapeHtml(spec.summary)}"`}></canvas>
            ${preview ? '' : `<div class="point-stage-labels" aria-hidden="true"></div><div class="point-stage-ring" hidden></div><div class="point-stage-tip" hidden></div><div class="point-stage-legend">${spec.legend}</div>`}`;
        if (!preview) {
            el.insertAdjacentHTML('beforeend', `<p class="point-stage-help">${escapeHtml(spec.summary)} Drag to turn the view and scroll or pinch to zoom; once it is selected, the arrow keys turn it and + and − zoom. Point at a light to read it, click it to see its photos.</p>`);
        }
        new PointStage(stageEl, spec, device, preview ? null : onPick);
    });
}

// Each photo, as many as there are: the more, the smaller and fainter each
// light, so that a crowd glows without burning out at once.
function pointLight(count) {
    return {
        radius: Math.max(5, Math.min(14, 19 - 2.6 * Math.log10(count + 1))),
        alpha: Math.max(0.045, Math.min(0.9, 6 / Math.sqrt(count + 1))),
    };
}

// A period of the trail, larger with more photos.
function stepRadius(photos, most) {
    return 8 + 10 * Math.sqrt(photos / Math.max(1, most));
}

// The trail's segments between consecutive steps; closed joins the last to
// the first, for a cycle such as the months of a year.
function trailBetween(steps, { closed = false } = {}) {
    const out = [];
    const n = steps.length / SPRITE_FLOATS;
    const end = closed && n > 2 ? n : n - 1;
    for (let i = 0; i < end; i++) {
        const a = i * SPRITE_FLOATS, b = ((i + 1) % n) * SPRITE_FLOATS;
        out.push(...steps.subarray(a, a + 6), ...steps.subarray(b, b + 6));
    }
    return new Float32Array(out);
}

// The legend's two kinds of light.
function legendDotHTML(text) {
    return `<span class="point-stage-legend-item"><span class="point-stage-legend-dot" aria-hidden="true"></span>${escapeHtml(text)}</span>`;
}

function legendStepHTML(text) {
    return `<span class="point-stage-legend-item"><span class="point-stage-legend-step" aria-hidden="true"></span>${escapeHtml(text)}</span>`;
}

function photoTipHTML(cs, libIndex, id, lines) {
    const thumb = LibraryAPI.thumbURL(cs.libraries[libIndex], id);
    return `<img class="point-stage-tip-thumb" src="${escapeHtml(thumb)}" alt="">${lines}`;
}

/* --- The stage --- */

class PointStage {
    constructor(el, spec, device, onPick) {
        this.el = el;
        this.spec = spec;
        this.onPick = onPick;
        this.size = onPick ? 1 : 0.45;
        this.canvas = el.querySelector('.point-stage-canvas');
        this.camera = new OrbitCamera(spec.distance ? { distance: spec.distance } : {});
        this.turning = !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        this.hovered = null;
        this.stage = new GpuStage(this.canvas, device, {
            draw: (encoder, view, w, h) => this.scene.render(encoder, view, w, h),
            animate: () => this.turning,
            onFrame: dt => this._frame(dt),
        });
        this.scene = new PointScene(device, this.stage.format);
        this.scene.setGeometry({
            sprites: this._sized(spec.sprites, false),
            steps: this._sized(spec.steps, true),
            trail: spec.trail,
            lines: spec.lines,
        });
        this.photoCount = spec.sprites.length / SPRITE_FLOATS;
        if (onPick) {
            this.labels = new StageLabels(el.querySelector('.point-stage-labels'), spec.labels);
            this.ring = el.querySelector('.point-stage-ring');
            this.tip = el.querySelector('.point-stage-tip');
            this._listen();
        }
        this.stage.invalidate();
    }

    // A preview's lights are smaller, and its trail fainter.
    _sized(data, fade) {
        if (this.size === 1) return data;
        const out = data.slice();
        for (let i = 0; i < out.length; i += SPRITE_FLOATS) {
            out[i + 6] *= this.size;
            if (fade) out[i + 7] *= this.size;
        }
        return out;
    }

    // Where photo or step i stands.
    _position(i) {
        const data = i < this.photoCount ? this.spec.sprites : this.spec.steps;
        const o = (i < this.photoCount ? i : i - this.photoCount) * SPRITE_FLOATS;
        return [data[o], data[o + 1], data[o + 2]];
    }

    _frame(dt) {
        if (this.turning) this.camera.turn(dt * 0.12, 0);
        const w = this.stage.cssWidth, h = this.stage.cssHeight;
        this.vp = this.camera.viewProjection(w / h);
        this.scene.setView(this.vp, {
            width: this.canvas.width, height: this.canvas.height,
            scale: this.canvas.width / w, trailWidth: (this.spec.trailWidth ?? 5) * this.size, trailAlpha: 0.9 * this.size,
        });
        this.labels?.place(this.vp, w, h);
        this._placeRing();
    }

    /* --- Input --- */

    _listen() {
        const c = this.canvas;
        const pointers = new Map();
        let moved = 0;
        c.addEventListener('pointerdown', e => {
            c.setPointerCapture(e.pointerId);
            pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
            moved = 0;
        });
        c.addEventListener('pointermove', e => {
            const last = pointers.get(e.pointerId);
            if (!last) { this._hover(e); return; }
            const dx = e.clientX - last.x, dy = e.clientY - last.y;
            moved += Math.abs(dx) + Math.abs(dy);
            if (pointers.size === 2) this._pinch(pointers, e);
            else if (moved > 4) this._turnBy(-dx * 0.008, dy * 0.008);
            pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
        });
        const release = e => {
            const wasClick = pointers.size === 1 && moved <= 4;
            pointers.delete(e.pointerId);
            if (wasClick && e.type === 'pointerup') this._click(e);
        };
        c.addEventListener('pointerup', release);
        c.addEventListener('pointercancel', release);
        c.addEventListener('pointerleave', () => { if (!pointers.size) this._setHovered(null); });
        c.addEventListener('wheel', e => {
            e.preventDefault();
            this._zoomBy(Math.exp(e.deltaY * 0.0015));
        }, { passive: false });
        // While it has the focus, the stage owns the arrow keys (app-keyboard.js
        // defers to .keyboard-owner); Escape gives them back.
        c.addEventListener('focus', () => c.classList.add('keyboard-owner'));
        c.addEventListener('blur', () => c.classList.remove('keyboard-owner'));
        c.addEventListener('keydown', e => this._key(e));
    }

    _key(e) {
        const step = 0.08;
        const actions = {
            ArrowLeft: () => this._turnBy(-step, 0), ArrowRight: () => this._turnBy(step, 0),
            ArrowUp: () => this._turnBy(0, step), ArrowDown: () => this._turnBy(0, -step),
            '+': () => this._zoomBy(0.9), '=': () => this._zoomBy(0.9), '-': () => this._zoomBy(1.1),
            Escape: () => this.canvas.blur(),
        };
        const act = actions[e.key];
        if (!act) return;
        e.preventDefault();
        e.stopPropagation();
        act();
    }

    _turnBy(dYaw, dPitch) {
        this.turning = false;
        this.camera.turn(dYaw, dPitch);
        this.stage.invalidate();
    }

    _zoomBy(factor) {
        this.turning = false;
        this.camera.zoom(factor);
        this.stage.invalidate();
    }

    _pinch(pointers, e) {
        const [a, b] = [...pointers.entries()];
        const other = a[0] === e.pointerId ? b[1] : a[1];
        const last = pointers.get(e.pointerId);
        const before = Math.hypot(last.x - other.x, last.y - other.y);
        const after = Math.hypot(e.clientX - other.x, e.clientY - other.y);
        if (before > 0 && after > 0) this._zoomBy(before / after);
    }

    /* --- Reading a light --- */

    // The nearest light within reach of the pointer; a step of the trail
    // wins over the photos around it.
    _nearest(x, y) {
        if (!this.vp) return null;
        const w = this.stage.cssWidth, h = this.stage.cssHeight;
        const total = this.photoCount + this.spec.steps.length / SPRITE_FLOATS;
        let best = null;
        for (let i = total - 1; i >= 0; i--) {
            const [px, py, pz] = this._position(i);
            const s = projectPoint(this.vp, px, py, pz, w, h);
            if (!s) continue;
            const isStep = i >= this.photoCount;
            const d = Math.hypot(s.x - x, s.y - y);
            if (d > (isStep ? 12 : 8)) continue;
            if (isStep) return i;
            if (!best || d < best.d) best = { i, d };
        }
        return best ? best.i : null;
    }

    _hover(e) {
        const r = this.canvas.getBoundingClientRect();
        this._setHovered(this._nearest(e.clientX - r.left, e.clientY - r.top));
    }

    _which(i) {
        return i < this.photoCount ? ['photo', i] : ['step', i - this.photoCount];
    }

    _setHovered(i) {
        if (i === this.hovered) return;
        this.hovered = i;
        this.canvas.style.cursor = i === null ? '' : 'pointer';
        if (i === null) {
            this.ring.hidden = true;
            this.tip.hidden = true;
            return;
        }
        this.tip.innerHTML = this.spec.tip(...this._which(i));
        this.ring.hidden = false;
        this.tip.hidden = false;
        this._placeRing();
    }

    _placeRing() {
        if (this.hovered === null || !this.vp || !this.ring) return;
        const [px, py, pz] = this._position(this.hovered);
        const s = projectPoint(this.vp, px, py, pz, this.stage.cssWidth, this.stage.cssHeight);
        if (!s) return;
        this.ring.style.transform = `translate(${s.x}px, ${s.y}px)`;
        const right = s.x < this.stage.cssWidth - 240;
        this.tip.style.transform = `translate(${right ? s.x + 16 : s.x - 16}px, ${s.y + 12}px) translateX(${right ? 0 : -100}%)`;
    }

    _click(e) {
        this._hover(e);
        if (this.hovered !== null) this.onPick(this.spec.pick(...this._which(this.hovered)));
    }
}

/* --- Labels on the stage --- */

// HTML over the canvas, so the type stays crisp.
class StageLabels {
    constructor(el, labels) {
        this.items = labels.map(({ text, pos, kind }) => {
            const span = document.createElement('span');
            span.className = `point-stage-label point-stage-label--${kind}`;
            span.textContent = text;
            el.appendChild(span);
            return { span, pos };
        });
    }

    place(vp, w, h) {
        for (const { span, pos } of this.items) {
            const s = projectPoint(vp, pos[0], pos[1], pos[2], w, h);
            span.hidden = !s;
            if (s) span.style.transform = `translate(${s.x}px, ${s.y}px)`;
        }
    }
}
