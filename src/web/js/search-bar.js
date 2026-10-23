// search-bar.js — the search above every place: people, things, or any words
// (ADR-0054). Typing suggests the people named and the things found whose
// names hold the letters, and last the photos that show the words as typed;
// choosing one opens Libraries with its filter set, where the search can be
// refined. "/" comes here from anywhere.

const SEARCH_PEOPLE = 5;
const SEARCH_THINGS = 6;

class SearchBar {
    constructor(host) {
        host.innerHTML = `
            <label class="search-bar-label" for="search-bar-input">Search</label>
            <div class="search-bar-field">
                <input id="search-bar-input" class="search-bar-input" type="text" autocomplete="off" spellcheck="false" aria-label="Search"
                    placeholder="A person, a thing, or words such as dog on a beach"
                    role="combobox" aria-expanded="false" aria-controls="search-bar-list" aria-autocomplete="list">
                <ul id="search-bar-list" class="search-bar-list" role="listbox" aria-label="Suggestions" hidden></ul>
            </div>`;
        this._input = host.querySelector('.search-bar-input');
        this._list = host.querySelector('.search-bar-list');
        this._options = [];
        this._active = -1;
        this._input.addEventListener('focus', () => { this._loading = this._load(); });
        this._input.addEventListener('input', () => this._suggest());
        this._input.addEventListener('keydown', (e) => this._key(e));
        this._input.addEventListener('blur', () => setTimeout(() => this._close(), 120));
        this._list.addEventListener('mousedown', (e) => e.preventDefault()); // the field keeps the focus
        this._list.addEventListener('click', (e) => {
            const li = e.target.closest('[role="option"]');
            if (li) this._choose(this._options[Number(li.dataset.index)]);
        });
    }

    focus() {
        this._input.focus();
        this._input.select();
    }

    // What there is to suggest, read again on every visit: names and things
    // change while the app is open.
    async _load() {
        const [names, things, status] = await Promise.all([
            LibraryAPI.personNames().catch(() => []),
            LibraryAPI.thingTotals().catch(() => []),
            ThingsAPI.status().catch(() => ({ installed: false })),
        ]);
        this._names = names;
        this._things = things;
        this._words = status.installed;
        if (this._input.value.trim()) this._suggest();
    }

    _suggest() {
        const text = this._input.value.trim();
        const low = text.toLowerCase();
        this._options = [];
        if (text) {
            for (const name of (this._names || []).filter(n => n.toLowerCase().includes(low)).slice(0, SEARCH_PEOPLE)) {
                this._options.push({ kind: 'People', label: name, criteria: { person: name } });
            }
            for (const t of (this._things || []).filter(t => t.thing.includes(low)).slice(0, SEARCH_THINGS)) {
                this._options.push({ kind: 'Things', label: t.thing, count: t.photos, criteria: { thing: t.thing } });
            }
            if (this._words) this._options.push({ kind: 'Words', label: `Photos that show “${text}”`, criteria: { text } });
        }
        this._active = this._options.length ? 0 : -1;
        this._draw();
    }

    _draw() {
        if (!this._options.length) { this._close(); return; }
        let kind = '';
        this._list.innerHTML = this._options.map((o, i) => {
            const head = o.kind !== kind && o.kind !== 'Words' ? `<li class="search-bar-kind" role="presentation">${o.kind}</li>` : '';
            kind = o.kind;
            const count = o.count !== undefined ? `<span class="search-bar-count">${countText(o.count, 'photo', 'photos')}</span>` : '';
            return `${head}<li id="search-bar-option-${i}" class="search-bar-option${o.kind === 'Words' ? ' search-bar-words' : ''}" role="option" data-index="${i}" aria-selected="${i === this._active}">` +
                `<span class="search-bar-name">${escapeHtml(o.label)}</span>${count}</li>`;
        }).join('');
        this._list.hidden = false;
        this._input.setAttribute('aria-expanded', 'true');
        this._input.setAttribute('aria-activedescendant', `search-bar-option-${this._active}`);
    }

    _close() {
        this._list.hidden = true;
        this._input.setAttribute('aria-expanded', 'false');
        this._input.removeAttribute('aria-activedescendant');
    }

    _key(e) {
        const open = !this._list.hidden;
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            if (!this._options.length) return;
            e.preventDefault();
            const step = e.key === 'ArrowDown' ? 1 : -1;
            this._active = (this._active + step + this._options.length) % this._options.length;
            this._draw();
        } else if (e.key === 'Enter') {
            e.preventDefault();
            this._enter();
        } else if (e.key === 'Escape' && open) {
            e.stopPropagation(); // the first Escape closes the list, the next leaves the field
            this._close();
        }
    }

    // Enter takes the suggestion shown, or — typed before the suggestions
    // were there — the best one once they are.
    async _enter() {
        if (!this._options.length && this._input.value.trim()) {
            await this._loading;
            this._suggest();
        }
        if (this._options[this._active]) this._choose(this._options[this._active]);
    }

    _choose(option) {
        this._close();
        this._input.value = '';
        this._input.blur();
        App.showPhotos(option.criteria);
    }
}
