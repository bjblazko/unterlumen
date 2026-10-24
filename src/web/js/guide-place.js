// guide-place.js — "How Unterlumen works": the model in one picture and a
// few paragraphs. A place, not a dialog (ADR-0033): you read it, and every
// place's sentence links here.

// The diagram's grid, in SVG user units: three columns, four rows.
const GUIDE_COL = [0, 124, 248];
const GUIDE_COL_W = 112;
const GUIDE_ROW = [0, 72, 156, 240];
const GUIDE_BOX_H = 56;

// One box. With a place it is a link there; the root and Map · Timeline are not.
function guideNode({ x, y, w = GUIDE_COL_W, h = GUIDE_BOX_H, title, sub, place }) {
    const box = `<rect class="guide-box" x="${x + 0.5}" y="${y + 0.5}" width="${w - 1}" height="${h - 1}" rx="4"/>
        <text class="guide-box-title" x="${x + 12}" y="${y + h / 2 - 5}">${title}</text>
        <text class="guide-box-sub" x="${x + 12}" y="${y + h / 2 + 13}">${sub}</text>`;
    if (!place) return `<g class="guide-node">${box}</g>`;
    const [mode, hash] = place;
    return `<a class="guide-node place-link" href="#${hash}" data-mode="${mode}">${box}</a>`;
}

// A connector straight down, ending in an arrowhead.
function guideEdge(x, from, to) {
    return `<path class="guide-edge" d="M${x + 0.5} ${from}V${to - 5}"/>
        <path class="guide-arrow" d="M${x - 3.5} ${to - 6}L${x + 0.5} ${to}L${x + 4.5} ${to - 6}Z"/>`;
}

function guideDiagram() {
    const [c1, c2, c3] = GUIDE_COL, [r1, r2, r3, r4] = GUIDE_ROW, h = GUIDE_BOX_H;
    const mid = c => c + GUIDE_COL_W / 2;
    const libW = c3 + GUIDE_COL_W - c2;
    return `<svg class="guide-diagram" viewBox="0 0 360 ${r4 + h}" role="img"
                aria-label="${t('Your folders are shown as they are in Folders, and cataloged by libraries. Map and Timeline read all libraries. Galleries are collected from libraries and go to destinations.')}">
        ${guideNode({ x: c1, y: r1, w: 360, h: 44, title: t('Your folders'), sub: t('On this computer, a card, a NAS') })}
        ${guideEdge(mid(c1), r1 + 44, r2)}
        ${guideEdge(c2 + libW / 2, r1 + 44, r2)}
        ${guideNode({ x: c1, y: r2, title: t('Folders'), sub: t('Files as on disk'), place: ['browse', 'folders'] })}
        ${guideNode({ x: c2, y: r2, w: libW, title: t('Libraries'), sub: t('A folder and all folders in it'), place: ['library', 'libraries'] })}
        ${guideEdge(mid(c2), r2 + h, r3)}
        ${guideEdge(mid(c3), r2 + h, r3)}
        ${guideNode({ x: c2, y: r3, title: t('Map · Timeline'), sub: t('All libraries') })}
        ${guideNode({ x: c3, y: r3, title: t('Galleries'), sub: t('Photos to publish'), place: ['published', 'galleries'] })}
        ${guideEdge(mid(c3), r3 + h, r4)}
        ${guideNode({ x: c3, y: r4, title: t('Destinations'), sub: t('Where they go'), place: ['destinations', 'destinations'] })}
    </svg>`;
}

function guideText() {
    return `
        <section class="guide-section">
            <h2>${t('Your folders')}</h2>
            <p>${t('Unterlumen works on folders where they are — on this computer, on a camera card, on a NAS. Nothing is imported or copied. On a server it sees the folder the container is given.')}</p>
        </section>
        <section class="guide-section">
            <h2>${t('Folders, Marked for deletion, Organize')}</h2>
            <p>${t('{folders} shows any folder as it is on disk, folder by folder. This is where you cull: mark what should go, and it waits in {marked} until you delete it there — nothing leaves the disk before. {organize} moves the photos of one folder into others with a key each. Folders needs nothing prepared; it works on any folder at once.', {
                folders: placeLink('browse', 'folders', t('Folders')),
                marked: placeLink('wastebin', 'marked', t('Marked for deletion')),
                organize: placeLink('organize', 'organize', t('Organize')),
            })}</p>
        </section>
        <section class="guide-section">
            <h2>${t('Libraries')}</h2>
            <p>${t('A {library} catalogs a folder together with every folder inside it, however deep. Keep your photos in subfolders as you like; a library does not flatten them, and you browse it folder by folder as in Folders. For example:', { library: placeLink('library', 'libraries', t('library')) })}</p>
            <pre class="guide-tree" aria-label="${t('Two libraries in two places: Projects on this computer and Travel on a NAS, each with subfolders')}">${t('Pictures/Projects/')}          <span class="guide-tree-note">one library, on this computer</span>
  2024 Wedding Anna and Ben/
  Portraits studio/
  Street Berlin/
    Day 1/
    Day 2/
nas/Travel/                 <span class="guide-tree-note">another library, on a NAS</span>
  2023 Iceland/
  2025 Lisbon/</pre>
            <p>${t('Each library is its own folder, wherever it is; libraries need no folder in common. New library… asks for that folder and nothing else.')}</p>
            <p>${t('Search and filter in Projects then cover every project in it at once. A library reads each photo once and keeps what it found — camera, lens, date, place — so you can search and filter across thousands of photos in an instant. The photos stay where they are; the catalog and its thumbnails live in Unterlumen’s own data folder. Beside a photo it writes only an XMP sidecar ({file}), when you give the photo a title or a field or publish it, so that record travels with the photo and every installation that sees the photo reads it; the catalog only copies it. Make a library of a folder you want to search, map or publish from. Folders and libraries show the same files, so a photo you mark or move in one is gone from the other too.', { file: '<span class="mono">photo.xmp</span>' })}</p>
        </section>
        <section class="guide-section">
            <h2>${t('Map and Timeline')}</h2>
            <p>${t('{map} and {timeline} read every library at once: every photo with a location on one map, every dated photo on one time axis. A photo in two libraries is shown once. A folder no library catalogs does not appear here.', { map: placeLink('map', 'map', t('Map')), timeline: placeLink('timeline', 'timeline', t('Timeline')) })}</p>
        </section>
        <section class="guide-section">
            <h2>${t('Galleries and destinations')}</h2>
            <p>${t('A {gallery} is a set of photos you collect in a library with Add to gallery…. Nothing is made yet; you can add to it over days. Publish then makes the files and sends them to the gallery’s {destination}: a website with an index of its albums, share links for family and friends, or plain image files you post yourself. A destination also says how the files are made — size, format, which metadata stays.', { gallery: placeLink('published', 'galleries', t('gallery')), destination: placeLink('destinations', 'destinations', t('destination')) })}</p>
        </section>
        <section class="guide-section">
            <h2>${t('Two installations')}</h2>
            <p>${t('Unterlumen can run on a NAS and on a desk computer at once, on the same photos. A library you share (Edit library…) is the same library on both: the other installation adds its folder and gets its name, and each one keeps its own index. Libraries you do not share stay on one installation. Destinations and galleries are shared through one shared folder, chosen in {settings}.', { settings: placeLink('settings', 'settings', t('Settings')) })}</p>
        </section>
        <section class="guide-section">
            <h2>${t('What Unterlumen does not do')}</h2>
            <p>${t('It does not edit pictures: no colour, no retouching. It does not read RAW files. It moves or deletes a file only when you say so, and it does not upload anything unless a destination is set up to.')}</p>
        </section>
        <section class="guide-section">
            <h2>${t('Words in the configuration files')}</h2>
            <p>${t('The files on disk use older names for two of these things.')}</p>
            <table class="guide-words">
                <thead><tr><th>${t('In the app')}</th><th>${t('In the files')}</th></tr></thead>
                <tbody>
                    <tr><td>${t('Destination')}</td><td>${t('{word}, in {file}', { word: '<span class="mono">channel</span>', file: '<span class="mono">channels.json</span>' })}</td></tr>
                    <tr><td>${t('Gallery')}</td><td>${t('{word}; one not yet published is a draft in {file}', { word: '<span class="mono">album</span>', file: '<span class="mono">drafts.json</span>' })}</td></tr>
                </tbody>
            </table>
        </section>`;
}

class GuidePane {
    constructor(container) {
        this.container = container;
    }

    render() {
        if (this.container.firstChild) return; // nothing on it changes
        this.container.innerHTML = `
            <div class="guide-pane">
                ${placeHeaderHTML({ title: t('How Unterlumen works') })}
                <div class="guide-body">
                    ${guideDiagram()}
                    <div class="guide-text">${guideText()}</div>
                </div>
            </div>`;
    }
}
