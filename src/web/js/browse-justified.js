// JustifiedRenderer — renders and layouts the justified view for a BrowsePane

class JustifiedRenderer {
    constructor(pane) {
        this._pane = pane;
        this._relayoutTimer = null;
    }

    renderChunk(start, end) {
        const { entries, focusedIndex, selection, showNames, _entryMeta, _aspectRatios, _justifiedTargetHeight } = this._pane;
        const thumbSize = this._pane._getThumbnailSize();
        const dirItems = [];
        const imageItems = [];

        for (let idx = start; idx < end; idx++) {
            const entry = entries[idx];
            const focusedClass = idx === focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                dirItems.push(this._renderDirItem(idx, entry.name, focusedClass));
            } else {
                imageItems.push(this._renderImageItem(idx, entry, focusedClass, thumbSize, selection, showNames, _entryMeta, _aspectRatios, _justifiedTargetHeight));
            }
        }

        if (start === 0) {
            const parts = [];
            if (dirItems.length > 0) parts.push(`<div class="grid justified-dirs">${dirItems.join('')}</div>`);
            parts.push(`<div class="justified">${imageItems.join('')}</div>`);
            return parts.join('');
        }
        return imageItems.join('');
    }

    _renderDirItem(idx, name, focusedClass) {
        return `<div class="grid-item dir-item${focusedClass}" data-index="${idx}" data-name="${name}" data-type="dir">
            <div class="dir-icon"><svg width="32" height="26" viewBox="0 0 32 26" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10l2-3h16v22H2z"/></svg></div>
            <div class="item-name">${name}</div>
        </div>`;
    }

    _renderImageItem(idx, entry, focusedClass, thumbSize, selection, showNames, entryMeta, aspectRatios, targetHeight) {
        const fp = this._pane.fullPath(entry.name);
        const selectedClass = selection.selected.has(fp) ? ' selected' : '';
        const markedClass = App.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
        const nameHtml = showNames ? `<div class="item-name">${entry.name}</div>` : '';
        const badgesHtml = this._pane._buildOverlayBadges(entry.name, entryMeta[entry.name]);
        const ar = aspectRatios[idx] || 1.5;
        return `<div class="justified-item image-item${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}" style="width:${Math.round(targetHeight * ar)}px;height:${targetHeight}px">
            <img src="${API.thumbnailURL(fp, thumbSize)}" alt="${entry.name}" loading="lazy" data-jidx="${idx}">${badgesHtml}${nameHtml}
        </div>`;
    }

    layout() {
        const pane = this._pane;
        const container = pane.container.querySelector('.justified');
        if (!container) return;
        const containerWidth = container.clientWidth;
        if (containerWidth <= 0) return;

        const items = container.querySelectorAll('.justified-item');
        if (items.length === 0) return;

        const gap = 1;
        let rowStart = 0;
        let rowAspectSum = 0;

        for (let i = 0; i <= items.length; i++) {
            if (i < items.length) {
                const idx = parseInt(items[i].dataset.index);
                const ar = pane._aspectRatios[idx] || 1.5;
                const rowGaps = (i - rowStart) * gap;
                if (rowAspectSum > 0 && rowAspectSum * pane._justifiedTargetHeight + ar * pane._justifiedTargetHeight + rowGaps + gap > containerWidth) {
                    this._setRow(items, rowStart, i, containerWidth, rowAspectSum, gap);
                    rowStart = i;
                    rowAspectSum = ar;
                } else {
                    rowAspectSum += ar;
                }
            } else {
                for (let j = rowStart; j < i; j++) {
                    const jIdx = parseInt(items[j].dataset.index);
                    const ar = pane._aspectRatios[jIdx] || 1.5;
                    items[j].style.width = Math.round(pane._justifiedTargetHeight * ar) + 'px';
                    items[j].style.height = pane._justifiedTargetHeight + 'px';
                }
            }
        }
    }

    _setRow(items, start, end, containerWidth, aspectSum, gap) {
        const gaps = (end - start - 1) * gap;
        const rowHeight = (containerWidth - gaps) / aspectSum;
        let usedWidth = 0;

        for (let i = start; i < end; i++) {
            const idx = parseInt(items[i].dataset.index);
            const ar = this._pane._aspectRatios[idx] || 1.5;
            if (i === end - 1) {
                const w = containerWidth - usedWidth - (end - start - 1) * gap;
                items[i].style.width = Math.round(w) + 'px';
            } else {
                const w = Math.round(ar * rowHeight);
                items[i].style.width = w + 'px';
                usedWidth += w;
            }
            items[i].style.height = Math.round(rowHeight) + 'px';
        }
    }

    scheduleRelayout() {
        if (this._relayoutTimer) return;
        this._relayoutTimer = requestAnimationFrame(() => {
            this._relayoutTimer = null;
            this.layout();
        });
    }
}
