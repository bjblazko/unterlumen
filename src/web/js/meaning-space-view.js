// meaning-space-view.js — the Meaning space (Things): every photo the vision
// model has looked at, placed by what it shows. Its vector of hundreds of
// numbers is reduced to the three directions in which the photos differ
// most (principal components, library/pca.go): dogs gather near dogs,
// beaches near beaches, without any keyword. Each photo in its main colour.
// From /api/library/space?view=meaning, on a point stage.

function renderMeaningSpace(el, pc, onPick, options) {
    if (!pc?.id?.length) { tlNoData(el, 'No photo has been looked at for things yet.'); return; }
    renderPointStage(el, () => meaningSpaceSpec(pc), onPick, options);
}

function meaningSpaceSpec(pc) {
    const n = pc.id.length;
    const light = pointLight(n);
    const sprites = new Float32Array(n * SPRITE_FLOATS);
    for (let i = 0; i < n; i++) {
        const o = i * 4;
        sprites.set([pc.xyz[i * 3], pc.xyz[i * 3 + 1], pc.xyz[i * 3 + 2],
            ...oklabToStageRGB(pc.colours[o], pc.colours[o + 1], pc.colours[o + 2]), light.radius, light.alpha], i * SPRITE_FLOATS);
    }
    return {
        summary: t('{photos} placed by what they show: the three directions in which what the vision model sees in them differs most. Photos close together show similar things. Each is in its main colour.', { photos: countText(n, 'photo') }),
        legend: legendDotHTML(t('One photo, near those that show something alike')),
        sprites, steps: new Float32Array(0), trail: new Float32Array(0),
        distance: 4.6,
        lines: new StageLines().box().toArray(),
        labels: [],
        tip: (kind, i) => photoTipHTML(pc, pc.lib[i], pc.id[i], `<span class="point-stage-tip-title">${escapeHtml(pc.date[i] ? pc.date[i].slice(0, 10) : t('No date'))}</span>`),
        pick: (kind, i) => ({ photo: { id: pc.id[i], lib: pc.libraries[pc.lib[i]], date: pc.date[i] } }),
    };
}
