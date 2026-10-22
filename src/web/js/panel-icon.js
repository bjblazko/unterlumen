// panel-icon.js — the glyph of a side panel's show and hide button: the
// sidebar's own, mirrored for a panel on the right. Open, it points the way
// the panel goes; closed, the way it comes back.

function panelIconSVG(open) {
    return `<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">`
        + `<rect x="1.5" y="2.5" width="13" height="11"/><path d="M10.5 2.5v11"/>`
        + `<path class="collapse-chevron" d="${open ? 'M5 6l2 2-2 2' : 'M7 6l-2 2 2 2'}"/></svg>`;
}

// panelHideButtonHTML is the framed button in a panel's head that hides it;
// cls names it for its panel, key is its shortcut if it has one.
function panelHideButtonHTML(cls, label, key = '') {
    return `<button type="button" class="btn btn-sm panel-hide-btn ${cls}" aria-label="${label}" title="${label}${key ? ` (${key})` : ''}">${panelIconSVG(true)}</button>`;
}

// The glyphs of the tool strip: what each panel holds.
const PANEL_GLYPHS = {
    filter: '<path d="M2 3h12l-4.5 5.5V13l-3-1.5V8.5z"/>',
    info: '<circle cx="8" cy="8" r="6.5"/><path d="M8 7.5V11"/><circle cx="8" cy="5" r="0.6" fill="currentColor" stroke="none"/>',
    photos: '<rect x="2" y="2" width="5" height="5"/><rect x="9" y="2" width="5" height="5"/><rect x="2" y="9" width="5" height="5"/><rect x="9" y="9" width="5" height="5"/>',
};

// panelToggleButton opens and closes one panel of the right-hand column and
// says whether it is open, with how many where that helps — Filter, Info,
// Photos. It stands in the place's tool strip; the panel's own head names it
// and hides it.
function panelToggleButton(label, cls = label === 'Info' ? 'info-head-toggle' : '') {
    const glyph = PANEL_GLYPHS[label.toLowerCase()] || PANEL_GLYPHS.info;
    const t = document.createElement('template');
    t.innerHTML = `<button type="button" class="panel-toggle ${cls}" aria-pressed="false" aria-expanded="false" data-state="off" title="${label}">`
        + `<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true">${glyph}</svg>`
        + `<span class="panel-toggle-label">${label}</span><span class="lib-filter-btn-count"></span></button>`;
    return t.content.firstElementChild;
}

function setPanelToggle(btn, open) {
    if (!btn) return;
    btn.dataset.state = open ? 'on' : 'off';
    btn.setAttribute('aria-pressed', String(open));
    btn.setAttribute('aria-expanded', String(open));
}

// toolStrip is the right edge of a place, as in IntelliJ: a narrow column of
// panel toggles that stays where it is, whatever is open beside it.
function toolStrip(...buttons) {
    const el = document.createElement('div');
    el.className = 'tool-strip';
    el.setAttribute('role', 'toolbar');
    el.setAttribute('aria-label', 'Panels');
    el.setAttribute('aria-orientation', 'vertical');
    el.append(...buttons.filter(Boolean));
    return el;
}
