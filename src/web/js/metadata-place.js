// metadata-place.js — Metadata: the place where what the chosen photos and
// videos say about themselves is changed (ADR-0060) — when they were taken,
// their title, description, keywords, creator, copyright and rating. Where
// it is written is always in view and chosen for each change: into the
// files themselves, or into what lies beside them (the XMP sidecar of a
// photo, the NFO file of a video). Photos are brought here from a selection
// in Folders, Organize, a library or the viewer.

const META_TARGETS = [
    ['sidecar', 'Sidecar (XMP · NFO)'],
    ['file', 'File (EXIF)'],
];

// More files than this are written in the progress dialog (ADR-0036).
const META_DIALOG_FROM = 5;

class MetadataPane {
    constructor(container) {
        this.container = container;
        this.files = [];
        this.target = 'sidecar';
        this.states = [];
        this.exiftool = true;
        this.result = '';
        this._previewAsk = 0;
    }

    // setFiles takes the files to edit, relative to the boundary.
    setFiles(files) {
        this.files = [...new Set(files)];
        this.result = '';
        this._loaded = null;
    }

    render() {
        this.container.innerHTML = `<div class="browse-container meta-place">
            <div class="meta-head"></div><div class="meta-body"></div></div>`;
        this._renderHead();
        if (!this.files.length) {
            this._renderEmpty();
            return;
        }
        if (this._loaded === this.files) this._renderEditor();
        else this._load();
    }

    _renderHead() {
        const head = this.container.querySelector('.meta-head');
        const meta = this.files.length ? countHTML(this.files.length, 'file') : '';
        const actions = this.files.length ? `<button type="button" class="btn btn-sm btn-accent" id="meta-write">${t('Write')}</button>` : '';
        head.innerHTML = placeHeaderHTML({
            title: t('Metadata'),
            meta,
            actions,
            lede: placeLede(t('When photos and videos were taken, where, what and who they show, who made them and on which terms, their keywords, rating and own fields. Select photos in {folders}, {organize} or a {library} and choose Edit metadata; they come here. Each change is written where you choose: into the files themselves, or beside them.', {
                folders: placeLink('browse', 'folders', t('Folders')),
                organize: placeLink('organize', 'organize', t('Organize')),
                library: placeLink('library', 'libraries', t('library')),
            })),
        });
        head.querySelector('#meta-write')?.addEventListener('click', (e) => this._write(e.currentTarget));
    }

    _renderEmpty() {
        this.container.querySelector('.meta-body').innerHTML = `<div class="wastebin-empty-box">
            <strong>${t('No photos chosen')}</strong>
            <span>${t('Select photos or videos in Folders, Organize or a library, then choose Edit metadata in the bar below them.')}</span></div>`;
    }

    async _load() {
        const body = this.container.querySelector('.meta-body');
        const activity = Activity.in(body, t('Reading what the files say…'), { area: true });
        const files = this.files;
        try {
            const res = await API.readMetadata(files);
            if (files !== this.files) return;
            this.states = res.files;
            this.exiftool = res.exiftool;
            if (!this.exiftool) this.target = 'sidecar';
            this._loaded = files;
        } catch (err) {
            activity.fail(t('The files could not be read: {error}. Check that their folder is reachable and try again.', { error: err.message }));
            return;
        }
        this._renderEditor();
    }

    _renderEditor() {
        const body = this.container.querySelector('.meta-body');
        body.innerHTML = `<div class="meta-layout">
            <div class="meta-edit">
                <section class="meta-target"></section>
                <section class="meta-fields"></section>
            </div>
            <div class="meta-side">
                <div class="meta-result" aria-live="polite"></div>
                <div class="meta-preview-wrap"></div>
            </div></div>`;
        this._renderTarget();
        const onChange = () => this._changed();
        this.form = new MetadataForm(body.querySelector('.meta-fields'), this.states, { onChange });
        this.date = new MetadataDate(this.form.dateSlot, this.states, { onChange });
        this._showResult();
        this._changed();
    }

    /* --- Where it is written --- */

    _renderTarget() {
        const box = this.container.querySelector('.meta-target');
        box.innerHTML = '<div class="meta-target-choice"></div><p class="meta-target-text"></p>';
        const field = segmentField({
            label: 'Write to',
            options: META_TARGETS,
            value: this.target,
            onChange: (target) => { this.target = target; this._changed(); },
        });
        if (!this.exiftool) {
            const btn = field.querySelector('button[data-value="file"]');
            btn.disabled = true;
            btn.title = t('exiftool is not installed');
            field.querySelector('option[value="file"]').disabled = true;
        }
        box.querySelector('.meta-target-choice').appendChild(field);
    }

    _targetText() {
        const videos = this.states.filter(s => s.current?.video).length;
        const photos = this.states.filter(s => s.current && !s.current.video).length;
        const parts = [];
        if (this.target === 'file') {
            if (photos) parts.push(t('Changes {photos} themselves. Their content changes, and with it their ID in a library; everything else in them stays.', { photos: countText(photos, 'photo') }));
            if (videos) parts.push(t('{videos} are never changed; what concerns them goes into an NFO file beside each.', { videos: countText(videos, 'video') }));
        } else {
            if (photos) parts.push(t('Writes the XMP sidecar beside {photos}.', { photos: countText(photos, 'photo') }));
            if (videos) parts.push(t('Writes an NFO file beside {videos}, as Kodi and Jellyfin read it.', { videos: countText(videos, 'video') }));
            parts.push(t('The files themselves stay as they are.'));
        }
        if (!this.exiftool) parts.push(t('exiftool is not installed, so nothing can be written into the files; sidecars work without it.'));
        const shadowed = this._shadowedBySidecar();
        if (shadowed) parts.push(shadowed);
        return parts.join(' ');
    }

    // A value written into a file is not what is shown while the sidecar has
    // one of its own: what is beside a file wins.
    _shadowedBySidecar() {
        if (this.target !== 'file' || !this.form) return '';
        const fields = this.form.changedFields();
        if (this.date?.dateFix()) fields.push('dateTaken');
        const n = this.states.filter(s => fields.some(f => s.current?.fields?.[f]?.from === 'sidecar')).length;
        if (!n) return '';
        return t('{files} have a value of their own in the sidecar, which stays and is still the one shown.', { files: countText(n, 'file') });
    }

    /* --- The change and its preview --- */

    change() {
        const change = this.form ? this.form.change() : {};
        const date = this.date?.dateFix();
        if (date) change.date = date;
        return change;
    }

    _hasChange() {
        const c = this.change();
        return Boolean(c.set || c.date || c.keywordsAdd || c.keywordsRemove || c.add || c.remove || c.own || c.countryFromGps);
    }

    _changed() {
        const text = this.container.querySelector('.meta-target-text');
        if (text) text.textContent = this._targetText();
        const btn = this.container.querySelector('#meta-write');
        if (btn) {
            btn.disabled = !this._hasChange();
            btn.title = btn.disabled ? t('Change a field to write it.') : '';
        }
        clearTimeout(this._previewTimer);
        this._previewTimer = setTimeout(() => this._preview(), 250);
    }

    async _preview() {
        const wrap = this.container.querySelector('.meta-preview-wrap');
        if (!wrap) return;
        const change = this.change();
        const ask = ++this._previewAsk;
        // Only a change worked out per file is asked for; the plan read with
        // the files is of no change at all.
        let states = this.states.map(s => ({ ...s, plan: null }));
        if (change.date || change.add || change.remove || change.own || change.countryFromGps) {
            try {
                states = (await API.readMetadata(this.files, change)).files;
            } catch (err) {
                wrap.innerHTML = `<p class="form-hint form-hint--error">${escapeHtml(err.message)}</p>`;
                return;
            }
        }
        if (ask !== this._previewAsk) return;
        const hint = this._hasChange() ? '' : `<p class="form-hint">${t('Nothing to write yet: change a field and each file shows here what it gets.')}</p>`;
        wrap.innerHTML = hint + metadataPreviewHTML(states, {
            changedFields: this.form.changedFields(),
            keywords: this.form.keywordsChanged(),
        });
    }

    /* --- Writing --- */

    async _write(btn) {
        const change = this.change();
        const target = this.target;
        const results = [];
        const action = async (file) => {
            const r = await API.writeMetadata(file, target, change);
            results.push(r);
            return r;
        };
        if (this.files.length > META_DIALOG_FROM) {
            new ProgressDialog().open(this.files, {
                title: t('Writing metadata'),
                done: t('Written'),
                action,
                onComplete: () => this._written(results),
            });
            return;
        }
        const restore = Activity.button(btn, t('Writing…'));
        for (const file of this.files) {
            try {
                await action(file);
            } catch (err) {
                results.push({ file, success: false, error: err.message });
            }
        }
        restore();
        this._written(results);
    }

    // What was written is read again, so the editor starts from it.
    _written(results) {
        const written = results.filter(r => r.success && !r.skipped).length;
        const skipped = results.filter(r => r.skipped).length;
        const failed = results.filter(r => !r.success);
        let text = t('Written to {files}.', { files: countText(written, 'file') });
        if (skipped) text += ' ' + t('{files} left as they were: no date taken.', { files: countText(skipped, 'file') });
        this.result = { text, failed };
        App.reloadLibraryPane();
        this._loaded = null;
        this.render();
    }

    _showResult() {
        const box = this.container.querySelector('.meta-result');
        if (!box || !this.result) return;
        box.innerHTML = `<p class="meta-result-text">${escapeHtml(this.result.text)}</p>`;
        if (this.result.failed.length) {
            box.insertAdjacentHTML('beforeend', `<p class="meta-result-error">${t('{files} could not be written:', { files: countText(this.result.failed.length, 'file') })}</p>`);
            box.appendChild(Activity.errorList(this.result.failed.map(r => `${r.file.split('/').pop()}: ${r.error}`)));
        }
    }
}
