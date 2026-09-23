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
    async rebuildSite(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/rebuild-site`, { method: 'POST' });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async rebuildGalleries(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/rebuild-galleries`, { method: 'POST' });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async path(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/path`);
        if (!r.ok) throw new Error(await r.text());
        return (await r.json()).path;
    },
    async reveal(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/reveal`, { method: 'POST' });
        if (!r.ok) throw new Error(await r.text());
    },
    async galleries(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/galleries`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async avatarStatus(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/avatar`);
        if (!r.ok) return { exists: false };
        return r.json();
    },
    async uploadAvatar(slug, file) {
        const fd = new FormData();
        fd.append('file', file);
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/avatar`, { method: 'POST', body: fd });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async deleteAvatar(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/avatar`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
    },
    async logoStatus(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/logo`);
        if (!r.ok) return { exists: false };
        return r.json();
    },
    async uploadLogo(slug, file) {
        const fd = new FormData();
        fd.append('file', file);
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/logo`, { method: 'POST', body: fd });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async deleteLogo(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/logo`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
    },
    // deployTest and deploy return their JSON body ({ok, error} / {ok, output, error})
    // even when the remote operation itself failed — only a 4xx (bad request, e.g.
    // "channel not found" or "handler is not rsync") throws.
    async deployTest(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/deploy/test`, { method: 'POST' });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async deploy(slug) {
        const r = await fetch(`/api/channels/${encodeURIComponent(slug)}/deploy`, { method: 'POST' });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async listDrafts(slug) {
        const r = await fetch(`/api/channels/${slug}/drafts`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async deleteDraft(slug, draftID) {
        const r = await fetch(`/api/channels/${slug}/drafts/${draftID}`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
    },
    async removeDraftPhoto(slug, draftID, libID, photoID) {
        const r = await fetch(`/api/channels/${slug}/drafts/${draftID}/photos/${libID}/${photoID}`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
        return r.status === 204 ? null : r.json();
    },
    async generateStream(slug, draftID, postID, { publishedAt } = {}, onProgress) {
        const qs = draftID === '-' ? `?postID=${encodeURIComponent(postID)}` : '';
        const r = await fetch(`/api/channels/${slug}/drafts/${draftID}/generate${qs}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ publishedAt }),
        });
        if (!r.ok) throw new Error(await r.text());
        if (!r.headers.get('content-type')?.includes('text/event-stream')) {
            return r.json();
        }
        const reader = r.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let finalEvt = null;
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });
            const blocks = buffer.split('\n\n');
            buffer = blocks.pop() ?? '';
            for (const block of blocks) {
                const line = block.split('\n').find(l => l.startsWith('data: '));
                if (!line) continue;
                try {
                    const evt = JSON.parse(line.slice(6));
                    if (evt.complete) finalEvt = evt;
                    else if (onProgress) onProgress(evt);
                } catch { /* skip malformed */ }
            }
        }
        if (!finalEvt) throw new Error('Generate stream ended without completion event');
        return finalEvt;
    },
};

/* --- Deploy URL helpers ---
 * A channel's public base URL comes from its Base URL setting, available for
 * both site-export and single-gallery channels. When it is empty but an rsync
 * handler is configured, that handler's Host is, in practice, almost always
 * the same domain the content is served from — so fall back to it as a
 * labelled best-effort guess rather than showing nothing. */

function _deployBaseURL(ch) {
    if (ch.siteURL) return { url: ch.siteURL.replace(/\/$/, ''), guessed: false };
    if (ch.handler === 'rsync' && ch.handlerConfig?.host) return { url: 'https://' + ch.handlerConfig.host, guessed: true };
    return null;
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

// Close any open path dropdown on outside click
document.addEventListener('click', () => {
    document.querySelectorAll('.ch-path-menu').forEach(m => { m.hidden = true; });
});

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
