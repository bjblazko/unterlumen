// colour-space-view.js — the Colour space topic (ADR-0046): every analysed
// photo as a point of light at its main colour in OKLab, and a trail through
// the mean colour of each period's colour photos. Turned by dragging, the
// wheel or the arrow keys; a point read by hovering, its photos shown by a
// click. The GPU side is in colour-space-gpu.js.

const COLOUR_SPACE_NO_GPU = 'This view needs WebGPU, which this browser does not offer. Current versions of Chrome, Edge and Safari have it.';

// renderColourSpace draws cs (/api/library/colour-space) into el. onPick
// gets { photo: { id, lib, date } } or { step } for a period of the trail.
// A preview turns by itself and takes no input.
function renderColourSpace(el, cs, onPick, { preview = false } = {}) {
    if (noColourData(el, cs)) return;
    el.innerHTML = `
        <div class="colour-space${preview ? ' colour-space--preview' : ''}">
            <canvas class="colour-space-canvas"${preview ? '' : ` tabindex="0" role="img" aria-label="${escapeHtml(colourSpaceSummary(cs))}"`}></canvas>
            ${preview ? '' : `<div class="colour-space-labels" aria-hidden="true"></div><div class="colour-space-ring" hidden></div><div class="colour-space-tip" hidden></div>${colourSpaceLegendHTML(cs)}`}
        </div>
        ${preview ? '' : `<p class="colour-space-help">${escapeHtml(colourSpaceSummary(cs))} Drag to turn the view and scroll or pinch to zoom; once it is selected, the arrow keys turn it and + and − zoom. Point at a light to read its colour, click it to see its photos.</p>`}`;
    gpuDevice().then(device => {
        if (!device) {
            el.querySelector('.colour-space').innerHTML = `<p class="colour-space-nogpu">${COLOUR_SPACE_NO_GPU}</p>`;
            return;
        }
        new ColourSpaceView(el, cs, device, preview ? null : onPick);
    });
}

// What the two kinds of light mean, at the foot of the stage.
function colourSpaceLegendHTML(cs) {
    const unit = cs.granularity === 'year' ? 'year' : 'month';
    const path = cs.path;
    const span = path.length > 1 ? `, from ${path[0].period} to ${path[path.length - 1].period}` : '';
    return `<div class="colour-space-legend">
        <span class="colour-space-legend-item"><span class="colour-space-legend-dot" aria-hidden="true"></span>One photo, at its main colour</span>
        <span class="colour-space-legend-item"><span class="colour-space-legend-step" aria-hidden="true"></span>One ${unit}: the average colour of its colour photos, larger with more photos. The line joins the ${unit}s in time order${escapeHtml(span)}.</span>
    </div>`;
}

function colourSpaceSummary(cs) {
    const n = cs.analysedPhotos;
    const path = cs.path;
    const unit = cs.granularity === 'year' ? 'year' : 'month';
    const trail = path.length > 1
        ? ` The trail joins the mean colour of each ${unit}'s colour photos, from ${path[0].period} to ${path[path.length - 1].period}.`
        : '';
    return `${formatCount(n)} ${n === 1 ? 'photo' : 'photos'}, each at its main colour: lightness up, hue around, chroma outward.${trail}`;
}

/* --- Geometry --- */

// The trail is drawn more colourful than it is: a mean of many colours is
// greyer than any of them, and the lights around it show the real ones.
const TRAIL_BOOST = 2.5;

// The more photos, the smaller and fainter each light, so that a crowd of
// similar colours glows without burning out. size scales every light, for
// a preview the size of a card.
function colourSpaceGeometry(cs, size) {
    const p = cs.points;
    const n = p.id.length;
    const radius = size * Math.max(5, Math.min(14, 19 - 2.6 * Math.log10(n + 1)));
    const alpha = Math.max(0.045, Math.min(0.9, 6 / Math.sqrt(n + 1)));
    const sprites = [];
    for (let i = 0; i < n; i++) sprites.push(p.l[i], p.a[i], p.b[i], radius, alpha, 1);
    const most = Math.max(1, ...cs.path.map(s => s.photos));
    const steps = [];
    for (const s of cs.path) steps.push(s.l, s.a, s.b, size * (8 + 10 * Math.sqrt(s.photos / most)), size, TRAIL_BOOST);
    const trail = [];
    for (let i = 1; i < cs.path.length; i++) {
        const a = cs.path[i - 1], b = cs.path[i];
        trail.push(a.l, a.a, a.b, b.l, b.a, b.b);
    }
    return { sprites: new Float32Array(sprites), steps: new Float32Array(steps), trail: new Float32Array(trail), grid: colourSpaceGrid() };
}

function labToLCh(l, a, b) {
    const c = Math.hypot(a, b);
    const h = (Math.atan2(b, a) * 180 / Math.PI + 360) % 360;
    return { l, c, h };
}

/* --- The view --- */

class ColourSpaceView {
    constructor(el, cs, device, onPick) {
        this.el = el;
        this.cs = cs;
        this.onPick = onPick;
        this.canvas = el.querySelector('.colour-space-canvas');
        this.camera = new OrbitCamera();
        this.turning = !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        this.hovered = null;
        this.stage = new GpuStage(this.canvas, device, {
            clear: COLOUR_SPACE_CLEAR,
            draw: pass => this.scene.draw(pass),
            animate: () => this.turning,
            onFrame: dt => this._frame(dt),
        });
        this.scene = new ColourSpaceScene(device, this.stage.format);
        this.size = onPick ? 1 : 0.45;
        this.scene.setGeometry(colourSpaceGeometry(cs, this.size));
        this._positions = this._placeAll();
        if (onPick) {
            this.labels = new ColourSpaceLabels(el.querySelector('.colour-space-labels'), cs);
            this.ring = el.querySelector('.colour-space-ring');
            this.tip = el.querySelector('.colour-space-tip');
            this._listen();
        }
        this.stage.invalidate();
    }

    // Every photo and then every period, where the shader puts them.
    _placeAll() {
        const p = this.cs.points;
        const out = [];
        for (let i = 0; i < p.id.length; i++) out.push(colourSpacePosition(p.l[i], p.a[i], p.b[i]));
        for (const s of this.cs.path) out.push(colourSpacePosition(s.l, s.a, s.b));
        return out;
    }

    _frame(dt) {
        if (this.turning) this.camera.turn(dt * 0.12, 0);
        const w = this.stage.cssWidth, h = this.stage.cssHeight;
        this.vp = this.camera.viewProjection(w / h);
        this.scene.setView(this.vp, {
            width: this.canvas.width, height: this.canvas.height,
            scale: this.canvas.width / w, trailWidth: 5 * this.size, trailAlpha: 0.9 * this.size, lift: 0.25, trailBoost: TRAIL_BOOST,
        });
        this.labels?.place(this.vp, w, h);
        this._placeRing();
    }

    stopTurning() {
        this.turning = false;
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
        // While it has the focus, the view owns the arrow keys (app-keyboard.js
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
        this.stopTurning();
        this.camera.turn(dYaw, dPitch);
        this.stage.invalidate();
    }

    _zoomBy(factor) {
        this.stopTurning();
        this.camera.zoom(factor);
        this.stage.invalidate();
    }

    _pinch(pointers, e) {
        const [a, b] = [...pointers.entries()];
        const other = a[0] === e.pointerId ? b[1] : a[1];
        const before = Math.hypot(pointers.get(e.pointerId).x - other.x, pointers.get(e.pointerId).y - other.y);
        const after = Math.hypot(e.clientX - other.x, e.clientY - other.y);
        if (before > 0 && after > 0) this._zoomBy(before / after);
    }

    /* --- Reading a light --- */

    // The nearest light within reach of the pointer; a period of the trail
    // wins over the photos around it.
    _nearest(x, y) {
        if (!this.vp) return null;
        const w = this.stage.cssWidth, h = this.stage.cssHeight;
        const photos = this.cs.points.id.length;
        let best = null;
        for (let i = this._positions.length - 1; i >= 0; i--) {
            const [px, py, pz] = this._positions[i];
            const s = projectPoint(this.vp, px, py, pz, w, h);
            if (!s) continue;
            const reach = i >= photos ? 12 : 8;
            const d = Math.hypot(s.x - x, s.y - y);
            if (d > reach) continue;
            if (i >= photos) return i;
            if (!best || d < best.d) best = { i, d };
        }
        return best ? best.i : null;
    }

    _hover(e) {
        const r = this.canvas.getBoundingClientRect();
        this._setHovered(this._nearest(e.clientX - r.left, e.clientY - r.top));
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
        this.tip.innerHTML = this._tipHTML(i);
        this.ring.hidden = false;
        this.tip.hidden = false;
        this._placeRing();
    }

    _placeRing() {
        if (this.hovered === null || !this.vp || !this.ring) return;
        const [px, py, pz] = this._positions[this.hovered];
        const s = projectPoint(this.vp, px, py, pz, this.stage.cssWidth, this.stage.cssHeight);
        if (!s) return;
        this.ring.style.transform = `translate(${s.x}px, ${s.y}px)`;
        const right = s.x < this.stage.cssWidth - 240;
        this.tip.style.transform = `translate(${right ? s.x + 16 : s.x - 16}px, ${s.y + 12}px) translateX(${right ? 0 : -100}%)`;
    }

    _tipHTML(i) {
        const p = this.cs.points;
        const photos = p.id.length;
        if (i >= photos) {
            const s = this.cs.path[i - photos];
            return `<span class="colour-space-tip-title">${escapeHtml(s.period)}</span>
                <span>${formatCount(s.photos)} colour ${s.photos === 1 ? 'photo' : 'photos'}, on average</span>
                ${colourValuesHTML(labToLCh(s.l, s.a, s.b))}`;
        }
        const lch = labToLCh(p.l[i], p.a[i], p.b[i]);
        const thumb = LibraryAPI.thumbURL(this.cs.libraries[p.lib[i]], p.id[i]);
        return `<img class="colour-space-tip-thumb" src="${escapeHtml(thumb)}" alt="">
            <span class="colour-space-tip-title">${escapeHtml(p.date[i] ? p.date[i].slice(0, 10) : 'No date')}</span>
            <span>${escapeHtml(capitalise(COLOUR_CLASSES[p.mono[i]] ?? ''))}, mostly ${escapeHtml(hueNameOf(lch))}</span>
            ${colourValuesHTML(lch)}`;
    }

    _click(e) {
        this._hover(e);
        const i = this.hovered;
        if (i === null) return;
        const p = this.cs.points;
        if (i >= p.id.length) this.onPick({ step: this.cs.path[i - p.id.length] });
        else this.onPick({ photo: { id: p.id[i], lib: this.cs.libraries[p.lib[i]], date: p.date[i] } });
    }
}

function colourValuesHTML({ l, c, h }) {
    return `<span class="colour-space-tip-values"><span class="colour-space-swatch" style="background:${photoColour({ l, c, h })}"></span>L ${l.toFixed(2)} · C ${c.toFixed(3)} · h ${Math.round(h)}°</span>`;
}

/* --- Labels on the stage --- */

// Hue names around the floor, lightness up the axis, and the first and last
// period of the trail; HTML over the canvas, so the type stays crisp.
class ColourSpaceLabels {
    constructor(el, cs) {
        this.el = el;
        this.items = [];
        for (let bin = 0; bin < 12; bin++) {
            const t = (bin * 30 + 15) * Math.PI / 180;
            this._add(capitalise(hueName(bin)), colourSpacePosition(0, 0.245 * Math.cos(t), 0.245 * Math.sin(t)), 'hue');
        }
        for (const l of [0, 0.5, 1]) this._add(`L ${l}`, colourSpacePosition(l, 0, 0), 'axis');
        const path = cs.path;
        if (path.length > 1) {
            for (const s of [path[0], path[path.length - 1]]) this._add(s.period, colourSpacePosition(s.l, s.a, s.b), 'period');
        }
    }

    _add(text, pos, kind) {
        const span = document.createElement('span');
        span.className = `colour-space-label colour-space-label--${kind}`;
        span.textContent = text;
        this.el.appendChild(span);
        this.items.push({ span, pos });
    }

    place(vp, w, h) {
        for (const { span, pos } of this.items) {
            const s = projectPoint(vp, pos[0], pos[1], pos[2], w, h);
            span.hidden = !s;
            if (s) span.style.transform = `translate(${s.x}px, ${s.y}px)`;
        }
    }
}
