// face-grid.js — pictures of faces, each one a switch: in or out of what
// the next action does. People's group and person pages are built on it.

class FaceGrid {
    // faces: [{ id, photoId }]; selected: the ids switched on at the start.
    // dimOff fades the faces switched off — the ones left out of a group;
    // without it only the switched-on faces are marked. onChange is called
    // with the selection after every switch; onOpen with the index of a
    // face that is double-clicked.
    constructor({ lib, faces, selected = [], dimOff = false, onChange = () => {}, onOpen = () => {} }) {
        this.selected = new Set(selected);
        this.el = document.createElement('div');
        this.el.className = dimOff ? 'face-grid face-grid-dim-off' : 'face-grid';
        this.el.innerHTML = faces.map((f, i) => `
            <button type="button" class="face-tile" data-face="${f.id}" data-index="${i}"
                aria-pressed="${this.selected.has(f.id)}" aria-label="Face ${i + 1}">
                <img src="${PeopleAPI.faceURL(lib, f.id)}" alt="" loading="lazy" width="96" height="96">
                <svg class="face-tile-mark" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
                    <rect width="16" height="16" rx="2"/><path d="M4 8.5l2.5 2.5L12 5.5" fill="none" stroke-width="1.8"/>
                </svg>
            </button>`).join('');
        this.el.addEventListener('click', (e) => {
            const tile = e.target.closest('.face-tile');
            if (!tile) return;
            const id = Number(tile.dataset.face);
            const on = !this.selected.has(id);
            if (on) this.selected.add(id); else this.selected.delete(id);
            tile.setAttribute('aria-pressed', String(on));
            onChange(this.selected);
        });
        this.el.addEventListener('dblclick', (e) => {
            const tile = e.target.closest('.face-tile');
            if (tile) onOpen(Number(tile.dataset.index));
        });
    }
}
