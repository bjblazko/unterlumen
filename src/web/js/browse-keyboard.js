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
        if (entry.type !== 'image') return;
        const fp = pane.fullPath(entry.name);
        pane.selection.toggle(fp);
        pane.selection.updateClasses(pane.container);
        if (pane.onSelectionChange) pane.onSelectionChange(pane.selection.getSelectedFiles());
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

    getColumnCount() {
        const pane = this._pane;
        if (pane.view === 'list') return 1;
        if (pane.view === 'justified') {
            const focused = pane.container.querySelector(`[data-index="${this.focusedIndex}"]`);
            if (!focused) return 1;
            if (focused.classList.contains('dir-item')) {
                const dirGrid = pane.container.querySelector('.justified-dirs');
                if (!dirGrid) return 1;
                const dirItems = dirGrid.querySelectorAll('.grid-item');
                if (dirItems.length < 2) return 1;
                const firstTop = dirItems[0].getBoundingClientRect().top;
                let cols = 0;
                for (const item of dirItems) {
                    if (item.getBoundingClientRect().top !== firstTop) break;
                    cols++;
                }
                return cols > 0 ? cols : 1;
            }
            const focusedTop = Math.round(focused.getBoundingClientRect().top);
            const items = pane.container.querySelectorAll('.justified-item');
            let cols = 0;
            for (const item of items) {
                if (Math.round(item.getBoundingClientRect().top) === focusedTop) cols++;
            }
            return cols > 0 ? cols : 1;
        }
        const items = pane.container.querySelectorAll('.grid-item');
        if (items.length < 2) return 1;
        const firstTop = items[0].getBoundingClientRect().top;
        let cols = 0;
        for (const item of items) {
            if (item.getBoundingClientRect().top !== firstTop) break;
            cols++;
        }
        return cols > 0 ? cols : 1;
    }
}
