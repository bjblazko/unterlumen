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
    async rebuildSite(libID, slug) {
        const r = await fetch(`/api/library/${libID}/channels/${encodeURIComponent(slug)}/rebuild-site`, {
            method: 'POST',
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
};

/* --- ChannelSettingsModal --- */

class ChannelSettingsModal {
    constructor() {
        this._el = null;
    }

    open(libID) {
        this._libID = libID;
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
        const accountCount = (ch.accounts || []).length;
        const handlerDesc = ch.handler ? ` · handler: ${ch.handler}` : '';
        const accountDesc = accountCount > 0 ? ` · ${accountCount} account${accountCount !== 1 ? 's' : ''}` : '';
        row.innerHTML = `
            <div class="channel-row-info">
                <span class="channel-row-name">${escapeHtml(ch.name)}</span>
                <span class="channel-row-slug">${escapeHtml(ch.slug)}</span>
                <span class="channel-row-detail">${escapeHtml(ch.format.toUpperCase())} · q${ch.quality} · ${escapeHtml(scaleDesc)} · ${escapeHtml(ch.exifMode)}${escapeHtml(handlerDesc)}${escapeHtml(accountDesc)}</span>
            </div>
            <div class="channel-row-actions">
                ${ch.siteExport ? '<button class="btn btn-sm ch-rebuild">Rebuild site</button>' : ''}
                <button class="btn btn-sm ch-edit">Edit</button>
                <button class="btn btn-sm ch-delete">Delete</button>
            </div>`;
        row.querySelector('.ch-edit').addEventListener('click', () => this._openForm(ch));
        row.querySelector('.ch-delete').addEventListener('click', () => this._deleteChannel(ch, row));
        if (ch.siteExport) {
            const rebuildBtn = row.querySelector('.ch-rebuild');
            rebuildBtn.addEventListener('click', () => this._rebuildSite(ch, rebuildBtn));
        }
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

    async _rebuildSite(ch, btn) {
        if (!this._libID) {
            alert('Open Channels from within a library to use Rebuild site.');
            return;
        }
        const orig = btn.textContent;
        btn.disabled = true;
        btn.textContent = 'Rebuilding…';
        try {
            const res = await ChannelAPI.rebuildSite(this._libID, ch.slug);
            btn.textContent = `Done (${res.albumCount})`;
            setTimeout(() => { btn.textContent = orig; btn.disabled = false; }, 2500);
        } catch (err) {
            btn.textContent = 'Failed';
            setTimeout(() => { btn.textContent = orig; btn.disabled = false; }, 2500);
            alert('Rebuild failed: ' + err.message);
        }
    }

    _openForm(existing) {
        const isNew = !existing;
        const ch = existing || {
            slug: '', name: '', format: 'jpeg', quality: 85, exifMode: 'keep_no_gps',
            scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 1920 },
            handler: '', handlerConfig: {}, accounts: [],
            galleryExport: false, siteExport: false, siteTitle: '', siteTheme: 'light',
        };

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
                        <option value="png"  ${ch.format==='png'?'selected':''}>PNG</option>
                        <option value="webp" ${ch.format==='webp'?'selected':''}>WebP</option>
                    </select>
                    <label class="form-label">Quality (1–100)</label>
                    <input class="form-input" id="chf-quality" type="number" min="1" max="100" value="${ch.quality}">
                    <label class="form-label">Scale</label>
                    <div class="form-row">
                        <select class="form-select" id="chf-scale-mode">
                            <option value="none"    ${ch.scale?.mode==='none'?'selected':''}>None (original size)</option>
                            <option value="max_dim" ${ch.scale?.mode==='max_dim'?'selected':''}>Max dimension</option>
                            <option value="percent" ${ch.scale?.mode==='percent'?'selected':''}>Percent</option>
                        </select>
                        <div id="chf-scale-opts" class="form-scale-opts">${_scaleOptsHTML(ch.scale)}</div>
                    </div>
                    <label class="form-label">EXIF</label>
                    <select class="form-select" id="chf-exif">
                        <option value="strip"       ${ch.exifMode==='strip'?'selected':''}>Strip all</option>
                        <option value="keep_no_gps" ${ch.exifMode==='keep_no_gps'?'selected':''}>Keep (no GPS)</option>
                        <option value="keep"        ${ch.exifMode==='keep'?'selected':''}>Keep all</option>
                    </select>

                    <label class="form-label">Export mode</label>
                    <select class="form-select" id="chf-export-mode">
                        <option value="standard" ${!ch.galleryExport && !ch.siteExport ? 'selected' : ''}>Standard — files only</option>
                        <option value="gallery"  ${ch.galleryExport && !ch.siteExport  ? 'selected' : ''}>Single gallery — index.html per publish</option>
                        <option value="site"     ${ch.siteExport                       ? 'selected' : ''}>Multi-album site — static website</option>
                    </select>
                    <div id="chf-site-opts" style="display:${ch.siteExport ? '' : 'none'}">
                        <label class="form-label">Site title <span class="form-hint">(shown on the root index page)</span></label>
                        <input class="form-input" id="chf-site-title" value="${escapeHtml(ch.siteTitle || '')}" placeholder="e.g. My Photography">
                        <label class="form-label">Default theme <span class="form-hint">(visitors can switch; this is the initial choice)</span></label>
                        <select class="form-select" id="chf-site-theme">
                            <option value="light" ${(ch.siteTheme || 'light') === 'light' ? 'selected' : ''}>Light</option>
                            <option value="dark"  ${ch.siteTheme === 'dark'              ? 'selected' : ''}>Dark</option>
                        </select>
                    </div>

                    <label class="form-label">Handler <span class="form-hint">(optional, for future upload automation)</span></label>
                    <input class="form-input" id="chf-handler" value="${escapeHtml(ch.handler || '')}" placeholder="e.g. mastodon, scp">

                    <label class="form-label">Handler config <span class="form-hint">(key → value)</span></label>
                    <div id="chf-hconfig" class="kv-editor">${_kvEditorHTML(ch.handlerConfig || {})}</div>
                    <button class="btn btn-sm" id="chf-hconfig-add" style="align-self:flex-start">+ Add config entry</button>

                    <label class="form-label">Accounts <span class="form-hint">(named sub-accounts, e.g. two Mastodon logins)</span></label>
                    <div id="chf-accounts" class="accounts-editor">${_accountsEditorHTML(ch.accounts || [])}</div>
                    <button class="btn btn-sm" id="chf-account-add" style="align-self:flex-start">+ Add account</button>
                </div>
                <div class="modal-footer">
                    <div class="channel-form-error" id="chf-error" style="display:none"></div>
                    <button class="btn" id="chf-cancel">Cancel</button>
                    <button class="btn btn-accent" id="chf-save">Save</button>
                </div>
            </div>`;
        document.body.appendChild(form);

        // Scale mode toggle
        const scaleMode = form.querySelector('#chf-scale-mode');
        const scaleOpts = form.querySelector('#chf-scale-opts');
        scaleMode.addEventListener('change', () => {
            scaleOpts.innerHTML = _scaleOptsHTML({ mode: scaleMode.value });
        });

        // Export mode toggle
        const exportModeEl = form.querySelector('#chf-export-mode');
        const siteOptsEl   = form.querySelector('#chf-site-opts');
        exportModeEl.addEventListener('change', () => {
            siteOptsEl.style.display = exportModeEl.value === 'site' ? '' : 'none';
        });

        // Handler config add row
        form.querySelector('#chf-hconfig-add').addEventListener('click', () => {
            const ed = form.querySelector('#chf-hconfig');
            ed.insertAdjacentHTML('beforeend', _kvRowHTML('', ''));
        });

        // Account add
        form.querySelector('#chf-account-add').addEventListener('click', () => {
            form.querySelector('#chf-accounts').insertAdjacentHTML('beforeend', _accountRowHTML({ id: '', label: '', config: {} }));
        });

        // Close / cancel
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
            if (!name || (isNew && !slug)) {
                errEl.textContent = 'Name and slug are required.';
                errEl.style.display = '';
                return;
            }
            const exportModeVal = form.querySelector('#chf-export-mode').value;
            const payload = {
                slug,
                name,
                format:        form.querySelector('#chf-format').value,
                quality:       parseInt(form.querySelector('#chf-quality').value, 10),
                exifMode:      form.querySelector('#chf-exif').value,
                scale:         _readScaleOpts(form),
                galleryExport: exportModeVal === 'gallery' ? true : undefined,
                siteExport:    exportModeVal === 'site'    ? true : undefined,
                siteTitle:     exportModeVal === 'site'    ? (form.querySelector('#chf-site-title').value.trim() || undefined) : undefined,
                siteTheme:     exportModeVal === 'site'    ? (form.querySelector('#chf-site-theme').value || undefined) : undefined,
                handler:       form.querySelector('#chf-handler').value.trim() || undefined,
                handlerConfig: _readKVEditor(form.querySelector('#chf-hconfig')) || undefined,
                accounts:      _readAccountsEditor(form.querySelector('#chf-accounts')),
            };
            try {
                if (isNew) {
                    await ChannelAPI.create(payload);
                } else {
                    await ChannelAPI.update(slug, payload);
                }
                form.remove();
                this._load();
            } catch (err) {
                errEl.textContent = err.message;
                errEl.style.display = '';
            }
        });
    }
}

/* --- KV editor helpers --- */

function _kvEditorHTML(obj) {
    return Object.entries(obj).map(([k, v]) => _kvRowHTML(k, v)).join('');
}

function _kvRowHTML(k, v) {
    return `<div class="kv-row">
        <input class="form-input kv-key"   value="${escapeHtml(k)}" placeholder="key">
        <input class="form-input kv-value" value="${escapeHtml(v)}" placeholder="value">
        <button class="btn btn-sm kv-del" title="Remove">&times;</button>
    </div>`;
}

function _readKVEditor(el) {
    const obj = {};
    el.querySelectorAll('.kv-row').forEach(row => {
        const k = row.querySelector('.kv-key').value.trim();
        const v = row.querySelector('.kv-value').value;
        if (k) obj[k] = v;
    });
    return Object.keys(obj).length ? obj : null;
}

// Delegate kv-del clicks via parent (handles dynamically added rows)
document.addEventListener('click', e => {
    if (e.target.classList.contains('kv-del')) {
        e.target.closest('.kv-row')?.remove();
    }
    if (e.target.classList.contains('account-del')) {
        e.target.closest('.account-row')?.remove();
    }
    if (e.target.classList.contains('account-kv-add')) {
        e.target.previousElementSibling?.insertAdjacentHTML('beforeend', _kvRowHTML('', ''));
    }
});

/* --- Accounts editor helpers --- */

function _accountsEditorHTML(accounts) {
    return accounts.map(a => _accountRowHTML(a)).join('');
}

function _accountRowHTML(a) {
    return `<div class="account-row">
        <div class="account-row-header">
            <input class="form-input account-id"    value="${escapeHtml(a.id || '')}"    placeholder="ID (e.g. personal)">
            <input class="form-input account-label" value="${escapeHtml(a.label || '')}" placeholder="Label (e.g. Personal)">
            <button class="btn btn-sm account-del" title="Remove account">&times;</button>
        </div>
        <div class="kv-editor account-config">${_kvEditorHTML(a.config || {})}</div>
        <button class="btn btn-sm account-kv-add">+ Add config</button>
    </div>`;
}

function _readAccountsEditor(el) {
    const accounts = [];
    el.querySelectorAll('.account-row').forEach(row => {
        const id    = row.querySelector('.account-id').value.trim();
        const label = row.querySelector('.account-label').value.trim();
        const config = _readKVEditor(row.querySelector('.account-config'));
        if (id) accounts.push({ id, label, ...(config ? { config } : {}) });
    });
    return accounts;
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
            <option value="width"  ${dim==='width'?'selected':''}>Width</option>
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
