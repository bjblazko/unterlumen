// The sentence under a place's title that says what the place is and what
// it does to your files. Every place that has one builds it here, so they
// read alike.

//
// On a phone the sentence would take the room the photos need, so it folds
// behind a round "i" button there; the desk shows it as it is.
function placeLede(text) {
    return `<p class="place-lede">` +
        `<button type="button" class="btn btn-sm place-lede-toggle" aria-expanded="false" aria-label="What this place is">` +
        `<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M8 7v5"/><circle cx="8" cy="4.25" r="0.75" fill="currentColor" stroke="none"/></svg>` +
        `</button>` +
        `<span class="place-lede-text">${text} ${placeLink('guide', 'guide', 'How Unterlumen works')}</span></p>`;
}

// One listener for every lede, since they are built as HTML in many places.
document.addEventListener('click', (e) => {
    const btn = e.target.closest('.place-lede-toggle');
    if (!btn) return;
    const open = btn.getAttribute('aria-expanded') !== 'true';
    btn.setAttribute('aria-expanded', String(open));
    btn.closest('.place-lede').classList.toggle('place-lede-open', open);
});

// A link to another place inside running text. App routes it; a hash link
// alone would not fire popstate.
function placeLink(mode, hash, label) {
    return `<a class="place-link" href="#${hash}" data-mode="${mode}">${label}</a>`;
}

// A link to one library, or to a folder in Folders. App routes both.
function libraryLink(lib) {
    return `<a class="library-link" href="#libraries" data-library-id="${escapeHtml(String(lib.id))}">${escapeHtml(lib.name)}</a>`;
}

function folderLink(relPath, label) {
    return `<a class="folder-link" href="#folders" data-path="${escapeHtml(relPath)}">${label}</a>`;
}
