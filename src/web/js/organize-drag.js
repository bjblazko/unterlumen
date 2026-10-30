// organize-drag.js — dragging photos onto Organize's targets, beside the
// number keys. A photo in the selection takes the selection with it; one
// outside it goes alone and becomes the selection, so what moves is always
// what is shown selected. ⌥ when dropping copies, as it does with the keys.

const ORGANIZE_DRAG_TYPE = 'application/x-unterlumen-organize';

class OrganizeDrag {
    // source: the source pane's element; targets: the target list's;
    // pane: the source BrowsePane; onDrop(index, { copy }) sends.
    constructor({ source, targets, pane, onDrop, onSelect }) {
        this._pane = pane;
        this._onDrop = onDrop;
        this._onSelect = onSelect;
        source.addEventListener('dragstart', (e) => this._start(e));
        targets.addEventListener('dragover', (e) => this._over(e));
        targets.addEventListener('dragleave', (e) => this._leave(e));
        targets.addEventListener('drop', (e) => this._drop(e));
    }

    _start(e) {
        const tile = e.target.closest('[data-type="image"]');
        if (!tile) return;
        const path = tile.dataset.path;
        const selection = this._pane.selection;
        if (!selection.selected.has(path)) {
            selection.clear();
            selection.toggle(path);
            this._pane.selectedDirs.clear();
            selection.updateClasses(this._pane.container);
            this._pane._updateDirSelectionClasses();
            this._onSelect();
        }
        e.dataTransfer.setData(ORGANIZE_DRAG_TYPE, String(selection.selected.size));
        e.dataTransfer.effectAllowed = 'copyMove';
    }

    _targetOf(e) {
        if (!e.dataTransfer.types.includes(ORGANIZE_DRAG_TYPE)) return null;
        return e.target.closest('[data-target]');
    }

    _over(e) {
        const target = this._targetOf(e);
        if (!target) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = e.altKey ? 'copy' : 'move';
        target.classList.add('org-target--drop');
    }

    _leave(e) {
        const target = e.target.closest('[data-target]');
        if (target && !target.contains(e.relatedTarget)) target.classList.remove('org-target--drop');
    }

    _drop(e) {
        const target = this._targetOf(e);
        if (!target) return;
        e.preventDefault();
        target.classList.remove('org-target--drop');
        const raw = target.dataset.target;
        this._onDrop(raw === 'mark' ? 'mark' : Number(raw), { copy: e.altKey });
    }
}
