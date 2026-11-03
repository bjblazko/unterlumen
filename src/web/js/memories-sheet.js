// memories-sheet.js — Memories as one sheet (ADR-0064): each memory a block
// of its photos, the blocks side by side in bands. A block is a collage
// without gaps, made from a random tree of splits — side by side or one
// above the other — in which each photo takes a format it can carry: a
// poster, a panorama, a square, its own shape. The tree is chosen so the
// block has a shape worth showing; the bands give their blocks the width
// their shapes ask for, so a band is as high everywhere. Chance decides the
// trees, the formats and the order; a seed makes it repeatable.

const SHEET_GAP = 4;

// sheetRandom is a small seeded generator (mulberry32), [0, 1).
function sheetRandom(seed) {
    let a = seed >>> 0;
    return () => {
        a = (a + 0x6d2b79f5) >>> 0;
        let t = a;
        t = Math.imul(t ^ (t >>> 15), t | 1);
        t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}

// sheetFormats are the shapes a photo of ratio r can take without losing
// what it shows: its own, and a poster, square, wide or panorama cut where
// the photo is near enough to it.
function sheetFormats(r) {
    const out = [r];
    if (r < 0.95) out.push(2 / 3);
    if (r > 0.75 && r < 1.6) out.push(1);
    if (r >= 1.3) out.push(16 / 9);
    if (r >= 1.45) out.push(21 / 9);
    return out;
}

function sheetPick(list, rnd) {
    return list[Math.floor(rnd() * list.length)];
}

// sheetTree splits photos [from, to) into a random tree: a leaf per photo
// with a format, and splits 'row' (side by side) or 'col' (one above the
// other). The order of the photos is kept.
function sheetTree(photos, from, to, rnd) {
    if (to - from === 1) {
        const ratio = sheetPick(sheetFormats(photos[from].ratio || 1.5), rnd);
        return { leaf: from, ratio };
    }
    const at = from + 1 + Math.floor(rnd() * (to - from - 1));
    const a = sheetTree(photos, from, at, rnd);
    const b = sheetTree(photos, at, to, rnd);
    const dir = rnd() < 0.5 ? 'row' : 'col';
    const ratio = dir === 'row' ? a.ratio + b.ratio : 1 / (1 / a.ratio + 1 / b.ratio);
    return { dir, a, b, ratio };
}

// sheetPlace lays a tree into the rectangle (x, y, w, h): each leaf gets
// its rectangle in out[leaf].
function sheetPlace(node, x, y, w, h, out) {
    if (node.leaf !== undefined) { out[node.leaf] = { x, y, w, h }; return out; }
    if (node.dir === 'row') {
        const wa = h * node.a.ratio;
        sheetPlace(node.a, x, y, wa, h, out);
        sheetPlace(node.b, x + wa, y, w - wa, h, out);
    } else {
        const ha = w / node.a.ratio;
        sheetPlace(node.a, x, y, w, ha, out);
        sheetPlace(node.b, x, y + ha, w, h - ha, out);
    }
    return out;
}

// sheetBlock chooses the tree of a block whose shape comes nearest the
// target ratio, among some tries, and none with a photo too small to see.
function sheetBlock(photos, target, rnd) {
    let best = null;
    for (let i = 0; i < 60; i++) {
        const tree = sheetTree(photos, 0, photos.length, rnd);
        const rects = sheetPlace(tree, 0, 0, tree.ratio, 1, []);
        const smallest = Math.min(...rects.map(r => r.w * r.h)) / tree.ratio;
        if (photos.length > 1 && smallest < 0.3 / photos.length) continue;
        const miss = Math.abs(Math.log(tree.ratio / target));
        if (!best || miss < best.miss) best = { tree, miss };
    }
    return best ? best.tree : sheetTree(photos, 0, photos.length, rnd);
}

// sheetTarget is the shape a block aims for: a lone photo keeps its format;
// more photos aim anywhere from upright to wide — the sheet's variety.
function sheetTarget(count, phone, rnd) {
    if (count === 1) return null;
    const [lo, hi] = phone ? [0.75, 1.6] : [0.7, 2.6];
    return lo * Math.pow(hi / lo, rnd());
}

// sheetLimits are the heights a band may have in the width.
function sheetLimits(width, phone) {
    return phone ? { minH: 0, maxH: width * 1.35 } : { minH: 220, maxH: Math.min(620, width * 0.62) };
}

// sheetBands groups the blocks into bands of one height: one per band on a
// phone, otherwise as many as keep the band no higher than maxH and no
// lower than minH, at most three.
function sheetBands(blocks, width, phone, rnd) {
    const { minH, maxH } = sheetLimits(width, phone);
    const bands = [];
    let band = [];
    let ratio = 0;
    for (const b of blocks) {
        const full = band.length && (phone || band.length >= 3 || width / (ratio + b.ratio) < minH
            || (width / ratio <= maxH && rnd() < 0.35));
        if (full) { bands.push(band); band = []; ratio = 0; }
        band.push(b);
        ratio += b.ratio;
    }
    if (band.length) bands.push(band);
    return bands;
}

// sheetWiden builds a block that stands alone in its band again, wide
// enough to fill it: its photos as a wide collage, a lone photo in its
// widest format. On a phone a block stands alone anyway.
function sheetWiden(block, width, rnd) {
    const photos = block.memory.photos;
    const target = width / 380;
    const tree = photos.length > 1
        ? sheetBlock(photos, target, rnd)
        : { leaf: 0, ratio: Math.max(...sheetFormats(photos[0].ratio || 1.5)) };
    return { ...block, tree, ratio: tree.ratio };
}

// sheetSize gives each block of a band its pixel size: one height, widths
// as the blocks' shapes ask, no higher than the band may be.
function sheetSize(band, width, maxH) {
    const sum = band.reduce((n, b) => n + b.ratio, 0);
    const h = Math.min(maxH, (width - SHEET_GAP * (band.length - 1)) / sum);
    return band.map(b => ({ ...b, w: Math.floor(h * b.ratio), h: Math.floor(h) }));
}

// sheetLayout lays out memories ([{ photos, ... }]) in the width: their
// order shuffled, a tree for each, the bands. Each block gets its tiles as
// pixel rectangles.
function sheetLayout(memories, width, { phone = false, seed = 1, shuffle = true } = {}) {
    const rnd = sheetRandom(seed);
    const order = memories.slice();
    if (shuffle) for (let i = order.length - 1; i > 0; i--) { const j = Math.floor(rnd() * (i + 1)); [order[i], order[j]] = [order[j], order[i]]; }
    const blocks = order.filter(m => m.photos.length).map(m => {
        const target = sheetTarget(m.photos.length, phone, rnd);
        const tree = target === null ? sheetTree(m.photos, 0, 1, rnd) : sheetBlock(m.photos, target, rnd);
        return { memory: m, tree, ratio: tree.ratio };
    });
    const { maxH } = sheetLimits(width, phone);
    return sheetBands(blocks, width, phone, rnd).map(band => {
        let sized = sheetSize(band, width, maxH);
        if (!phone && sized.length === 1 && sized[0].w < width * 0.85) sized = sheetSize([sheetWiden(band[0], width, rnd)], width, maxH);
        return sized.map(b => ({ ...b, tiles: sheetPlace(b.tree, 0, 0, b.w, b.h, []) }));
    });
}

// sheetBlockHTML draws one block: its tiles, each a button that opens the
// block's photos at that one, inset by half the gap.
function sheetBlockHTML(block, index) {
    const g = SHEET_GAP / 2;
    const tiles = block.tiles.map((r, i) => {
        const p = block.memory.photos[i];
        return `<button type="button" class="sheet-tile" data-block="${index}" data-index="${i}" aria-label="${escapeHtml(p.name)}"
            style="left:${r.x}px;top:${r.y}px;width:${r.w}px;height:${r.h}px;padding:${g}px">
            <img src="${LibraryAPI.thumbURL(p.lib, p.id)}" alt="" loading="lazy" decoding="async"></button>`;
    }).join('');
    return `<div class="sheet-collage" style="width:${block.w}px;height:${block.h}px">${tiles}</div>`;
}
