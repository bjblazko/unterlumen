// select-all.js — Select all and Select none for a collection whose items
// can be selected, beside how many are selected. How many there are is the
// place's title's to say.
// One component wherever a selection is made: Folders, Organize, a library
// and the filter's results, Marked for deletion, and faces in People.

class SelectAll {
    // onAll and onNone change the selection; the owner calls update after
    // every change, theirs or these.
    constructor({ onAll, onNone }) {
        this.el = document.createElement('div');
        this.el.className = 'select-all';
        this.el.innerHTML = `
            <span class="select-all-count"></span>
            <button type="button" class="btn btn-sm select-all-all">${t('Select all')}</button>
            <button type="button" class="btn btn-sm select-all-none">${t('Select none')}</button>`;
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
        this._count.textContent = selected ? t('{n} selected', { n: formatCount(selected) }) : '';
        this._all.hidden = selected >= total;
        this._none.hidden = selected === 0;
    }
}
