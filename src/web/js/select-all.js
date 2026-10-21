// select-all.js — Select all and Select none for a collection whose items
// can be selected, beside how many there are and how many are selected.
// One component wherever a selection is made: Folders, Organize, a library
// and the filter's results, Marked for deletion, and faces in People.

class SelectAll {
    // onAll and onNone change the selection; the owner calls update after
    // every change, theirs or these. label(total, selected) is the count.
    constructor({ onAll, onNone, label }) {
        this._label = label;
        this.el = document.createElement('div');
        this.el.className = 'select-all';
        this.el.innerHTML = `
            <span class="select-all-count"></span>
            <button type="button" class="btn btn-sm select-all-all">Select all</button>
            <button type="button" class="btn btn-sm select-all-none">Select none</button>`;
        this._count = this.el.querySelector('.select-all-count');
        this._all = this.el.querySelector('.select-all-all');
        this._none = this.el.querySelector('.select-all-none');
        this._all.addEventListener('click', () => onAll());
        this._none.addEventListener('click', () => onNone());
        this.update(0, 0);
    }

    // Nothing to select shows nothing; each button shows while it would
    // change something.
    update(selected, total) {
        this.el.hidden = total === 0;
        this._count.textContent = this._label(total, selected);
        this._all.hidden = selected >= total;
        this._none.hidden = selected === 0;
    }
}
