// people-everyone.js — People in all libraries: everyone named in any of
// them, put together by name (ADR-0051). It only reads: a name is given,
// changed and taken away in each library, where it goes into that library's
// sidecars, so this page sends the owner there.
//
// Addresses: #people/all and #people/all/person/<name>.

const PEOPLE_ALL = 'all';

class PeopleEveryone {
    // pane is the PeoplePane that hosts the page: its body, head and crumb.
    constructor(pane) {
        this.pane = pane;
    }

    // —— Everyone ——

    async showOverview() {
        const body = this.pane._body;
        const activity = Activity.in(body, 'Reading the people of all libraries…', { area: true });
        let people;
        try {
            people = await PeopleAPI.everyone();
        } catch (err) {
            activity.fail(`The people could not be read: ${err.message}.`);
            return;
        }
        const libraries = new Set(people.flatMap(p => p.libraries.map(l => l.library))).size;
        this.pane._head(people.length ? `${countPeople(people.length)} in ${countLibraries(libraries)}` : '');
        body.innerHTML = `
            <p class="people-hint">Everyone named in any of your libraries. The same name is the same person. Names are given and changed in each library.</p>
            ${people.length ? `<div class="people-grid">${people.map(p => this._card(p)).join('')}</div>`
                : `<p class="people-empty">Nobody is named in any library yet. Choose a library above and give a group a name.</p>`}`;
    }

    _card(p) {
        const top = p.libraries[0];
        const where = p.libraries.length === 1 ? top.libraryName : `in ${countLibraries(p.libraries.length)}`;
        return `
            <a class="people-card" href="${peopleHash({ library: PEOPLE_ALL, view: 'person', id: p.name })}">
                <img src="${PeopleAPI.faceURL(top.library, top.cover)}" alt="" loading="lazy" width="96" height="96">
                <span class="people-card-name">${escapeHtml(p.name)}</span>
                <span class="people-card-count">${countPhotos(p.photos)}</span>
                <span class="people-card-where">${escapeHtml(where)}</span>
            </a>`;
    }

    // —— Someone ——

    async showPerson(name) {
        const body = this.pane._body;
        const activity = Activity.in(body, `Looking for ${name} in all libraries…`, { area: true });
        let data, birthdays;
        try {
            [data, birthdays] = await Promise.all([PeopleAPI.someone(name), peopleBirthdays()]);
        } catch (err) {
            activity.fail(`${name} could not be looked for: ${err.message}.`);
            return;
        }
        if (!data.in.length) {
            body.innerHTML = `<div class="people-note"><p>Nobody is named ${escapeHtml(name)} in any library any more. ${this.pane._backLink()}</p></div>`;
            return;
        }
        name = data.name;
        const photos = data.in.reduce((n, l) => n + l.photos, 0);
        this.pane._head(`${countPhotos(photos)} in ${countLibraries(data.in.length)}`);
        body.innerHTML = `
            <div class="people-page-head">${this.pane._crumb(escapeHtml(name))}
                <span class="people-head-spacer"></span>
                <a class="btn people-show" href="#libraries">Show photos</a>
            </div>
            ${bornHTML(birthdayOf(birthdays, name))}
            ${oftenWithHTML(data.with)}
            <p class="people-hint">Names are given and changed in each library. To rename ${escapeHtml(name)} or take the name away, do it on ${escapeHtml(name)}'s page in each library: ${sentenceList(data.in.map(l => this._personLink(l, l.libraryName)))}.</p>
            ${data.in.map(l => this._namedIn(name, l)).join('')}
            ${data.notIn.length ? `
                <section class="people-section">
                    <h2 class="people-section-title">Not named in</h2>
                    ${data.notIn.map(l => this._notNamedIn(name, l)).join('')}
                </section>` : ''}`;
        showPhotosLink(body.querySelector('.people-show'), { person: name });
        wireOftenWith(body, name);
        const menu = new Menu({ label: 'Person options', items: () => [
            { label: 'Birthday…', onSelect: () => editBirthday(name, birthdayOf(birthdays, name), () => this.showPerson(name)) },
        ] });
        body.querySelector('.people-show').after(menu.button);
        data.in.forEach((l, i) => this._wireFaces(body.querySelectorAll('.people-in .face-grid')[i], l));
    }

    _personLink(l, text) {
        return `<a href="${peopleHash({ library: l.library, view: 'person', id: l.person })}">${escapeHtml(text)}</a>`;
    }

    _namedIn(name, l) {
        const more = l.faces > l.shown.length
            ? `<p class="people-hint">The ${formatCount(l.shown.length)} newest of ${formatCount(l.faces)} faces; ${this._personLink(l, `all of them in ${l.libraryName}`)}.</p>` : '';
        return `
            <section class="people-section people-in">
                <div class="people-in-head">
                    <h2 class="people-section-title">${escapeHtml(l.libraryName)}</h2>
                    <span class="people-count">${countPhotos(l.photos)}</span>
                </div>
                <div class="face-grid">${l.shown.map((f, i) => `
                    <button type="button" class="face-tile" data-index="${i}" aria-label="Open the photo of face ${i + 1}">
                        <img src="${PeopleAPI.faceURL(l.library, f.id)}" alt="" loading="lazy" width="96" height="96">
                    </button>`).join('')}
                </div>
                ${more}
            </section>`;
    }

    // A face opens its photo; resting on it shows it in its photo.
    _wireFaces(grid, l) {
        new FacePeek(l.library, l.shown, grid);
        grid.addEventListener('click', (e) => {
            const tile = e.target.closest('.face-tile');
            if (!tile) return;
            const ids = [...new Set(l.shown.map(f => f.photoId))];
            const photos = ids.map(id => ({ lib: l.library, id, name: id }));
            openLibraryPhotos(photos, ids.indexOf(l.shown[Number(tile.dataset.index)].photoId));
        });
    }

    // A library without the name: who there may be them, and where to name
    // them so they count as one person.
    _notNamedIn(name, l) {
        const lib = escapeHtml(l.libraryName);
        let faces = '', text, link = '';
        if (l.looksLike) {
            const other = l.looksLike;
            faces = `<img src="${PeopleAPI.faceURL(l.library, other.cover)}" alt="" loading="lazy" width="64" height="64">`;
            text = `${escapeHtml(other.name)} there looks like ${escapeHtml(name)}. If it is the same person, rename them to ${escapeHtml(name)} there.`;
            link = this._personLink({ library: l.library, person: other.id }, `${other.name} in ${l.libraryName}`);
        } else if (l.group) {
            faces = l.group.faces.map(f => `<img src="${PeopleAPI.faceURL(l.library, f)}" alt="" loading="lazy" width="64" height="64">`).join('');
            text = `A group of <span class="people-num">${formatCount(l.group.size)}</span> faces there waiting for a name looks like ${escapeHtml(name)}.`;
            link = `<a href="${peopleHash({ library: l.library, view: 'group', id: l.group.id })}">Name them in ${lib}</a>`;
        } else {
            text = `Nobody there looks like ${escapeHtml(name)}.`;
        }
        return `
            <div class="people-elsewhere">
                <h3 class="people-elsewhere-title">${lib}</h3>
                ${faces ? `<div class="people-group-faces">${faces}</div>` : ''}
                <p class="people-elsewhere-text">${text}${link ? ` ${link}` : ''}</p>
            </div>`;
    }
}

function countPeople(n) { return n === 1 ? '1 person' : `${formatCount(n)} people`; }
function countLibraries(n) { return n === 1 ? '1 library' : `${formatCount(n)} libraries`; }
function countPhotos(n) { return n === 1 ? '1 photo' : `${formatCount(n)} photos`; }

// sentenceList joins items as a sentence does: "a", "a and b", "a, b and c".
function sentenceList(items) {
    return items.length < 2 ? items.join('') : `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;
}

// The people most often in the same photos as someone, each a link to the
// photos of both in Libraries (the library filter with two people).
function oftenWithHTML(withList) {
    if (!withList?.length) return '';
    const links = withList.map(c => `<a class="btn btn-sm people-with-chip" href="#libraries" data-name="${escapeHtml(c.name)}">${escapeHtml(c.name)} <span class="people-num">${formatCount(c.photos)}</span></a>`);
    return `<div class="people-with"><span class="people-label">Often with</span><div class="people-with-row">${links.join('')}</div></div>`;
}

function wireOftenWith(container, name) {
    for (const a of container.querySelectorAll('a.people-with-chip')) showPhotosLink(a, { person: [name, a.dataset.name] });
}
