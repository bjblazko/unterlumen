// GridRenderer — renders the grid view for a BrowsePane

class GridRenderer {
    constructor(pane) {
        this._pane = pane;
    }

    renderChunk(start, end) {
        const { entries, focusedIndex, selection, showNames, _entryMeta } = this._pane;
        const thumbSize = this._pane._getThumbnailSize();
        const items = [];
        const dirItems = [];

        for (let idx = start; idx < end; idx++) {
            const entry = entries[idx];
            const focusedClass = idx === focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                dirItems.push(this._renderDirItem(idx, entry.name, focusedClass));
            } else {
                items.push(this._renderImageItem(idx, entry, focusedClass, thumbSize, selection, showNames, _entryMeta));
            }
        }

        if (start === 0) {
            const parts = [];
            if (dirItems.length > 0) parts.push(`<div class="folder-row">${dirItems.join('')}</div>`);
            parts.push(`<div class="grid">${items.join('')}</div>`);
            return parts.join('');
        }
        return items.join('');
    }

    // A folder is a place you go to, not a photo you look at: a row of chips
    // above the photos rather than large empty tiles among them.
    _renderDirItem(idx, name, focusedClass) {
        return `<button class="folder-chip dir-item${focusedClass}" data-index="${idx}" data-name="${escapeHtml(name)}" data-type="dir">
            <svg width="18" height="14" viewBox="0 0 18 14" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round" aria-hidden="true"><path d="M1 2.5v10h16v-8.5H8L6.5 2.5H1z"/></svg>
            <span class="item-name">${escapeHtml(name)}</span>
        </button>`;
    }

    _renderImageItem(idx, entry, focusedClass, thumbSize, selection, showNames, entryMeta) {
        const fp = this._pane.fullPath(entry.name);
        const selectedClass = selection.selected.has(fp) ? ' selected' : '';
        const markedClass = this._pane.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
        const label = entry.label ?? entry.name;
        const nameHtml = showNames ? `<div class="item-name">${escapeHtml(label)}</div>` : '';
        const badgesHtml = this._pane._buildOverlayBadges(entry.name, entryMeta[entry.name]);
        const fallback = this._pane.thumbFallbackURL ? this._pane.thumbFallbackURL(entry, thumbSize) : null;
        const onerror = fallback ? ` onerror="this.onerror=null;this.src='${fallback}'"` : '';
        return `<div class="grid-item image-item${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${escapeHtml(entry.name)}" data-type="image" data-path="${escapeHtml(fp)}">
            <img src="${this._pane.thumbURL(entry, thumbSize)}" alt="${escapeHtml(label)}" loading="lazy" decoding="async" fetchpriority="low" onload="this.classList.add('img-loaded')"${onerror}>${badgesHtml}${nameHtml}
        </div>`;
    }
}
