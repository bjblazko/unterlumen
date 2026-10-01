// point-scene.js — what a point stage draws on the GPU (ADR-0046): soft
// points of light, a glowing trail and faint lines. A view hands over
// positions in stage space (about -1…1 on each axis) and sRGB colours;
// light adds up (additive blending), so where many photos meet it glows.
//
// The light is added up in a float texture first, then mapped onto the
// screen (tone mapping). The mapping keeps each colour's hue and only
// compresses how bright it is, so a crowd of blue photos glows saturated
// blue instead of burning out to white.

const POINT_SCENE_WGSL = /* wgsl */`
struct U {
    vp: mat4x4f,
    viewport: vec2f,   // device pixels
    scale: f32,        // device pixels per CSS pixel
    trailWidth: f32,   // CSS pixels
    trailAlpha: f32,
};
@group(0) @binding(0) var<uniform> u: U;

struct SpriteOut {
    @builtin(position) pos: vec4f,
    @location(0) uv: vec2f,
    @location(1) colour: vec3f,
    @location(2) alpha: f32,
};

@vertex fn spriteVertex(@builtin(vertex_index) vi: u32, @location(0) at: vec3f, @location(1) colour: vec3f, @location(2) sizeAlpha: vec2f) -> SpriteOut {
    var quad = array<vec2f, 6>(vec2f(-1, -1), vec2f(1, -1), vec2f(-1, 1), vec2f(-1, 1), vec2f(1, -1), vec2f(1, 1));
    let q = quad[vi];
    let c = u.vp * vec4f(at, 1.0);
    var o: SpriteOut;
    o.pos = c + vec4f(q * sizeAlpha.x * u.scale * 2.0 / u.viewport * c.w, 0.0, 0.0);
    o.uv = q;
    o.colour = colour;
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

@vertex fn trailVertex(@builtin(vertex_index) vi: u32, @location(0) p0: vec3f, @location(1) c0: vec3f, @location(2) p1: vec3f, @location(3) c1: vec3f) -> TrailOut {
    var ends = array<f32, 6>(0, 0, 1, 1, 0, 1);
    var sides = array<f32, 6>(-1, 1, -1, -1, 1, 1);
    let ca = u.vp * vec4f(p0, 1.0);
    let cb = u.vp * vec4f(p1, 1.0);
    var dir = (cb.xy / cb.w - ca.xy / ca.w) * u.viewport;
    if (length(dir) < 0.001) { dir = vec2f(1.0, 0.0); }
    let n = normalize(vec2f(-dir.y, dir.x));
    let c = mix(ca, cb, ends[vi]);
    var o: TrailOut;
    o.pos = c + vec4f(n * sides[vi] * u.trailWidth * u.scale / u.viewport * c.w, 0.0, 0.0);
    o.side = sides[vi];
    o.colour = mix(c0, c1, ends[vi]);
    return o;
}

@fragment fn trailFragment(i: TrailOut) -> @location(0) vec4f {
    let a = exp(-i.side * i.side * 5.0) * u.trailAlpha;
    return vec4f(i.colour * a, a);
}

struct LineOut {
    @builtin(position) pos: vec4f,
    @location(0) alpha: f32,
};

@vertex fn lineVertex(@location(0) at: vec3f, @location(1) alpha: f32) -> LineOut {
    var o: LineOut;
    o.pos = u.vp * vec4f(at, 1.0);
    o.alpha = alpha;
    return o;
}

@fragment fn lineFragment(i: LineOut) -> @location(0) vec4f {
    return vec4f(vec3f(0.55, 0.6, 0.7) * i.alpha, i.alpha);
}
`;

const TONE_MAP_WGSL = /* wgsl */`
@group(0) @binding(0) var light: texture_2d<f32>;

// The stage behind the light, the same in both themes.
const STAGE = vec3f(0.027, 0.031, 0.043);

// How bright a single light shows: above 1, a faint one is lifted before
// the mapping compresses a crowd.
const EXPOSURE = 1.5;

@vertex fn fullVertex(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4f {
    var corners = array<vec2f, 3>(vec2f(-1, -1), vec2f(3, -1), vec2f(-1, 3));
    return vec4f(corners[vi], 0.0, 1.0);
}

@fragment fn toneFragment(@builtin(position) at: vec4f) -> @location(0) vec4f {
    let c = max(textureLoad(light, vec2i(at.xy), 0).rgb, vec3f(0.0));
    let m = max(max(c.r, c.g), c.b);
    var shown = c;
    if (m > 0.0001) { shown = c * ((1.0 - exp(-m * EXPOSURE)) / m); }
    return vec4f(STAGE + shown * (1.0 - STAGE), 1.0);
}
`;

// Where the light is added up before it is mapped onto the screen.
const LIGHT_FORMAT = 'rgba16float';

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

const UNIFORM_FLOATS = 24;
const SPRITE_FLOATS = 8;  // x, y, z, r, g, b, radius in CSS pixels, alpha
const SEGMENT_FLOATS = 12; // x, y, z, r, g, b of each end
const LINE_FLOATS = 4;    // x, y, z, alpha per vertex

class PointScene {
    constructor(device, format) {
        this.device = device;
        const module = device.createShaderModule({ code: POINT_SCENE_WGSL });
        this.uniforms = device.createBuffer({ size: UNIFORM_FLOATS * 4, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
        const layout = device.createBindGroupLayout({
            entries: [{ binding: 0, visibility: GPUShaderStage.VERTEX | GPUShaderStage.FRAGMENT, buffer: {} }],
        });
        this.bindGroup = device.createBindGroup({ layout, entries: [{ binding: 0, resource: { buffer: this.uniforms } }] });
        const pipelineLayout = device.createPipelineLayout({ bindGroupLayouts: [layout] });
        const pipeline = (vertex, fragment, buffers, { topology = 'triangle-list', blend = ADDITIVE } = {}) => device.createRenderPipeline({
            layout: pipelineLayout,
            vertex: { module, entryPoint: vertex, buffers },
            fragment: { module, entryPoint: fragment, targets: [{ format: LIGHT_FORMAT, blend }] },
            primitive: { topology },
        });
        const attr = (location, offset, n) => ({ shaderLocation: location, offset: offset * 4, format: n === 1 ? 'float32' : `float32x${n}` });
        const sprites = [{ arrayStride: SPRITE_FLOATS * 4, stepMode: 'instance', attributes: [attr(0, 0, 3), attr(1, 3, 3), attr(2, 6, 2)] }];
        this.spritePipeline = pipeline('spriteVertex', 'spriteFragment', sprites);
        this.stepPipeline = pipeline('spriteVertex', 'spriteFragment', sprites, { blend: OVER });
        this.trailPipeline = pipeline('trailVertex', 'trailFragment', [{
            arrayStride: SEGMENT_FLOATS * 4, stepMode: 'instance',
            attributes: [attr(0, 0, 3), attr(1, 3, 3), attr(2, 6, 3), attr(3, 9, 3)],
        }], { blend: OVER });
        this.linePipeline = pipeline('lineVertex', 'lineFragment', [{
            arrayStride: LINE_FLOATS * 4, attributes: [attr(0, 0, 3), attr(1, 3, 1)],
        }], { topology: 'line-list' });
        const tone = device.createShaderModule({ code: TONE_MAP_WGSL });
        this.tonePipeline = device.createRenderPipeline({
            layout: 'auto',
            vertex: { module: tone, entryPoint: 'fullVertex' },
            fragment: { module: tone, entryPoint: 'toneFragment', targets: [{ format }] },
        });
    }

    // The float texture the light adds up in, as large as the canvas.
    _lightTexture(width, height) {
        if (this.light?.width === width && this.light?.height === height) return this.light;
        this.light?.destroy();
        this.light = this.device.createTexture({
            size: [width, height], format: LIGHT_FORMAT,
            usage: GPUTextureUsage.RENDER_ATTACHMENT | GPUTextureUsage.TEXTURE_BINDING,
        });
        this.toneBindGroup = this.device.createBindGroup({
            layout: this.tonePipeline.getBindGroupLayout(0),
            entries: [{ binding: 0, resource: this.light.createView() }],
        });
        return this.light;
    }

    render(encoder, view, width, height) {
        const light = this._lightTexture(width, height);
        const pass = encoder.beginRenderPass({
            colorAttachments: [{ view: light.createView(), clearValue: { r: 0, g: 0, b: 0, a: 0 }, loadOp: 'clear', storeOp: 'store' }],
        });
        this._drawLight(pass);
        pass.end();
        const tone = encoder.beginRenderPass({
            colorAttachments: [{ view, clearValue: { r: 0, g: 0, b: 0, a: 1 }, loadOp: 'clear', storeOp: 'store' }],
        });
        tone.setPipeline(this.tonePipeline);
        tone.setBindGroup(0, this.toneBindGroup);
        tone.draw(3);
        tone.end();
    }

    _buffer(data) {
        const buf = this.device.createBuffer({ size: Math.max(16, data.byteLength), usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST });
        this.device.queue.writeBuffer(buf, 0, data);
        return buf;
    }

    // sprites (the photos) and steps (the periods of the trail): SPRITE_FLOATS
    // each; trail: SEGMENT_FLOATS per segment; lines: LINE_FLOATS per vertex.
    setGeometry({ sprites, steps, trail, lines }) {
        this.spriteBuffer = this._buffer(sprites);
        this.spriteCount = sprites.length / SPRITE_FLOATS;
        this.stepBuffer = this._buffer(steps);
        this.stepCount = steps.length / SPRITE_FLOATS;
        this.trailBuffer = this._buffer(trail);
        this.trailCount = trail.length / SEGMENT_FLOATS;
        this.lineBuffer = this._buffer(lines);
        this.lineCount = lines.length / LINE_FLOATS;
    }

    setView(vp, { width, height, scale, trailWidth, trailAlpha }) {
        const data = new Float32Array(UNIFORM_FLOATS);
        data.set(vp, 0);
        data.set([width, height, scale, trailWidth, trailAlpha], 16);
        this.device.queue.writeBuffer(this.uniforms, 0, data);
    }

    _drawLight(pass) {
        pass.setBindGroup(0, this.bindGroup);
        const layers = [
            [this.linePipeline, this.lineBuffer, this.lineCount, false],
            [this.spritePipeline, this.spriteBuffer, this.spriteCount, true],
            [this.trailPipeline, this.trailBuffer, this.trailCount, true],
            [this.stepPipeline, this.stepBuffer, this.stepCount, true],
        ];
        for (const [pipeline, buffer, count, quads] of layers) {
            if (!count) continue;
            pass.setPipeline(pipeline);
            pass.setVertexBuffer(0, buffer);
            if (quads) pass.draw(6, count);
            else pass.draw(count);
        }
    }
}

/* --- Colour --- */

// An OKLab colour as sRGB 0…1 for the stage. lift raises dark colours so they
// still glow (their position says how dark they are); boost multiplies the
// chroma, for means that are greyer than what they average.
function oklabToStageRGB(l, a, b, { lift = 0.25, boost = 1 } = {}) {
    const L = lift + (1 - lift) * l;
    const A = a * boost, B = b * boost;
    const l_ = (L + 0.3963377774 * A + 0.2158037573 * B) ** 3;
    const m_ = (L - 0.1055613458 * A - 0.0638541728 * B) ** 3;
    const s_ = (L - 0.0894841775 * A - 1.2914855480 * B) ** 3;
    const lin = [
        4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_,
        -1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_,
        -0.0041960863 * l_ - 0.7034186147 * m_ + 1.7076147010 * s_,
    ];
    return lin.map(c => {
        c = Math.min(1, Math.max(0, c));
        return c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055;
    });
}

// A CSS colour (a token's value) as sRGB 0…1.
function cssColourToRGB(css) {
    const ctx = new OffscreenCanvas(1, 1).getContext('2d');
    ctx.fillStyle = css;
    ctx.fillRect(0, 0, 1, 1);
    const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
    return [r / 255, g / 255, b / 255];
}

/* --- Lines --- */

// Collects the faint lines of a stage: axes, rings, ticks.
class StageLines {
    constructor() { this.v = []; }

    line(a, b, alpha) {
        this.v.push(...a, alpha, ...b, alpha);
        return this;
    }

    // A circle around the vertical axis at height y.
    ring(r, y, alpha, steps = 96) {
        for (let i = 0; i < steps; i++) {
            const t0 = i / steps * 2 * Math.PI, t1 = (i + 1) / steps * 2 * Math.PI;
            this.line([r * Math.cos(t0), y, r * Math.sin(t0)], [r * Math.cos(t1), y, r * Math.sin(t1)], alpha);
        }
        return this;
    }

    // The box -1…1 on every axis: its floor and the corner where the axes
    // meet drawn clearer than the rest.
    box() {
        const corners = [[-1, -1], [1, -1], [1, 1], [-1, 1]];
        for (let i = 0; i < 4; i++) {
            const [x0, z0] = corners[i], [x1, z1] = corners[(i + 1) % 4];
            this.line([x0, -1, z0], [x1, -1, z1], 0.3).line([x0, 1, z0], [x1, 1, z1], 0.12).line([x0, -1, z0], [x0, 1, z0], i === 0 ? 0.3 : 0.12);
        }
        return this;
    }

    toArray() { return new Float32Array(this.v); }
}
