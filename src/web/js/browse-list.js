// ListRenderer — renders the list view for a BrowsePane

class ListRenderer {
    constructor(pane) {
        this._pane = pane;
    }

    renderChunk(start, end) {
        const { entries, focusedIndex, selection, _entryMeta } = this._pane;
        const thumbSize = this._pane._getThumbnailSize();
        const rows = [];

        for (let idx = start; idx < end; idx++) {
            const entry = entries[idx];
            const focusedClass = idx === focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                rows.push(this._renderDirRow(idx, entry, focusedClass));
            } else {
                rows.push(this._renderImageRow(idx, entry, focusedClass, thumbSize, selection, _entryMeta));
            }
        }

        if (start === 0) {
            return `<table class="list-view">
                <thead><tr><th></th><th>Name</th><th>Date</th><th>Size</th></tr></thead>
                <tbody>${rows.join('')}</tbody>
            </table>`;
        }
        return rows.join('');
    }

    _renderDirRow(idx, entry, focusedClass) {
        return `<tr class="dir-row${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="dir">
            <td class="list-icon"><svg width="32" height="26" viewBox="0 0 32 26" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10l2-3h16v22H2z"/></svg></td>
            <td class="list-name">${entry.name}</td>
            <td class="list-date">${formatDate(entry.date)}</td>
            <td class="list-size"></td>
        </tr>`;
    }

    _renderImageRow(idx, entry, focusedClass, thumbSize, selection, entryMeta) {
        const fp = this._pane.fullPath(entry.name);
        const selectedClass = selection.selected.has(fp) ? ' selected' : '';
        const markedClass = App.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
        const size = entry.size ? formatSize(entry.size) : '';
        const badgesHtml = this._pane._buildOverlayBadges(entry.name, entryMeta[entry.name]);
        return `<tr class="image-row${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}">
            <td class="list-icon"><img src="${API.thumbnailURL(fp, thumbSize)}" alt="" loading="lazy"></td>
            <td class="list-name">${entry.name}${badgesHtml ? `<span class="list-badges">${badgesHtml}</span>` : ''}</td>
            <td class="list-date">${formatDate(entry.date)}</td>
            <td class="list-size">${size}</td>
        </tr>`;
    }
}
