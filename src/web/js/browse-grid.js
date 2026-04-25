// GridRenderer — renders the grid view for a BrowsePane

class GridRenderer {
    constructor(pane) {
        this._pane = pane;
    }

    renderChunk(start, end) {
        const { entries, focusedIndex, selection, showNames, _entryMeta } = this._pane;
        const thumbSize = this._pane._getThumbnailSize();
        const items = [];

        for (let idx = start; idx < end; idx++) {
            const entry = entries[idx];
            const focusedClass = idx === focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                items.push(this._renderDirItem(idx, entry.name, focusedClass));
            } else {
                items.push(this._renderImageItem(idx, entry, focusedClass, thumbSize, selection, showNames, _entryMeta));
            }
        }

        if (start === 0) return `<div class="grid">${items.join('')}</div>`;
        return items.join('');
    }

    _renderDirItem(idx, name, focusedClass) {
        return `<div class="grid-item dir-item${focusedClass}" data-index="${idx}" data-name="${name}" data-type="dir">
            <div class="dir-icon"><svg width="32" height="26" viewBox="0 0 32 26" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10l2-3h16v22H2z"/></svg></div>
            <div class="item-name">${name}</div>
        </div>`;
    }

    _renderImageItem(idx, entry, focusedClass, thumbSize, selection, showNames, entryMeta) {
        const fp = this._pane.fullPath(entry.name);
        const selectedClass = selection.selected.has(fp) ? ' selected' : '';
        const markedClass = this._pane.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
        const label = entry.label ?? entry.name;
        const nameHtml = showNames ? `<div class="item-name">${label}</div>` : '';
        const badgesHtml = this._pane._buildOverlayBadges(entry.name, entryMeta[entry.name]);
        const fallback = this._pane.thumbFallbackURL ? this._pane.thumbFallbackURL(entry, thumbSize) : null;
        const onerror = fallback ? ` onerror="this.onerror=null;this.src='${fallback}'"` : '';
        return `<div class="grid-item image-item${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}">
            <img src="${this._pane.thumbURL(entry, thumbSize)}" alt="${label}" loading="lazy" onload="this.classList.add('img-loaded')"${onerror}>${badgesHtml}${nameHtml}
        </div>`;
    }
}
