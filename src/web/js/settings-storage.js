// settings-storage.js — Storage in Settings: what Unterlumen keeps on this
// computer, part by part, and what each part is for. The cache is copies,
// made again when needed, so each part of it can be cleared and the whole
// kept within a limit. The data folder is not copies: a library is deleted
// here as in its own dialog, and the models can be removed, to be
// downloaded again where they are used.

const STORAGE_CACHE_PARTS = {
    thumbnails: [t('Previews'), t('Small versions of the photos for Folders and the grids. Made again as you browse.')],
    images: [t('Viewer images'), t('Full-size images for the viewer, among them HEIF photos converted to JPEG. Made again when a photo is opened.')],
    older: [t('Left by older versions'), t('Nothing reads these any more; they go at the next start, or now.')],
};

const STORAGE_LIMITS = [[2, '2 GB'], [5, '5 GB'], [10, '10 GB'], [20, '20 GB'], [50, '50 GB'], [0, t('No limit')]];

// Thousands of megabytes are hard to read; say what a person would say.
function readableBytes(bytes) {
    if (bytes >= 1073741824) return `${(bytes / 1073741824).toFixed(1)} GB`;
    if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(1)} MB`;
    return `${Math.round(bytes / 1024)} KB`;
}

class SettingsStorage {
    constructor(el) {
        this.el = el;
    }

    async load() {
        let s;
        try {
            s = await API.storage();
        } catch (err) {
            this.el.innerHTML = `<span class="form-label">${t('Storage')}</span><span class="gal-detail-error">Could not measure the storage: ${escapeHtml(err.message)}</span>`;
            return;
        }
        const parts = s.cache.segments.filter(p => p.id !== 'older' || p.bytes > 0);
        const data = s.data ? this._dataRows(s.data) : [];
        const most = Math.max(1, ...parts.map(p => p.bytes), ...data.map(d => d.bytes));
        const total = parts.reduce((n, p) => n + p.bytes, 0);
        this.el.innerHTML = `
            <span class="form-label">${t('Storage')}</span>
            <span class="form-hint">${t('What Unterlumen keeps on this computer besides your photos, and what each part is for.')}</span>
            <div class="storage-group">
                <div class="storage-head">
                    <h3 class="storage-title">${t('Cache')}</h3>
                    <span class="storage-size">${readableBytes(total)}</span>
                    ${this._reveal(s, 'cache')}
                </div>
                <span class="settings-cache settings-cache-path">${escapeHtml(s.cache.path)}</span>
                ${parts.map(p => this._row(escapeHtml(STORAGE_CACHE_PARTS[p.id][0]), STORAGE_CACHE_PARTS[p.id][1], p.bytes, most,
                    p.bytes ? `<button type="button" class="btn btn-sm storage-clear">${t('Clear')}</button>` : '', p.id)).join('')}
                <label class="storage-limit">${t('Keep the cache within')}
                    <select class="btn btn-sm select-btn" id="storage-limit">${this._limitOptions(s.cache.limit)}</select>
                </label>
                <span class="form-hint">${t('When it grows past that, the files used least recently go.')}</span>
            </div>
            ${s.data ? `
            <div class="storage-group">
                <div class="storage-head">
                    <h3 class="storage-title">${t('Data folder')}</h3>
                    <span class="storage-size">${readableBytes(data.reduce((n, d) => n + d.bytes, 0))}</span>
                    ${this._reveal(s, 'data')}
                </div>
                <span class="settings-cache settings-cache-path">${escapeHtml(s.data.path)}</span>
                <span class="form-hint">${t('Not copies. Each library keeps its thumbnails, faces and index here: the thumbnails say where its faces are, the index what was found and named. A library’s name leads to it.')}</span>
                ${data.map(d => this._row(d.labelHTML, d.hint, d.bytes, most, d.action, d.part)).join('')}
            </div>` : ''}`;
        this._wire();
    }

    // A row's name leads to the place that uses it; its button removes it.
    // What a library keeps is said once above the rows, not on each.
    _dataRows(data) {
        this._libraries = new Map(data.libraries.map(l => [`library:${l.id}`, l]));
        const remove = (label) => `<button type="button" class="btn btn-sm storage-remove">${label}</button>`;
        const rows = data.libraries.map(l => ({ labelHTML: libraryLink(l), bytes: l.bytes, hint: '', action: remove(t('Delete library…')), part: `library:${l.id}` }));
        const runtimeNeeded = data.recognition || data.things;
        if (data.recognition) rows.push({ labelHTML: placeLink('people', 'people', t('Face recognition')), bytes: data.recognition, hint: t('The two face models.'), action: remove(t('Remove…')), part: 'recognition' });
        if (data.things) rows.push({ labelHTML: placeLink('statistics', 'statistics', t('Things')), bytes: data.things, hint: t('SigLIP 2 and the object detector.'), action: remove(t('Remove…')), part: 'things' });
        if (data.runtime) rows.push({ labelHTML: escapeHtml(t('ONNX Runtime')), bytes: data.runtime, hint: runtimeNeeded ? t('Runs the face and things models.') : t('Runs the face and things models; neither is here now.'), action: runtimeNeeded ? '' : remove(t('Remove…')), part: 'runtime' });
        // The galleries overview is read from the output, so it is not
        // cleared here; Destinations looks after it.
        if (data.output) rows.push({ labelHTML: placeLink('destinations', 'destinations', t('Destination output')), bytes: data.output, hint: t('What destinations built on this computer, ready to publish. Galleries are read from it, so it stays.'), action: '', part: '' });
        return rows.sort((a, b) => b.bytes - a.bytes);
    }

    // labelHTML: the row's name, escaped already — a library's is a link.
    _row(labelHTML, hint, bytes, most, action, part = '') {
        const width = bytes ? Math.max(0.5, 100 * bytes / most) : 0;
        return `
            <div class="storage-row"${part ? ` data-part="${escapeHtml(part)}"` : ''}>
                <span class="storage-label">${labelHTML}</span>
                <span class="storage-bar" aria-hidden="true"><span style="width:${width.toFixed(1)}%"></span></span>
                <span class="storage-size">${readableBytes(bytes)}</span>
                <span class="storage-action">${action}</span>
                ${hint ? `<span class="storage-hint">${escapeHtml(hint)}</span>` : ''}
                <div class="storage-confirm"></div>
            </div>`;
    }

    _reveal(s, folder) {
        if (!s.canReveal) return '';
        const where = /Mac/.test(navigator.platform) ? 'Finder' : t('file manager');
        return `<button type="button" class="btn btn-sm storage-reveal" data-folder="${escapeHtml(folder)}">${t('Show in {where}', { where })}</button>`;
    }

    _limitOptions(limit) {
        const gb = Math.round(limit / 1073741824);
        const known = STORAGE_LIMITS.some(([v]) => v === gb);
        const options = known ? STORAGE_LIMITS : [...STORAGE_LIMITS, [gb, `${gb} GB`]];
        return options.map(([v, text]) => `<option value="${v}"${v === gb ? ' selected' : ''}>${text}</option>`).join('');
    }

    _wire() {
        for (const btn of this.el.querySelectorAll('.storage-clear')) {
            btn.addEventListener('click', () => this._confirmClear(btn.closest('.storage-row')));
        }
        for (const btn of this.el.querySelectorAll('.storage-remove')) {
            btn.addEventListener('click', () => this._confirmRemove(btn.closest('.storage-row')));
        }
        for (const btn of this.el.querySelectorAll('.storage-reveal')) {
            btn.addEventListener('click', async () => {
                try {
                    await API.storageReveal(btn.dataset.folder);
                } catch (err) {
                    App.showToast(err.message);
                }
            });
        }
        this.el.querySelector('#storage-limit').addEventListener('change', async (e) => {
            try {
                await API.storageLimit(Number(e.target.value) * 1073741824);
                setTimeout(() => this.load(), 1500);
            } catch (err) {
                App.showToast(t('The limit was not changed: {err}', { err: err.message }));
            }
        });
    }

    // Clearing costs nothing but time, so the question is short — but it is
    // still asked, and asked here rather than in a browser box.
    _confirmClear(row) {
        const segment = row.dataset.part;
        const [label] = STORAGE_CACHE_PARTS[segment];
        this._confirm(row, {
            question: `${t('Clear {what}?', { what: escapeHtml(label.toLowerCase()) })} ${segment === 'older' ? t('Nothing uses them.') : t('They are made again when needed; the photos are untouched.')}`,
            keep: t('Keep them'), confirm: t('Clear'), busy: t('Clearing…'),
            run: () => API.storageClear(segment),
            failed: t('Could not clear it: {error}'),
        });
    }

    // Removing from the data folder says what stays and how it comes back.
    _confirmRemove(row) {
        const part = row.dataset.part;
        const lib = this._libraries.get(part);
        const ask = lib ? {
            question: t('Delete “{name}”? This removes the index and its thumbnails. The photos in {folder} are not touched.', { name: escapeHtml(lib.name), folder: escapeHtml(lib.sourcePath) }),
            keep: t('Keep it'), confirm: t('Delete library'), busy: t('Deleting…'),
            run: async () => {
                await LibraryAPI.delete(lib.id);
                if (App._libraryTab) App._libraryTab._cachedLibs = null;
                App.refreshLibraryVisibility();
            },
            failed: t('Could not delete it: {error}'),
        } : {
            question: STORAGE_REMOVE_QUESTIONS[part],
            keep: t('Keep them'), confirm: t('Remove'), busy: t('Removing…'),
            run: () => API.storageRemove(part),
            failed: t('Could not remove it: {error}'),
        };
        this._confirm(row, ask);
    }

    // _confirm asks in the row, runs on yes and measures again; a failure
    // stays in the row, saying why.
    _confirm(row, { question, keep, confirm, busy, run, failed }) {
        const wrap = row.querySelector('.storage-confirm');
        wrap.innerHTML = `
            <p class="gal-danger-question">${question}</p>
            <div class="gal-danger-actions">
                <button type="button" class="btn btn-sm storage-keep">${keep}</button>
                <button type="button" class="btn btn-sm btn-danger storage-confirm-btn">${confirm}</button>
            </div>`;
        wrap.querySelector('.storage-keep').addEventListener('click', () => { wrap.innerHTML = ''; });
        wrap.querySelector('.storage-confirm-btn').addEventListener('click', async (e) => {
            wrap.querySelectorAll('button').forEach(b => { b.disabled = true; });
            Activity.button(e.currentTarget, busy);
            try {
                await run();
            } catch (err) {
                wrap.innerHTML = `<div class="gal-detail-error">${failed.replace('{error}', escapeHtml(err.message))}</div>`;
                return;
            }
            this.load();
        });
    }
}

const STORAGE_REMOVE_QUESTIONS = {
    recognition: t('Remove the face models? The faces found and the names given stay. New photos are not looked at for faces until the models are downloaded again in People.'),
    things: t('Remove the things models? What was found stays. New photos are not looked at for things until the models are downloaded again in Statistics.'),
    runtime: t('Remove ONNX Runtime? Nothing uses it now. People and Statistics download it again with their models.'),
};
