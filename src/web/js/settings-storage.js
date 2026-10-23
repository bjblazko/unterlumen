// settings-storage.js — Storage in Settings: what Unterlumen keeps on this
// computer, part by part, and what each part is for. The cache is copies,
// made again when needed, so each part of it can be cleared and the whole
// kept within a limit; the data folder is not, so it is only shown, with a
// way to where it is looked after.

const STORAGE_CACHE_PARTS = {
    thumbnails: ['Previews', 'Small versions of the photos for Folders and the grids. Made again as you browse.'],
    images: ['Viewer images', 'Full-size images for the viewer, among them HEIF photos converted to JPEG. Made again when a photo is opened.'],
    older: ['Left by older versions', 'Nothing reads these any more; they go at the next start, or now.'],
};

const STORAGE_LIMITS = [[2, '2 GB'], [5, '5 GB'], [10, '10 GB'], [20, '20 GB'], [50, '50 GB'], [0, 'No limit']];

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
            this.el.innerHTML = `<span class="form-label">Storage</span><span class="gal-detail-error">Could not measure the storage: ${escapeHtml(err.message)}</span>`;
            return;
        }
        const parts = s.cache.segments.filter(p => p.id !== 'older' || p.bytes > 0);
        const data = s.data ? this._dataRows(s.data) : [];
        const most = Math.max(1, ...parts.map(p => p.bytes), ...data.map(d => d.bytes));
        const total = parts.reduce((n, p) => n + p.bytes, 0);
        this.el.innerHTML = `
            <span class="form-label">Storage</span>
            <span class="form-hint">What Unterlumen keeps on this computer besides your photos, and what each part is for.</span>
            <div class="storage-group">
                <div class="storage-head">
                    <h3 class="storage-title">Cache</h3>
                    <span class="storage-size">${readableBytes(total)}</span>
                    ${this._reveal(s, 'cache')}
                </div>
                <span class="settings-cache settings-cache-path">${escapeHtml(s.cache.path)}</span>
                ${parts.map(p => this._row(STORAGE_CACHE_PARTS[p.id][0], STORAGE_CACHE_PARTS[p.id][1], p.bytes, most,
                    p.bytes ? `<button type="button" class="btn btn-sm storage-clear" data-segment="${p.id}">Clear</button>` : '', p.id)).join('')}
                <label class="storage-limit">Keep the cache within
                    <select class="btn btn-sm select-btn" id="storage-limit">${this._limitOptions(s.cache.limit)}</select>
                </label>
                <span class="form-hint">When it grows past that, the files used least recently go.</span>
            </div>
            ${s.data ? `
            <div class="storage-group">
                <div class="storage-head">
                    <h3 class="storage-title">Data folder</h3>
                    <span class="storage-size">${readableBytes(data.reduce((n, d) => n + d.bytes, 0))}</span>
                    ${this._reveal(s, 'data')}
                </div>
                <span class="settings-cache settings-cache-path">${escapeHtml(s.data.path)}</span>
                <span class="form-hint">Not copies: a library's thumbnails say where its faces are, its index what was found and given. They are looked after where they belong.</span>
                ${data.map(d => this._row(d.label, d.hint, d.bytes, most, d.link)).join('')}
            </div>` : ''}`;
        this._wire();
    }

    _dataRows(data) {
        const rows = data.libraries.map(l => ({
            label: l.name, bytes: l.bytes,
            hint: 'Thumbnails, faces and the index of this library.',
            link: placeLink('library', 'libraries', 'Libraries'),
        }));
        if (data.recognition) rows.push({ label: 'Face recognition', bytes: data.recognition, hint: 'The two face models.', link: placeLink('people', 'people', 'People') });
        if (data.things) rows.push({ label: 'Things', bytes: data.things, hint: 'CLIP and the object detector.', link: placeLink('statistics', 'statistics', 'Statistics') });
        if (data.runtime) rows.push({ label: 'ONNX Runtime', bytes: data.runtime, hint: 'Runs the face and things models.', link: '' });
        if (data.output) rows.push({ label: 'Destination output', bytes: data.output, hint: 'What destinations built on this computer, ready to publish.', link: placeLink('destinations', 'destinations', 'Destinations') });
        return rows.sort((a, b) => b.bytes - a.bytes);
    }

    _row(label, hint, bytes, most, action, segment = '') {
        const width = bytes ? Math.max(0.5, 100 * bytes / most) : 0;
        return `
            <div class="storage-row"${segment ? ` data-segment="${segment}"` : ''}>
                <span class="storage-label">${escapeHtml(label)}</span>
                <span class="storage-bar" aria-hidden="true"><span style="width:${width.toFixed(1)}%"></span></span>
                <span class="storage-size">${readableBytes(bytes)}</span>
                <span class="storage-action">${action}</span>
                <span class="storage-hint">${escapeHtml(hint)}</span>
                <div class="storage-confirm"></div>
            </div>`;
    }

    _reveal(s, folder) {
        if (!s.canReveal) return '';
        const where = /Mac/.test(navigator.platform) ? 'Finder' : 'file manager';
        return `<button type="button" class="btn btn-sm storage-reveal" data-folder="${folder}">Show in ${where}</button>`;
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
                App.showToast(`The limit was not changed: ${err.message}`);
            }
        });
    }

    // Clearing costs nothing but time, so the question is short — but it is
    // still asked, and asked here rather than in a browser box.
    _confirmClear(row) {
        const segment = row.dataset.segment;
        const [label] = STORAGE_CACHE_PARTS[segment];
        const wrap = row.querySelector('.storage-confirm');
        wrap.innerHTML = `
            <p class="gal-danger-question">Clear ${escapeHtml(label.toLowerCase())}? ${segment === 'older' ? 'Nothing uses them.' : 'They are made again when needed; the photos are untouched.'}</p>
            <div class="gal-danger-actions">
                <button type="button" class="btn btn-sm storage-keep">Keep them</button>
                <button type="button" class="btn btn-sm btn-danger storage-confirm-btn">Clear</button>
            </div>`;
        wrap.querySelector('.storage-keep').addEventListener('click', () => { wrap.innerHTML = ''; });
        wrap.querySelector('.storage-confirm-btn').addEventListener('click', async (e) => {
            wrap.querySelectorAll('button').forEach(b => { b.disabled = true; });
            Activity.button(e.currentTarget, 'Clearing…');
            try {
                await API.storageClear(segment);
            } catch (err) {
                wrap.innerHTML = `<div class="gal-detail-error">Could not clear it: ${escapeHtml(err.message)}</div>`;
                return;
            }
            this.load();
        });
    }
}
