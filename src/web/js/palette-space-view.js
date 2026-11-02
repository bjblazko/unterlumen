// palette-space-view.js — the Palette space (Colour): each photo's three
// largest colours as three lights in OKLab, joined into a triangle. Calm,
// close palettes make small triangles; complementary ones — orange against
// teal — span the space. Drawn on a point stage (point-stage.js) in the
// Colour space's geometry, from /api/library/space?view=palette.

function renderPaletteSpace(el, pc, onPick, options) {
    if (!pc?.id?.length) { tlNoData(el, 'No photo has been analysed for its colours yet.'); return; }
    renderPointStage(el, () => paletteSpaceSpec(pc), onPick, options);
}

function paletteSpaceSpec(pc) {
    const n = pc.id.length;
    const light = pointLight(n * 3);
    const sprites = new Float32Array(n * 3 * SPRITE_FLOATS);
    const trail = new Float32Array(n * 3 * SEGMENT_FLOATS);
    for (let i = 0; i < n; i++) {
        const ends = [];
        for (let k = 0; k < 3; k++) {
            const o = (i * 3 + k) * 4;
            const [l, a, b, share] = [pc.colours[o], pc.colours[o + 1], pc.colours[o + 2], pc.colours[o + 3]];
            const pos = colourSpacePosition(l, a, b), rgb = oklabToStageRGB(l, a, b);
            ends.push([...pos, ...rgb]);
            sprites.set([...pos, ...rgb, light.radius * (0.6 + share), light.alpha], (i * 3 + k) * SPRITE_FLOATS);
        }
        for (let k = 0; k < 3; k++) trail.set([...ends[k], ...ends[(k + 1) % 3]], (i * 3 + k) * SEGMENT_FLOATS);
    }
    const sample = pc.total > n ? ' ' + t('A sample of {n} of {total}.', { n: formatCount(n), total: formatCount(pc.total) }) : '';
    return {
        summary: t('{photos}, each as its three largest colours in OKLab, joined into a triangle: lightness up, hue around, colourfulness outward.', { photos: countText(n, 'photo') }) + sample,
        legend: legendDotHTML(t('One of a photo’s three largest colours, larger the more of the frame it covers; the lines join a photo’s three')),
        sprites, steps: new Float32Array(0), trail, trailWidth: 1,
        distance: 4.2,
        lines: colourSpaceLines(),
        labels: [],
        tip: (kind, i) => paletteTip(pc, Math.floor(i / 3)),
        pick: (kind, i) => {
            const p = Math.floor(i / 3);
            return { photo: { id: pc.id[p], lib: pc.libraries[pc.lib[p]], date: pc.date[p] } };
        },
    };
}

function paletteTip(pc, p) {
    const swatches = [0, 1, 2].map(k => {
        const o = (p * 3 + k) * 4;
        const [r, g, b] = oklabToStageRGB(pc.colours[o], pc.colours[o + 1], pc.colours[o + 2], { lift: 0 }).map(v => Math.round(v * 255));
        return `<span class="palette-tip-swatch" style="background:rgb(${r},${g},${b})"></span>`;
    }).join('');
    return photoTipHTML(pc, pc.lib[p], pc.id[p], `
        <span class="point-stage-tip-title">${escapeHtml(pc.date[p] ? pc.date[p].slice(0, 10) : t('No date'))}</span>
        <span class="palette-tip-swatches">${swatches}</span>`);
}
