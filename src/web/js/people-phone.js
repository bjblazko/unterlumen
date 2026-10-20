// people-phone.js — People on a phone: finding people and looking at their
// photos, nothing to manage (ADR-0051). Faces are found and named on the
// computer; the phone shows everyone named in any library, a search over
// their names, and for each person their photos by year and who is most
// often with them.
//
// Addresses as on the computer: #people and #people/person/<name>; the
// computer's pages for naming lead to everyone here.

const PEOPLE_PHONE = '(max-width: 700px)';
const PEOPLE_PHONE_PAGE = 60;

function peopleOnPhone() {
    return matchMedia(PEOPLE_PHONE).matches;
}

class PeoplePhone {
    // pane is the PeoplePane that hosts the page: its body and head.
    constructor(pane) {
        this.pane = pane;
    }

    // —— Everyone ——

    async showOverview() {
        const body = this.pane._body;
        const activity = Activity.in(body, 'Reading the people…', { area: true });
        let people;
        try {
            people = await PeopleAPI.everyone();
        } catch (err) {
            activity.fail(`The people could not be read: ${err.message}.`);
            return;
        }
        this.pane._head(people.length ? countPeople(people.length) : '');
        this.pane._lede(!people.length);
        if (!people.length) {
            body.innerHTML = '<div class="people-note"><p>Nobody is named yet. Faces are found and named on your computer; the people named there show here.</p></div>';
            return;
        }
        body.innerHTML = `
            <div class="people-phone">
                <label class="people-label" for="people-find">Find a person</label>
                <input class="form-input people-find" id="people-find" type="search" autocomplete="off" enterkeyhint="search">
                <div class="people-grid people-phone-grid">${people.map(p => this._card(p)).join('')}</div>
                <p class="people-empty people-phone-none" hidden>Nobody by that name.</p>
            </div>`;
        const input = body.querySelector('#people-find');
        const cards = [...body.querySelectorAll('.people-card')];
        input.addEventListener('input', () => {
            const q = input.value.trim().toLowerCase();
            let shown = 0;
            for (const card of cards) {
                const hit = !q || card.dataset.name.includes(q);
                card.hidden = !hit;
                if (hit) shown++;
            }
            body.querySelector('.people-phone-none').hidden = shown > 0;
        });
    }

    _card(p) {
        const top = p.libraries[0];
        return `
            <a class="people-card" href="${peopleHash({ view: 'person', id: p.name })}" data-name="${escapeHtml(p.name.toLowerCase())}">
                <img src="${PeopleAPI.faceURL(top.library, top.cover)}" alt="" loading="lazy" width="96" height="96">
                <span class="people-card-name">${escapeHtml(p.name)}</span>
                <span class="people-card-count">${countPhotos(p.photos)}</span>
            </a>`;
    }

    // —— Someone ——

    async showPerson(name) {
        const body = this.pane._body;
        const activity = Activity.in(body, `Reading ${name}…`, { area: true });
        let data, birthdays;
        try {
            [data, birthdays] = await Promise.all([PeopleAPI.someone(name), peopleBirthdays()]);
        } catch (err) {
            activity.fail(`${name} could not be read: ${err.message}.`);
            return;
        }
        if (!data.in.length) {
            body.innerHTML = `<div class="people-note"><p>Nobody is named ${escapeHtml(name)} any more. <a class="people-back" href="#people">People</a></p></div>`;
            return;
        }
        this._name = data.name;
        this._with = new Set();
        const top = data.in[0];
        const photos = data.in.reduce((n, l) => n + l.photos, 0);
        this.pane._head('');
        body.innerHTML = `
            <div class="people-phone">
                <a class="people-back" href="#people">People</a>
                <div class="people-phone-hero">
                    <img src="${PeopleAPI.faceURL(top.library, top.cover)}" alt="" width="88" height="88">
                    <div>
                        <h2 class="people-page-title">${escapeHtml(data.name)}</h2>
                        ${bornHTML(birthdayOf(birthdays, data.name))}
                        <p class="people-count">${countPhotos(photos)}</p>
                    </div>
                </div>
                ${data.with.length ? `
                <div class="people-with">
                    <span class="people-label">Often with</span>
                    <div class="people-with-row">${data.with.map(c => `
                        <button type="button" class="btn btn-sm people-with-chip" aria-pressed="false" data-name="${escapeHtml(c.name)}">${escapeHtml(c.name)} <span class="people-num">${formatCount(c.photos)}</span></button>`).join('')}
                    </div>
                </div>` : ''}
                <p class="people-phone-subject"></p>
                <div class="people-phone-photos"></div>
                <div class="people-phone-more"></div>
            </div>`;
        for (const chip of body.querySelectorAll('.people-with-chip')) {
            chip.addEventListener('click', () => {
                const on = !this._with.has(chip.dataset.name);
                if (on) this._with.add(chip.dataset.name); else this._with.delete(chip.dataset.name);
                chip.setAttribute('aria-pressed', String(on));
                this._startPhotos();
            });
        }
        this._startPhotos();
    }

    // The photos with the person, and with everyone chosen under Often
    // with, newest first, by year; more as the end comes into view.
    _startPhotos() {
        const body = this.pane._body;
        const names = [this._name, ...this._with];
        body.querySelector('.people-phone-subject').textContent = this._with.size
            ? `Photos of ${sentenceList(names)} together` : '';
        this._grid = body.querySelector('.people-phone-photos');
        this._grid.innerHTML = '';
        this._photos = [];
        this._total = Infinity;
        this._year = null;
        this._gen = (this._gen || 0) + 1;
        this._observer?.disconnect();
        const more = body.querySelector('.people-phone-more');
        this._observer = new IntersectionObserver((entries) => {
            if (entries.some(e => e.isIntersecting)) this._morePhotos(names, this._gen);
        }, { rootMargin: '400px' });
        this._observer.observe(more);
        this._morePhotos(names, this._gen);
    }

    // A load belongs to its choice of people (gen): one still on its way
    // for an earlier choice neither blocks this one nor adds its photos.
    async _morePhotos(names, gen) {
        if (this._loadingGen === gen || gen !== this._gen || this._photos.length >= this._total) return;
        this._loadingGen = gen;
        try {
            const page = await LibraryAPI.search({ person: names, offset: this._photos.length, limit: PEOPLE_PHONE_PAGE });
            if (gen !== this._gen) return;
            // An empty page ends it, whatever the total says.
            this._total = page.results.length ? page.total : this._photos.length;
            this._append(page.results.map(r => ({ lib: r.libraryID, id: r.id, name: r.filename, taken: r.dateTaken || '' })));
            if (!this._photos.length) this._grid.innerHTML = '<p class="people-empty">No photos show all of them together.</p>';
        } catch (err) {
            if (gen !== this._gen) return;
            this._grid.insertAdjacentHTML('beforeend', `<p class="people-group-error">The photos could not be read: ${escapeHtml(err.message)}</p>`);
            this._total = this._photos.length;
        } finally {
            if (this._loadingGen === gen) this._loadingGen = null;
        }
        // The observer only speaks when the end comes into view; when it is
        // in view already, the next page is asked for here.
        if (gen === this._gen && this._photos.length < this._total && this._moreInView()) this._morePhotos(names, gen);
    }

    _moreInView() {
        const r = this.pane._body.querySelector('.people-phone-more')?.getBoundingClientRect();
        return !!r && r.top < window.innerHeight + 400;
    }

    _append(photos) {
        let block = this._grid.lastElementChild?.classList.contains('people-phone-year-photos') ? this._grid.lastElementChild : null;
        for (const p of photos) {
            const year = p.taken.slice(0, 4) || 'No date';
            if (year !== this._year) {
                this._year = year;
                this._grid.insertAdjacentHTML('beforeend', `<h3 class="people-year">${escapeHtml(year)}</h3><div class="people-phone-year-photos"></div>`);
                block = this._grid.lastElementChild;
            }
            const index = this._photos.push(p) - 1;
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'people-phone-photo';
            btn.setAttribute('aria-label', `Open ${p.name}`);
            btn.innerHTML = `<img src="${LibraryAPI.thumbURL(p.lib, p.id)}" alt="" loading="lazy">`;
            btn.addEventListener('click', () => openLibraryPhotos(this._photos, index));
            block.appendChild(btn);
        }
    }
}
