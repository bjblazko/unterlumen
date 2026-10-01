// gpu-stage.js — what every WebGPU view needs (ADR-0046): one device for the
// page, a canvas that follows its size, an orbit camera and a loop that draws
// only when something moved. There is no WebGL fallback: without WebGPU a
// view says so instead.

let gpuDevicePromise = null;

// The page's device, or null when the browser offers no WebGPU.
function gpuDevice() {
    if (!gpuDevicePromise) {
        gpuDevicePromise = (async () => {
            if (!navigator.gpu) return null;
            const adapter = await navigator.gpu.requestAdapter();
            if (!adapter) return null;
            const device = await adapter.requestDevice();
            device.lost.then(() => { gpuDevicePromise = null; });
            return device;
        })().catch(() => null);
    }
    return gpuDevicePromise;
}

/* --- Matrices (column-major, as WGSL reads them) --- */

function mat4Perspective(fovY, aspect, near, far) {
    const f = 1 / Math.tan(fovY / 2);
    const nf = 1 / (near - far);
    return new Float32Array([f / aspect, 0, 0, 0, 0, f, 0, 0, 0, 0, far * nf, -1, 0, 0, far * near * nf, 0]);
}

function mat4LookAt(eye, target, up) {
    const sub = (a, b) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
    const norm = v => { const l = Math.hypot(...v) || 1; return v.map(x => x / l); };
    const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
    const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
    const z = norm(sub(eye, target));
    const x = norm(cross(up, z));
    const y = cross(z, x);
    return new Float32Array([x[0], y[0], z[0], 0, x[1], y[1], z[1], 0, x[2], y[2], z[2], 0, -dot(x, eye), -dot(y, eye), -dot(z, eye), 1]);
}

function mat4Multiply(a, b) {
    const out = new Float32Array(16);
    for (let c = 0; c < 4; c++) {
        for (let r = 0; r < 4; r++) {
            out[c * 4 + r] = a[r] * b[c * 4] + a[4 + r] * b[c * 4 + 1] + a[8 + r] * b[c * 4 + 2] + a[12 + r] * b[c * 4 + 3];
        }
    }
    return out;
}

/* --- Orbit camera --- */

// Turns around the origin: yaw around the vertical, pitch above or below the
// horizon, distance to the origin.
class OrbitCamera {
    constructor({ yaw = 0.6, pitch = 0.3, distance = 4 } = {}) {
        this.yaw = yaw;
        this.pitch = pitch;
        this.distance = distance;
    }

    turn(dYaw, dPitch) {
        this.yaw += dYaw;
        this.pitch = Math.max(-1.45, Math.min(1.45, this.pitch + dPitch));
    }

    zoom(factor) {
        this.distance = Math.max(2.2, Math.min(9, this.distance * factor));
    }

    viewProjection(aspect) {
        const cp = Math.cos(this.pitch);
        const eye = [this.distance * cp * Math.sin(this.yaw), this.distance * Math.sin(this.pitch), this.distance * cp * Math.cos(this.yaw)];
        return mat4Multiply(mat4Perspective(Math.PI / 5, aspect, 0.1, 40), mat4LookAt(eye, [0, 0, 0], [0, 1, 0]));
    }
}

// A point in CSS pixels of a w × h canvas, with its depth (clip w), or null
// behind the camera.
function projectPoint(vp, x, y, z, w, h) {
    const cx = vp[0] * x + vp[4] * y + vp[8] * z + vp[12];
    const cy = vp[1] * x + vp[5] * y + vp[9] * z + vp[13];
    const cw = vp[3] * x + vp[7] * y + vp[11] * z + vp[15];
    if (cw <= 0) return null;
    return { x: (cx / cw * 0.5 + 0.5) * w, y: (0.5 - cy / cw * 0.5) * h, depth: cw };
}

/* --- Stage --- */

// A canvas drawn by draw(encoder, view, width, height) whenever invalidate()
// asks for it, or on every frame while animate() returns true. It stops for
// good once the canvas has left the page.
class GpuStage {
    constructor(canvas, device, { draw, animate = () => false, onFrame = () => {} }) {
        this.canvas = canvas;
        this.device = device;
        this.format = navigator.gpu.getPreferredCanvasFormat();
        this.context = canvas.getContext('webgpu');
        this.context.configure({ device, format: this.format, alphaMode: 'opaque' });
        this._draw = draw;
        this._animate = animate;
        this._onFrame = onFrame;
        this._pending = false;
        this._visible = true;
        this._last = 0;
        this._resizeObserver = new ResizeObserver(() => this._resize());
        this._resizeObserver.observe(canvas);
        this._visibility = new IntersectionObserver(([e]) => {
            this._visible = e.isIntersecting;
            if (this._visible) this.invalidate();
        });
        this._visibility.observe(canvas);
    }

    get cssWidth() { return this.canvas.clientWidth || 1; }
    get cssHeight() { return this.canvas.clientHeight || 1; }

    _resize() {
        const dpr = window.devicePixelRatio || 1;
        const w = Math.max(1, Math.round(this.cssWidth * dpr));
        const h = Math.max(1, Math.round(this.cssHeight * dpr));
        if (this.canvas.width !== w || this.canvas.height !== h) {
            this.canvas.width = w;
            this.canvas.height = h;
        }
        this.invalidate();
    }

    invalidate() {
        if (this._pending || !this._visible) return;
        this._pending = true;
        requestAnimationFrame(t => this._frame(t));
    }

    _frame(t) {
        this._pending = false;
        if (!this.canvas.isConnected) { this.dispose(); return; }
        const dt = this._last ? Math.min(0.1, (t - this._last) / 1000) : 0;
        this._last = t;
        this._onFrame(dt);
        const encoder = this.device.createCommandEncoder();
        this._draw(encoder, this.context.getCurrentTexture().createView(), this.canvas.width, this.canvas.height);
        this.device.queue.submit([encoder.finish()]);
        if (this._animate()) this.invalidate();
        else this._last = 0;
    }

    dispose() {
        this._resizeObserver.disconnect();
        this._visibility.disconnect();
        this._visible = false;
    }
}
