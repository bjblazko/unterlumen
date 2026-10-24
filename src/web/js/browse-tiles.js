// browse-tiles.js — what a browse pane draws on and beside its tiles: a
// folder's tile, the details over a photo, and the order the tiles come in.

const FOLDER_SIGN_SVG = '<svg width="40" height="32" viewBox="0 0 18 14" fill="none" stroke="currentColor" stroke-width="1" stroke-linejoin="round"><path d="M1 2.5v10h16v-8.5H8L6.5 2.5H1z"/></svg>';

// A folder is a tile in the row above the photos, the same tile a library
// draws. A folder a library covers shows what the library knows of it —
// four photos, how many and the years — once that has come; a folder no
// library covers shows the folder sign.
function folderTileHTML(idx, name, classes, body) {
    return `<button class="folder-tile dir-item${classes}" data-index="${idx}" data-name="${escapeHtml(name)}" data-type="dir">
            <span class="folder-tile-body">${body}</span>
        </button>`;
}

function folderSignHTML(name) {
    return `
                <span class="folder-tile-icon" aria-hidden="true">${FOLDER_SIGN_SVG}</span>
                <span class="item-name">${escapeHtml(name)}</span>`;
}

function folderTileBodyHTML(libID, name, preview) {
    const ids = preview?.photoIds || [];
    const cells = [0, 1, 2, 3].map(i => ids[i]
        ? `<img src="${LibraryAPI.thumbURL(libID, ids[i])}" alt="" loading="lazy">`
        : '<span></span>').join('');
    return `<span class="folder-tile-mosaic${ids.length === 1 ? ' folder-tile-mosaic--one' : ''}" aria-hidden="true">${cells}</span>
            <span class="item-name">${escapeHtml(name)}</span>
            <span class="folder-tile-meta">${preview ? folderTileMeta(preview) : '&nbsp;'}</span>`;
}

// All overlay chips look the same: a dark chip with mono text over the
// photo. Colour is a signal in this UI (ADR-0030), and a colour per file
// format and per film simulation would be eight decorative colours
// competing with the photograph itself.
function overlayBadgesHTML(name, meta) {
    const badges = [];
    if (meta?.hasGPS) {
        badges.push(`<span class="overlay-badge overlay-badge-gps" title="${t('Has GPS coordinates')}"><svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5a2.5 2.5 0 1 1 0-5 2.5 2.5 0 0 1 0 5z"/></svg></span>`);
    }
    const fileType = fileTypeLabel(name);
    if (fileType) badges.push(`<span class="overlay-badge">${fileType}</span>`);
    if (meta?.filmSimulation) badges.push(`<span class="overlay-badge">${escapeHtml(meta.filmSimulation)}</span>`);
    if (meta?.aspectRatio) {
        const icon = aspectRatioIcon(meta.aspectRatio, 'rgba(255,255,255,0.9)');
        badges.push(`<span class="overlay-badge overlay-badge-ratio">${icon}${escapeHtml(meta.aspectRatio)}</span>`);
    }
    return badges.length ? `<div class="overlay-badges">${badges.join('')}</div>` : '';
}

function fileTypeLabel(name) {
    const labels = {
        jpg: 'JPEG', jpeg: 'JPEG',
        heif: 'HEIF', heic: 'HEIF', hif: 'HEIF',
        png: 'PNG', gif: 'GIF', webp: 'WebP',
    };
    return labels[name.split('.').pop().toLowerCase()] || null;
}

// updateOverlayBadges puts each photo's details in place once they have come:
// over a tile, or beside a list row's name.
function updateOverlayBadges(container, badgesOf) {
    container.querySelectorAll('[data-type="image"]').forEach(el => {
        const badges = badgesOf(el.dataset.name);
        if (el.tagName !== 'TR') {
            el.querySelector('.overlay-badges')?.remove();
            if (badges) el.insertAdjacentHTML('beforeend', badges);
            return;
        }
        const nameCell = el.querySelector('td.list-name');
        if (!nameCell) return;
        let listBadges = nameCell.querySelector('.list-badges');
        if (!badges) { listBadges?.remove(); return; }
        if (!listBadges) {
            listBadges = document.createElement('span');
            listBadges.className = 'list-badges';
            nameCell.appendChild(listBadges);
        }
        listBadges.innerHTML = badges;
    });
}

// compareBrowseEntries orders folders first, then by the field chosen; a
// photo without a date taken goes last whichever way.
function compareBrowseEntries(sort, order) {
    const less = {
        date: (a, b) => new Date(a.date) < new Date(b.date),
        size: (a, b) => (a.size || 0) < (b.size || 0),
        name: (a, b) => a.name.toLowerCase() < b.name.toLowerCase(),
    };
    return (a, b) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        if (sort === 'taken' && !(a.exifDate && b.exifDate)) {
            if (!a.exifDate && !b.exifDate) return 0;
            return a.exifDate ? -1 : 1;
        }
        const isLess = sort === 'taken' ? new Date(a.exifDate) < new Date(b.exifDate) : (less[sort] || less.name)(a, b);
        return order === 'desc' ? (isLess ? 1 : -1) : (isLess ? -1 : 1);
    };
}
