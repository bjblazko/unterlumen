// The sentence under a place's title that says what the place is and what
// it does to your files. Every place that has one builds it here, so they
// read alike.

function placeLede(text) {
    return `<p class="place-lede"><span class="place-lede-text">${text} ${placeLink('guide', 'guide', 'How Unterlumen works')}</span></p>`;
}

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
