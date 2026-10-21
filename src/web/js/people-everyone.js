// people-everyone.js — People's first page on the computer: everyone named
// in any library, put together by name (ADR-0051), a field to find them,
// and the libraries where faces wait for a name.
//
// Address: #people.

class PeopleEveryone {
    // pane is the PeoplePane that hosts the page.
    constructor(pane) {
        this.pane = pane;
    }

    async show() {
        const body = this.pane._body;
        const activity = Activity.in(body, 'Reading the people…', { area: true });
        let people, toName;
        try {
            [people, toName] = await Promise.all([PeopleAPI.everyone(), PeopleAPI.toName()]);
        } catch (err) {
            activity.fail(`The people could not be read: ${err.message}.`);
            return;
        }
        this.pane._head(people.length ? countPeople(people.length) : '');
        this.pane._lede(!people.length);
        body.innerHTML = `
            ${toNameHTML(toName)}
            ${people.length ? `
                <div class="people-find-row">
                    <label class="people-label" for="people-find">Find a person</label>
                    <input class="form-input people-find" id="people-find" type="search" autocomplete="off">
                </div>
                <div class="people-grid">${people.map(p => this._card(p)).join('')}</div>
                <p class="people-empty people-none" hidden>Nobody by that name.</p>`
                : `<p class="people-empty">Nobody is named yet. ${toName.some(l => l.groups) ? 'Name the faces waiting above, and the people show here.' : 'The faces found in your libraries wait for a name here once they are looked at.'}</p>`}`;
        wirePeopleFind(body);
    }

    _card(p) {
        const where = this.pane.libraries.length < 2 ? '' : p.libraries.length === 1 ? p.libraries[0].libraryName : `in ${countLibraries(p.libraries.length)}`;
        return `
            <a class="people-card" href="${peopleHash({ view: 'person', id: p.name })}" data-name="${escapeHtml(p.name.toLowerCase())}">
                <img src="${PeopleAPI.faceURL(p.cover.library, p.cover.face)}" alt="" loading="lazy" width="96" height="96">
                <span class="people-card-name">${escapeHtml(p.name)}</span>
                <span class="people-card-count">${countPhotos(p.photos)}</span>
                ${where ? `<span class="people-card-where">${escapeHtml(where)}</span>` : ''}
            </a>`;
    }
}

// toNameHTML is the line of libraries where faces wait for a name, each a
// link to them with how many groups wait; current marks the library shown.
function toNameHTML(toName, current = '') {
    const waiting = toName.filter(l => l.groups || String(l.library) === current);
    if (!waiting.length) return '';
    const links = waiting.map(l => {
        const here = String(l.library) === current;
        return `<a class="people-to-name-link" href="${peopleHash({ view: 'name', library: l.library })}"${here ? ' aria-current="page"' : ''}>${escapeHtml(l.libraryName)} <span class="people-num">${formatCount(l.groups)}</span></a>`;
    });
    return `<nav class="people-to-name" aria-label="Groups of faces waiting for a name"><span class="people-label">Groups waiting for a name</span>${links.join('')}</nav>`;
}

// wirePeopleFind narrows the cards in container to the names holding what
// is typed.
function wirePeopleFind(container) {
    const input = container.querySelector('#people-find');
    if (!input) return;
    const cards = [...container.querySelectorAll('.people-card')];
    input.addEventListener('input', () => {
        const q = input.value.trim().toLowerCase();
        let shown = 0;
        for (const card of cards) {
            const hit = !q || card.dataset.name.includes(q);
            card.hidden = !hit;
            if (hit) shown++;
        }
        container.querySelector('.people-none').hidden = shown > 0;
    });
}

function countPeople(n) { return n === 1 ? '1 person' : `${formatCount(n)} people`; }
function countLibraries(n) { return n === 1 ? '1 library' : `${formatCount(n)} libraries`; }
function countPhotos(n) { return n === 1 ? '1 photo' : `${formatCount(n)} photos`; }
function countFaces(n) { return n === 1 ? '1 face' : `${formatCount(n)} faces`; }

// sentenceList joins items as a sentence does: "a", "a and b", "a, b and c".
function sentenceList(items) {
    return items.length < 2 ? items.join('') : `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;
}

// The people most often in the same photos as someone, with the photos they
// share, each a link to the photos of both in Libraries (the library filter
// with two people).
function oftenWithHTML(withList) {
    if (!withList?.length) return '';
    const links = withList.map(c => `<a class="btn btn-sm people-with-chip" href="#libraries" data-name="${escapeHtml(c.name)}" title="${escapeHtml(countPhotos(c.photos))} together">${withChipHTML(c)}</a>`);
    return `<div class="people-with"><span class="people-label">${OFTEN_WITH_LABEL}</span><div class="people-with-row">${links.join('')}</div></div>`;
}

const OFTEN_WITH_LABEL = 'Often in photos together with · number of photos';

// withChipHTML is what a chip of Often with shows: the person's face, their
// name, and in how many photos they are together.
function withChipHTML(c) {
    const face = c.cover?.face ? `<img class="people-with-face" src="${PeopleAPI.faceURL(c.cover.library, c.cover.face)}" alt="" loading="lazy" width="24" height="24">` : '';
    return `${face}${escapeHtml(c.name)} <span class="people-num">${formatCount(c.photos)}</span>`;
}

function wireOftenWith(container, name) {
    for (const a of container.querySelectorAll('a.people-with-chip')) showPhotosLink(a, { person: [name, a.dataset.name] });
}
