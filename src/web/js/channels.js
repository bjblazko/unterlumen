// channels.js — Channel API and management UI

/* --- ChannelAPI --- */

const ChannelAPI = {
    async list() {
        const r = await fetch('/api/channels/');
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async create(ch) {
        const r = await fetch('/api/channels/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(ch),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async update(slug, ch) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(ch),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async delete(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
    },
};

/* --- ChannelSettingsModal --- */

class ChannelSettingsModal {
    constructor() {
        this._el = null;
    }

    open() {
        this._el = document.createElement('div');
        this._el.className = 'modal-backdrop';
        this._el.innerHTML = `
            <div class="modal channel-settings-modal">
                <div class="modal-header">
                    <span class="modal-title">Channels</span>
                    <button class="modal-close" id="ch-close">&times;</button>
                </div>
                <div class="modal-body" id="ch-list-body">
                    <div class="channel-loading">Loading…</div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-accent" id="ch-add-btn">Add channel</button>
                </div>
            </div>`;
        document.body.appendChild(this._el);

        this._el.querySelector('#ch-close').addEventListener('click', () => this.close());
        this._el.querySelector('#ch-add-btn').addEventListener('click', () => this._openForm(null));
        this._el.addEventListener('click', e => { if (e.target === this._el) this.close(); });

        this._load();
    }

    close() {
        this._el?.remove();
        this._el = null;
    }

    async _load() {
        const body = this._el.querySelector('#ch-list-body');
        try {
            const channels = await ChannelAPI.list();
            body.innerHTML = '';
            if (channels.length === 0) {
                body.innerHTML = '<div class="channel-empty">No channels yet.</div>';
                return;
            }
            for (const ch of channels) {
                body.appendChild(this._row(ch));
            }
        } catch (err) {
            body.innerHTML = `<div class="channel-error">Failed to load: ${escapeHtml(err.message)}</div>`;
        }
    }

    _row(ch) {
        const row = document.createElement('div');
        row.className = 'channel-row';
        const scaleDesc = _scaleDesc(ch.scale);
        row.innerHTML = `
            <div class="channel-row-info">
                <span class="channel-row-name">${escapeHtml(ch.name)}</span>
                <span class="channel-row-slug">${escapeHtml(ch.slug)}</span>
                <span class="channel-row-detail">${escapeHtml(ch.format.toUpperCase())} · q${ch.quality} · ${escapeHtml(scaleDesc)} · ${escapeHtml(ch.exifMode)}</span>
            </div>
            <div class="channel-row-actions">
                <button class="btn btn-sm ch-edit">Edit</button>
                <button class="btn btn-sm ch-delete">Delete</button>
            </div>`;
        row.querySelector('.ch-edit').addEventListener('click', () => this._openForm(ch));
        row.querySelector('.ch-delete').addEventListener('click', () => this._deleteChannel(ch, row));
        return row;
    }

    async _deleteChannel(ch, row) {
        if (!confirm(`Delete channel "${ch.name}"?`)) return;
        try {
            await ChannelAPI.delete(ch.slug);
            row.remove();
        } catch (err) {
            alert('Delete failed: ' + err.message);
        }
    }

    _openForm(existing) {
        const isNew = !existing;
        const ch = existing || { slug: '', name: '', format: 'jpeg', quality: 85, exifMode: 'keep_no_gps', scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 1920 } };

        const form = document.createElement('div');
        form.className = 'modal-backdrop';
        form.innerHTML = `
            <div class="modal channel-form-modal">
                <div class="modal-header">
                    <span class="modal-title">${isNew ? 'New Channel' : 'Edit Channel'}</span>
                    <button class="modal-close" id="chf-close">&times;</button>
                </div>
                <div class="modal-body">
                    ${isNew ? `
                    <label class="form-label">Slug <span class="form-hint">(immutable after save)</span></label>
                    <input class="form-input" id="chf-slug" value="${escapeHtml(ch.slug)}" placeholder="my-channel" autocomplete="off">
                    ` : `
                    <div class="form-slug-display">Slug: <strong>${escapeHtml(ch.slug)}</strong></div>
                    `}
                    <label class="form-label">Name</label>
                    <input class="form-input" id="chf-name" value="${escapeHtml(ch.name)}" placeholder="My Channel">
                    <label class="form-label">Format</label>
                    <select class="form-select" id="chf-format">
                        <option value="jpeg" ${ch.format==='jpeg'?'selected':''}>JPEG</option>
                        <option value="png" ${ch.format==='png'?'selected':''}>PNG</option>
                        <option value="webp" ${ch.format==='webp'?'selected':''}>WebP</option>
                    </select>
                    <label class="form-label">Quality (1–100)</label>
                    <input class="form-input" id="chf-quality" type="number" min="1" max="100" value="${ch.quality}">
                    <label class="form-label">Scale</label>
                    <div class="form-row">
                        <select class="form-select" id="chf-scale-mode">
                            <option value="none" ${ch.scale?.mode==='none'?'selected':''}>None (original size)</option>
                            <option value="max_dim" ${ch.scale?.mode==='max_dim'?'selected':''}>Max dimension</option>
                            <option value="percent" ${ch.scale?.mode==='percent'?'selected':''}>Percent</option>
                        </select>
                        <div id="chf-scale-opts" class="form-scale-opts">${_scaleOptsHTML(ch.scale)}</div>
                    </div>
                    <label class="form-label">EXIF</label>
                    <select class="form-select" id="chf-exif">
                        <option value="strip" ${ch.exifMode==='strip'?'selected':''}>Strip all</option>
                        <option value="keep_no_gps" ${ch.exifMode==='keep_no_gps'?'selected':''}>Keep (no GPS)</option>
                        <option value="keep" ${ch.exifMode==='keep'?'selected':''}>Keep all</option>
                    </select>
                </div>
                <div class="modal-footer">
                    <div class="channel-form-error" id="chf-error" style="display:none"></div>
                    <button class="btn" id="chf-cancel">Cancel</button>
                    <button class="btn btn-accent" id="chf-save">Save</button>
                </div>
            </div>`;
        document.body.appendChild(form);

        const scaleMode = form.querySelector('#chf-scale-mode');
        const scaleOpts = form.querySelector('#chf-scale-opts');
        scaleMode.addEventListener('change', () => {
            scaleOpts.innerHTML = _scaleOptsHTML({ mode: scaleMode.value });
        });

        form.querySelector('#chf-close').addEventListener('click', () => form.remove());
        form.querySelector('#chf-cancel').addEventListener('click', () => form.remove());
        form.addEventListener('click', e => { if (e.target === form) form.remove(); });

        // Auto-derive slug from name for new channels
        if (isNew) {
            const nameEl = form.querySelector('#chf-name');
            const slugEl = form.querySelector('#chf-slug');
            nameEl.addEventListener('input', () => {
                if (!slugEl._touched) {
                    slugEl.value = nameEl.value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
                }
            });
            slugEl.addEventListener('input', () => { slugEl._touched = true; });
        }

        form.querySelector('#chf-save').addEventListener('click', async () => {
            const errEl = form.querySelector('#chf-error');
            const slug = isNew ? form.querySelector('#chf-slug').value.trim() : ch.slug;
            const name = form.querySelector('#chf-name').value.trim();
            const format = form.querySelector('#chf-format').value;
            const quality = parseInt(form.querySelector('#chf-quality').value, 10);
            const exifMode = form.querySelector('#chf-exif').value;
            const scale = _readScaleOpts(form);

            if (!name || (isNew && !slug)) {
                errEl.textContent = 'Name and slug are required.';
                errEl.style.display = '';
                return;
            }

            const payload = { slug, name, format, quality, exifMode, scale };
            try {
                if (isNew) {
                    await ChannelAPI.create(payload);
                } else {
                    await ChannelAPI.update(slug, payload);
                }
                form.remove();
                this._load(); // refresh list
            } catch (err) {
                errEl.textContent = err.message;
                errEl.style.display = '';
            }
        });
    }
}

/* --- Scale helpers --- */

function _scaleDesc(scale) {
    if (!scale || scale.mode === 'none' || !scale.mode) return 'original size';
    if (scale.mode === 'max_dim') return `max ${scale.maxDimension || 'width'} ${scale.maxValue}px`;
    if (scale.mode === 'percent') return `${scale.percent}%`;
    if (scale.mode === 'pixels') return `${scale.width}×${scale.height}px`;
    return scale.mode;
}

function _scaleOptsHTML(scale) {
    const mode = scale?.mode || 'none';
    if (mode === 'max_dim') {
        const dim = scale?.maxDimension || 'width';
        const val = scale?.maxValue || 1920;
        return `<select class="form-select" id="chf-max-dim">
            <option value="width" ${dim==='width'?'selected':''}>Width</option>
            <option value="height" ${dim==='height'?'selected':''}>Height</option>
        </select>
        <input class="form-input" id="chf-max-val" type="number" min="1" value="${val}" placeholder="px">`;
    }
    if (mode === 'percent') {
        const pct = scale?.percent || 50;
        return `<input class="form-input" id="chf-percent" type="number" min="1" max="200" value="${pct}" placeholder="%">`;
    }
    return '';
}

function _readScaleOpts(form) {
    const mode = form.querySelector('#chf-scale-mode').value;
    if (mode === 'max_dim') {
        const dimEl = form.querySelector('#chf-max-dim');
        const valEl = form.querySelector('#chf-max-val');
        return { mode, maxDimension: dimEl?.value || 'width', maxValue: parseInt(valEl?.value || '1920', 10) };
    }
    if (mode === 'percent') {
        const pctEl = form.querySelector('#chf-percent');
        return { mode, percent: parseFloat(pctEl?.value || '50') };
    }
    return { mode: 'none' };
}
