// colour-space-gpu.js — what the Colour space draws on the GPU (ADR-0046):
// each photo as a soft point of light in its own colour, the periods' path as
// a glowing trail, and a faint floor of chroma rings and hue spokes. Light
// adds up (additive blending), so where many photos share a colour it glows.
//
// Positions are OKLab: a and b across the floor, lightness up.

const COLOUR_SPACE_WGSL = /* wgsl */`
struct U {
    vp: mat4x4f,
    viewport: vec2f,   // device pixels
    scale: f32,        // device pixels per CSS pixel
    trailWidth: f32,   // CSS pixels
    trailAlpha: f32,
    lift: f32,         // how far dark colours are lifted so they still glow
    trailBoost: f32,   // the trail's chroma, multiplied: a mean of colours is greyer than any of them
    _pad: f32,
};
@group(0) @binding(0) var<uniform> u: U;

// L 0…1 runs from -1 to 1; a and b, mostly within ±0.2 for photos, are
// stretched to about ±1.
fn place(lab: vec3f) -> vec4f {
    return u.vp * vec4f(lab.y * 5.0, (lab.x - 0.5) * 2.0, lab.z * 5.0, 1.0);
}

fn glowColour(lab: vec3f, boost: f32) -> vec3f {
    let L = u.lift + (1.0 - u.lift) * lab.x;
    let A = lab.y * boost;
    let B = lab.z * boost;
    let l_ = L + 0.3963377774 * A + 0.2158037573 * B;
    let m_ = L - 0.1055613458 * A - 0.0638541728 * B;
    let s_ = L - 0.0894841775 * A - 1.2914855480 * B;
    let l = l_ * l_ * l_;
    let m = m_ * m_ * m_;
    let s = s_ * s_ * s_;
    let lin = clamp(vec3f(
         4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
        -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
        -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s), vec3f(0.0), vec3f(1.0));
    return select(1.055 * pow(lin, vec3f(1.0 / 2.4)) - 0.055, 12.92 * lin, lin <= vec3f(0.0031308));
}

struct SpriteOut {
    @builtin(position) pos: vec4f,
    @location(0) uv: vec2f,
    @location(1) colour: vec3f,
    @location(2) alpha: f32,
};

@vertex fn spriteVertex(@builtin(vertex_index) vi: u32, @location(0) lab: vec3f, @location(1) sizeAlpha: vec3f) -> SpriteOut {
    var quad = array<vec2f, 6>(vec2f(-1, -1), vec2f(1, -1), vec2f(-1, 1), vec2f(-1, 1), vec2f(1, -1), vec2f(1, 1));
    let q = quad[vi];
    let c = place(lab);
    var o: SpriteOut;
    o.pos = c + vec4f(q * sizeAlpha.x * u.scale * 2.0 / u.viewport * c.w, 0.0, 0.0);
    o.uv = q;
    o.colour = glowColour(lab, sizeAlpha.z);
    o.alpha = sizeAlpha.y;
    return o;
}

@fragment fn spriteFragment(i: SpriteOut) -> @location(0) vec4f {
    let d = dot(i.uv, i.uv);
    if (d > 1.0) { discard; }
    let a = (exp(-d * 14.0) + 0.45 * exp(-d * 4.0)) * i.alpha;
    return vec4f(i.colour * a, a);
}

struct TrailOut {
    @builtin(position) pos: vec4f,
    @location(0) side: f32,
    @location(1) colour: vec3f,
};

@vertex fn trailVertex(@builtin(vertex_index) vi: u32, @location(0) p0: vec3f, @location(1) p1: vec3f) -> TrailOut {
    var ends = array<f32, 6>(0, 0, 1, 1, 0, 1);
    var sides = array<f32, 6>(-1, 1, -1, -1, 1, 1);
    let ca = place(p0);
    let cb = place(p1);
    var dir = (cb.xy / cb.w - ca.xy / ca.w) * u.viewport;
    if (length(dir) < 0.001) { dir = vec2f(1.0, 0.0); }
    let n = normalize(vec2f(-dir.y, dir.x));
    let c = mix(ca, cb, ends[vi]);
    var o: TrailOut;
    o.pos = c + vec4f(n * sides[vi] * u.trailWidth * u.scale / u.viewport * c.w, 0.0, 0.0);
    o.side = sides[vi];
    o.colour = mix(glowColour(p0, u.trailBoost), glowColour(p1, u.trailBoost), ends[vi]);
    return o;
}

@fragment fn trailFragment(i: TrailOut) -> @location(0) vec4f {
    let a = exp(-i.side * i.side * 5.0) * u.trailAlpha;
    return vec4f(i.colour * a, a);
}

struct GridOut {
    @builtin(position) pos: vec4f,
    @location(0) alpha: f32,
};

@vertex fn gridVertex(@location(0) lab: vec3f, @location(1) alpha: f32) -> GridOut {
    var o: GridOut;
    o.pos = place(lab);
    o.alpha = alpha;
    return o;
}

@fragment fn gridFragment(i: GridOut) -> @location(0) vec4f {
    return vec4f(vec3f(0.55, 0.6, 0.7) * i.alpha, i.alpha);
}
`;

const ADDITIVE = {
    color: { srcFactor: 'one', dstFactor: 'one', operation: 'add' },
    alpha: { srcFactor: 'one', dstFactor: 'one', operation: 'add' },
};

// The trail is laid over the cloud rather than added to it, so it still
// shows where many photos have burnt the cloud to white.
const OVER = {
    color: { srcFactor: 'one', dstFactor: 'one-minus-src-alpha', operation: 'add' },
    alpha: { srcFactor: 'one', dstFactor: 'one-minus-src-alpha', operation: 'add' },
};

// The stage the light is drawn on, the same in both themes.
const COLOUR_SPACE_CLEAR = { r: 0.027, g: 0.031, b: 0.043, a: 1 };

const UNIFORM_FLOATS = 24;

class ColourSpaceScene {
    constructor(device, format) {
        this.device = device;
        const module = device.createShaderModule({ code: COLOUR_SPACE_WGSL });
        this.uniforms = device.createBuffer({ size: UNIFORM_FLOATS * 4, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
        const layout = device.createBindGroupLayout({
            entries: [{ binding: 0, visibility: GPUShaderStage.VERTEX | GPUShaderStage.FRAGMENT, buffer: {} }],
        });
        this.bindGroup = device.createBindGroup({ layout, entries: [{ binding: 0, resource: { buffer: this.uniforms } }] });
        const pipelineLayout = device.createPipelineLayout({ bindGroupLayouts: [layout] });
        const pipeline = (vertex, fragment, buffers, { topology = 'triangle-list', blend = ADDITIVE } = {}) => device.createRenderPipeline({
            layout: pipelineLayout,
            vertex: { module, entryPoint: vertex, buffers },
            fragment: { module, entryPoint: fragment, targets: [{ format, blend }] },
            primitive: { topology },
        });
        const vec3At = (location, offset) => ({ shaderLocation: location, offset, format: 'float32x3' });
        const spriteBuffers = [{ arrayStride: 24, stepMode: 'instance', attributes: [vec3At(0, 0), vec3At(1, 12)] }];
        this.spritePipeline = pipeline('spriteVertex', 'spriteFragment', spriteBuffers);
        this.stepPipeline = pipeline('spriteVertex', 'spriteFragment', spriteBuffers, { blend: OVER });
        this.trailPipeline = pipeline('trailVertex', 'trailFragment', [{
            arrayStride: 24, stepMode: 'instance', attributes: [vec3At(0, 0), vec3At(1, 12)],
        }], { blend: OVER });
        this.gridPipeline = pipeline('gridVertex', 'gridFragment', [{
            arrayStride: 16, attributes: [vec3At(0, 0), { shaderLocation: 1, offset: 12, format: 'float32' }],
        }], { topology: 'line-list' });
    }

    _buffer(data) {
        const buf = this.device.createBuffer({ size: Math.max(16, data.byteLength), usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST });
        this.device.queue.writeBuffer(buf, 0, data);
        return buf;
    }

    // sprites (the photos) and steps (the periods of the trail): [L, a, b,
    // radius in CSS pixels, alpha, chroma boost] each. trail: [L, a, b, L, a,
    // b] per segment. grid: [L, a, b, alpha] per vertex.
    setGeometry({ sprites, steps, trail, grid }) {
        this.spriteBuffer = this._buffer(sprites);
        this.spriteCount = sprites.length / 6;
        this.stepBuffer = this._buffer(steps);
        this.stepCount = steps.length / 6;
        this.trailBuffer = this._buffer(trail);
        this.trailCount = trail.length / 6;
        this.gridBuffer = this._buffer(grid);
        this.gridCount = grid.length / 4;
    }

    setView(vp, { width, height, scale, trailWidth, trailAlpha, lift, trailBoost }) {
        const data = new Float32Array(UNIFORM_FLOATS);
        data.set(vp, 0);
        data.set([width, height, scale, trailWidth, trailAlpha, lift, trailBoost], 16);
        this.device.queue.writeBuffer(this.uniforms, 0, data);
    }

    draw(pass) {
        pass.setBindGroup(0, this.bindGroup);
        pass.setPipeline(this.gridPipeline);
        pass.setVertexBuffer(0, this.gridBuffer);
        pass.draw(this.gridCount);
        pass.setPipeline(this.spritePipeline);
        pass.setVertexBuffer(0, this.spriteBuffer);
        pass.draw(6, this.spriteCount);
        if (this.trailCount) {
            pass.setPipeline(this.trailPipeline);
            pass.setVertexBuffer(0, this.trailBuffer);
            pass.draw(6, this.trailCount);
        }
        if (this.stepCount) {
            pass.setPipeline(this.stepPipeline);
            pass.setVertexBuffer(0, this.stepBuffer);
            pass.draw(6, this.stepCount);
        }
    }
}

// The floor at L 0: chroma rings at 0.05, 0.1, 0.15 and 0.2, a spoke at every 30°
// sector edge, and the lightness axis standing in the middle.
function colourSpaceGrid() {
    const v = [];
    const line = (a, b, alpha) => v.push(...a, alpha, ...b, alpha);
    for (const r of [0.05, 0.1, 0.15, 0.2]) {
        const steps = 96;
        for (let i = 0; i < steps; i++) {
            const t0 = i / steps * 2 * Math.PI, t1 = (i + 1) / steps * 2 * Math.PI;
            line([0, r * Math.cos(t0), r * Math.sin(t0)], [0, r * Math.cos(t1), r * Math.sin(t1)], 0.22);
        }
    }
    for (let bin = 0; bin < 12; bin++) {
        const t = bin * 30 * Math.PI / 180;
        line([0, 0.015 * Math.cos(t), 0.015 * Math.sin(t)], [0, 0.22 * Math.cos(t), 0.22 * Math.sin(t)], 0.12);
    }
    line([0, 0, 0], [1, 0, 0], 0.35);
    for (const l of [0.25, 0.5, 0.75, 1]) line([l, -0.008, 0], [l, 0.008, 0], 0.35);
    return new Float32Array(v);
}

// Where place() in the shader puts a colour, for the same maths on the CPU.
function colourSpacePosition(l, a, b) {
    return [a * 5, (l - 0.5) * 2.0, b * 5];
}
