// duplicates-place.js — Duplicates: the versions of one photo in all the
// libraries, side by side, with what tells them apart — pixels, bytes,
// format, name and path — so the ones not wanted can be marked for deletion
// (ADR-0059). Deleting happens in Marked for deletion, as for any photo.

const DUPLICATE_KINDS = [
    ['all', 'All'],
    ['identical', 'Identical'],
    ['same', 'Same picture'],
    ['similar', 'Similar'],
];

// What a version is to the group's largest, which comes first.
const DUPLICATE_RELATIONS = {
    identical: 'Identical file',
    scaled: 'Scaled',
    resaved: 'Saved again',
    similar: 'Similar',
};

class DuplicatesPane {
    constructor(container) {
        this.container = container;
        this.kind = 'all';
        this.report = null;
    }

    // Each visit reads the libraries again: files deleted since, here or in
    // Marked for deletion, must not be offered once more.
    async render() {
        this.container.innerHTML = `<div class="browse-container duplicates-place">
            <div class="duplicates-head"></div><div class="duplicates-body"></div></div>`;
        this._renderHead();
        const body = this.container.querySelector('.duplicates-body');
        const activity = Activity.in(body, t('Comparing the photos of all libraries…'), { area: true });
        try {
            this.report = await LibraryAPI.duplicates();
        } catch (err) {
            activity.fail(t('The libraries could not be compared: {error}. Try again when every library folder is reachable.', { error: err.message }));
            return;
        }
        this._renderHead();
        this._renderUnreachable();
        this._renderGroups();
    }

    // A library whose folder is not connected is left out, and says so: its
    // duplicates are not gone, only not looked at.
    _renderUnreachable() {
        const away = this.report.unreachable || [];
        if (!away.length) return;
        this.container.querySelector('.duplicates-head').insertAdjacentHTML('afterend',
            `<div class="library-notice">${t('Left out because their folder cannot be reached: {libraries}. Connect the disk or the share and open Duplicates again.', { libraries: away.map(escapeHtml).join(', ') })}</div>`);
    }

    _shownGroups() {
        const groups = this.report?.groups || [];
        return this.kind === 'all' ? groups : groups.filter(g => g.kind === this.kind);
    }

    _renderHead() {
        const head = this.container.querySelector('.duplicates-head');
        const meta = this.report ? countHTML(this._shownGroups().length, 'group') : '';
        head.innerHTML = placeHeaderHTML({ title: t('Duplicates'), meta, lede: placeLede(this._ledeText()) });
        head.querySelector('.place-header-actions').appendChild(segmentField({
            label: 'Show',
            options: DUPLICATE_KINDS,
            value: this.kind,
            onChange: (kind) => { this.kind = kind; this._renderHead(); this._renderGroups(); },
        }));
    }

    _ledeText() {
        let text = t('Versions of one photo in all libraries: the same file twice, the same picture scaled or saved again, and edited or cropped copies. The largest comes first. Mark the versions you don’t want; they are deleted in Marked for deletion.');
        const n = this.report?.unanalysed || 0;
        if (n) text += ' ' + t('{photos} are not analysed yet and are compared only as identical files.', { photos: countText(n, 'photo') });
        return text;
    }

    _renderGroups() {
        const body = this.container.querySelector('.duplicates-body');
        if (!this.report) return;
        const groups = this._shownGroups();
        if (!groups.length) {
            body.innerHTML = `<div class="wastebin-empty-box"><strong>${t('No duplicates')}</strong>
                <span>${this.report.unreachable?.length ? t('No photo is in the reachable libraries more than once.') : t('No photo is in the libraries more than once.')}</span></div>`;
            return;
        }
        body.innerHTML = groups.map((g, gi) => this._groupHTML(g, gi)).join('');
        body.onclick = (e) => this._onClick(e, groups);
    }

    _groupHTML(group, gi) {
        const kindLabel = DUPLICATE_KINDS.find(([k]) => k === group.kind)?.[1] || group.kind;
        return `<section class="duplicates-group" data-group="${gi}">
            <h2 class="duplicates-group-title">${escapeHtml(t(kindLabel))} <span class="duplicates-group-count">${countText(group.files.length, 'version')}</span></h2>
            <div class="duplicates-versions">${group.files.map((f, fi) => this._versionHTML(f, fi)).join('')}</div>
        </section>`;
    }

    _versionHTML(f, fi) {
        const marked = App.wastebin.isMarked(f.path);
        const relation = escapeHtml(fi === 0 ? t('Largest') : t(DUPLICATE_RELATIONS[f.relation] || f.relation));
        const size = f.width && f.height ? `${formatCount(f.width)} × ${formatCount(f.height)}` : t('Size unknown');
        const libs = f.libraries.length > 1 ? `<div class="duplicates-libs">${f.libraries.map(l => escapeHtml(l.name)).join(' · ')}</div>` : '';
        return `<article class="duplicates-version${marked ? ' is-marked' : ''}" data-file="${fi}">
            <button type="button" class="duplicates-thumb grid-item${marked ? ' marked-for-deletion' : ''}" data-open aria-label="${escapeHtml(t('Open {name}', { name: f.name }))}">
                <img src="${LibraryAPI.thumbURL(f.libraries[0].id, f.photoID)}" alt="" loading="lazy">
            </button>
            <div class="duplicates-relation">${relation}</div>
            <dl class="duplicates-data">
                <dt>${t('Pixels')}</dt><dd>${size}</dd>
                <dt>${t('File size')}</dt><dd>${formatSize(f.size)}</dd>
                <dt>${t('Format')}</dt><dd>${escapeHtml((f.ext || '').toUpperCase())}</dd>
            </dl>
            <div class="duplicates-name">${escapeHtml(f.name)}</div>
            <div class="duplicates-path" title="${escapeHtml(f.path)}">${escapeHtml(f.path.slice(0, f.path.length - f.name.length))}</div>
            ${libs}
            <button type="button" class="btn btn-sm" data-mark>${marked ? t('Unmark') : t('Mark for deletion')}</button>
        </article>`;
    }

    _onClick(e, groups) {
        const card = e.target.closest('.duplicates-version');
        if (!card) return;
        const group = groups[+card.closest('.duplicates-group').dataset.group];
        const index = +card.dataset.file;
        if (e.target.closest('[data-open]')) {
            openLibraryPhotos(group.files.map(f => ({ lib: f.libraries[0].id, id: f.photoID, name: f.path })), index);
        } else if (e.target.closest('[data-mark]')) {
            this._toggleMark(group.files[index]);
            card.outerHTML = this._versionHTML(group.files[index], index);
        }
    }

    // A version is marked by its path, so that of two copies in one library
    // the one chosen is deleted, not the one the index names first.
    _toggleMark(f) {
        if (App.wastebin.isMarked(f.path)) {
            App.wastebin.restore([f.path]);
            return;
        }
        App.wastebin.markFile(f.path, {
            name: f.name, label: f.name, type: 'image', size: f.size,
            libID: f.libraries[0].id, photoID: f.photoID, byPath: true,
        });
    }
}
