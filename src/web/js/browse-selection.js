// SelectionManager — owns image selection state for a BrowsePane

class SelectionManager {
    constructor(onSelectionChange) {
        this.selected = new Set();
        this.lastClickedIndex = -1;
        this._onSelectionChange = onSelectionChange;
    }

    getSelectedFiles() {
        return Array.from(this.selected);
    }

    selectAll(entries, fullPathFn) {
        entries.filter(e => e.type !== 'dir').forEach(e => {
            this.selected.add(fullPathFn(e.name));
        });
        if (this._onSelectionChange) this._onSelectionChange(this.getSelectedFiles());
    }

    clear() {
        this.selected.clear();
        this.lastClickedIndex = -1;
    }

    toggle(fp) {
        if (this.selected.has(fp)) this.selected.delete(fp);
        else this.selected.add(fp);
    }

    handleImageClick(e, idx, fp, entries, fullPathFn) {
        if (e.ctrlKey || e.metaKey) {
            this.toggle(fp);
            this.lastClickedIndex = idx;
        } else if (e.shiftKey && this.lastClickedIndex >= 0) {
            const start = Math.min(this.lastClickedIndex, idx);
            const end = Math.max(this.lastClickedIndex, idx);
            for (let i = start; i <= end; i++) {
                if (entries[i].type === 'image') {
                    this.selected.add(fullPathFn(entries[i].name));
                }
            }
        } else {
            this.selected.clear();
            this.selected.add(fp);
            this.lastClickedIndex = idx;
        }
        if (this._onSelectionChange) this._onSelectionChange(this.getSelectedFiles());
    }

    updateClasses(container) {
        container.querySelectorAll('[data-type="image"]').forEach(el => {
            el.classList.toggle('selected', this.selected.has(el.dataset.path));
        });
        const statusEl = container.querySelector('.status-bar');
        if (!statusEl) return;
        const imageCount = container.querySelectorAll('[data-type="image"]').length;
        const selectedCount = this.selected.size;
        // Clearing the selection belongs to the selection bar, which is where
        // every other action on a selection is; the status bar only counts.
        statusEl.textContent = selectedCount > 0
            ? `${imageCount} images · ${selectedCount} selected`
            : `${imageCount} images`;
    }
}
