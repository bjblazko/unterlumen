// BrowseKeyboard — owns focus and navigation state for a BrowsePane

class BrowseKeyboard {
    constructor(pane) {
        this._pane = pane;
        this.focusedIndex = -1;
    }

    moveFocus(delta) {
        const count = this._pane.entries.length;
        if (count === 0) return;
        let next = this.focusedIndex + delta;
        if (next < 0) next = 0;
        if (next >= count) next = count - 1;
        this.focusedIndex = next;
        this._pane._ensureRenderedUpTo(next);
        this.updateFocusClass();
        this.scrollFocusedIntoView();
        this._pane._notifyFocusChange();
    }

    activateFocused() {
        const pane = this._pane;
        if (this.focusedIndex < 0 || this.focusedIndex >= pane.entries.length) return;
        const entry = pane.entries[this.focusedIndex];
        const path = pane.fullPath(entry.name);
        if (entry.type === 'dir') {
            pane.load(path);
            if (pane.onNavigate) pane.onNavigate(path);
        } else {
            if (pane.onImageClick) pane.onImageClick(path);
        }
    }

    toggleFocusedSelection() {
        const pane = this._pane;
        if (this.focusedIndex < 0 || this.focusedIndex >= pane.entries.length) return;
        const entry = pane.entries[this.focusedIndex];
        const fp = pane.fullPath(entry.name);
        if (entry.type === 'dir') {
            if (pane.selectedDirs.has(fp)) {
                pane.selectedDirs.delete(fp);
            } else {
                // Mutual exclusion: clear photo selection when adding a dir
                if (pane.selection.selected.size > 0) {
                    pane.selection.clear();
                    pane.selection.updateClasses(pane.container);
                }
                pane.selectedDirs.add(fp);
            }
            pane._updateDirSelectionClasses();
            if (pane.onSelectionChange) pane.onSelectionChange([]);
        } else {
            // Clear dir selection when toggling a photo
            if (pane.selectedDirs.size > 0) {
                pane.selectedDirs.clear();
                pane._updateDirSelectionClasses();
            }
            pane.selection.toggle(fp);
            pane.selection.updateClasses(pane.container);
            if (pane.onSelectionChange) pane.onSelectionChange(pane.selection.getSelectedFiles());
        }
    }

    getFocusedEntry() {
        const pane = this._pane;
        if (this.focusedIndex < 0 || this.focusedIndex >= pane.entries.length) return null;
        return pane.fullPath(pane.entries[this.focusedIndex].name);
    }

    getFocusedDir() {
        const pane = this._pane;
        if (this.focusedIndex < 0 || this.focusedIndex >= pane.entries.length) return null;
        const entry = pane.entries[this.focusedIndex];
        if (entry.type !== 'dir') return null;
        return pane.fullPath(entry.name);
    }

    getFocusedFile() {
        const pane = this._pane;
        if (this.focusedIndex < 0 || this.focusedIndex >= pane.entries.length) return null;
        const entry = pane.entries[this.focusedIndex];
        if (entry.type !== 'image') return null;
        return pane.fullPath(entry.name);
    }

    updateFocusClass() {
        this._pane.container.querySelectorAll('[data-index]').forEach(el => {
            el.classList.toggle('focused', parseInt(el.dataset.index) === this.focusedIndex);
        });
    }

    scrollFocusedIntoView() {
        const el = this._pane.container.querySelector(`[data-index="${this.focusedIndex}"]`);
        if (el) el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }

    // moveFocusVertically goes to the tile in the row above or below whose
    // middle is nearest the focused one's. Folders, the grid and the justified
    // rows differ in length, so no count of columns can say which tile that
    // is; the tiles' places on screen can.
    moveFocusVertically(dir) {
        const pane = this._pane;
        const current = pane.container.querySelector(`[data-index="${this.focusedIndex}"]`);
        if (!current) { this.moveFocus(dir); return; }
        if (dir > 0) pane._ensureRenderedUpTo(Math.min(this.focusedIndex + CHUNK_SIZE, pane.entries.length - 1));
        const target = this._nearestInNextRow(current.getBoundingClientRect(), dir);
        if (target !== null) this.moveFocus(target - this.focusedIndex);
    }

    // _nearestInNextRow is the index of the tile in the nearest row beyond
    // from, in direction dir, closest to its middle; null at the edge.
    _nearestInNextRow(from, dir) {
        const tiles = [...this._pane.container.querySelectorAll('[data-index]')]
            .filter(el => el.offsetParent)
            .map(el => ({ index: parseInt(el.dataset.index, 10), r: el.getBoundingClientRect() }))
            .filter(({ r }) => (dir > 0 ? r.top >= from.bottom - 1 : r.bottom <= from.top + 1));
        if (!tiles.length) return null;
        const edge = t => (dir > 0 ? t.r.top : -t.r.bottom);
        const rowEdge = Math.min(...tiles.map(edge));
        const mid = r => r.left + r.width / 2;
        const row = tiles.filter(t => edge(t) - rowEdge < 4);
        row.sort((a, b) => Math.abs(mid(a.r) - mid(from)) - Math.abs(mid(b.r) - mid(from)));
        return row[0].index;
    }
}
