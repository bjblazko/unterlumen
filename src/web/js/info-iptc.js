// info-iptc.js — the IPTC fields of the photo shown, in the info panel
// (ADR-0061): where it was taken, its event and headline, origin and rights,
// and who and what is shown — the filled ones, each editable in place, and
// any other added from "Add a field…". A change is written to the sidecar
// (a video's NFO file), like the stars, so the file itself stays as it is.

const INFO_IPTC_GROUPS = ['location', 'event', 'origin', 'shown'];

class InfoIPTC {
    constructor(panel) {
        this._panel = panel;
    }

    _groups() {
        return META_GROUPS.filter(g => INFO_IPTC_GROUPS.includes(g.id));
    }

    render(d, tags, used) {
        if (!d) return '';
        const rows = [];
        const empty = [];
        for (const g of this._groups()) {
            const filled = g.fields.filter(f => { used.add(f.tag); return tags[f.tag]; });
            empty.push(...g.fields.filter(f => !tags[f.tag]));
            if (!filled.length) continue;
            rows.push(`<div class="info-iptc-group">${escapeHtml(t(g.title))}</div>`);
            rows.push(...filled.map(f => this._rowHTML(f, tags[f.tag])));
        }
        if (!rows.length) rows.push(`<div class="info-row"><span class="info-value info-iptc-none">${t('None yet.')}</span></div>`);
        rows.push(this._addHTML(empty));
        return this._panel.section('IPTC', rows);
    }

    _rowHTML(f, value) {
        const mono = f.kind === 'mono' ? ' info-iptc-mono' : '';
        return `<div class="info-row info-iptc-row" data-iptc="${f.field}">` +
            `<span class="info-label">${escapeHtml(t(f.label))}</span>` +
            `<span class="info-value info-iptc-val${mono}" contenteditable="true" data-field="${f.field}">${escapeHtml(String(value).replace(/^"|"$/g, ''))}</span>` +
            `<button class="info-meta-del info-iptc-del" title="${t('Remove')}" aria-label="${escapeHtml(t('Remove {field}', { field: t(f.label) }))}" data-field="${f.field}">×</button>` +
        `</div>`;
    }

    _addHTML(empty) {
        if (!empty.length) return '';
        const options = this._groups().map(g => {
            const own = g.fields.filter(f => empty.includes(f));
            if (!own.length) return '';
            return `<optgroup label="${escapeHtml(t(g.title))}">${own.map(f => `<option value="${f.field}">${escapeHtml(t(f.label))}</option>`).join('')}</optgroup>`;
        }).join('');
        return `<div class="info-meta-add info-iptc-add">` +
            `<select class="info-iptc-field" aria-label="${escapeHtml(t('Field'))}"><option value="">${t('Add a field…')}</option>${options}</select>` +
            `<input class="info-iptc-input" type="text" placeholder="${t('Value')}" aria-label="${escapeHtml(t('Value'))}" autocomplete="off">` +
            `<button class="btn btn-sm info-iptc-add-btn">${t('Add')}</button>` +
        `</div>`;
    }

    attachEvents(container) {
        container.querySelectorAll('.info-iptc-val').forEach(el => this._wireEditable(el));
        container.querySelectorAll('.info-iptc-del').forEach(btn => {
            btn.addEventListener('click', () => this._write(btn.dataset.field, ''));
        });
        const add = container.querySelector('.info-iptc-add-btn');
        add?.addEventListener('click', () => {
            const field = container.querySelector('.info-iptc-field').value;
            const value = container.querySelector('.info-iptc-input').value.trim();
            if (field && value) this._write(field, value, { added: true });
        });
    }

    _wireEditable(el) {
        let saved = el.textContent;
        el.addEventListener('blur', () => {
            const value = el.textContent.trim();
            if (value === saved) return;
            saved = value;
            this._write(el.dataset.field, value);
        });
        el.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') { e.preventDefault(); el.blur(); }
        });
    }

    // _write writes one field beside the photo shown and shows it as read
    // back, so what the panel says is what the sidecar holds.
    async _write(field, value, { added = false } = {}) {
        const p = this._panel;
        const key = p.currentPath;
        const files = p.boundaryFile();
        if (!files) { App._outsideBoundary(); return; }
        let res;
        try {
            res = await API.writeMetadata(files[0], 'sidecar', { set: { [field]: value } });
            if (!res.success) throw new Error(res.error || t('it was not written'));
        } catch (err) {
            App.showToast(t('Could not save it: ') + err.message);
            p.render();
            return;
        }
        if (p.currentPath !== key || !p.data) return;
        const tag = META_GROUPS.flatMap(g => g.fields || []).find(f => f.field === field).tag;
        const after = res.after?.fields?.[field]?.value || '';
        p.data.exif = p.data.exif || { tags: {} };
        p.data.exif.tags = { ...(p.data.exif.tags || {}), [tag]: after };
        if (!after) delete p.data.exif.tags[tag];
        // An edit in place may be followed at once by typing elsewhere in
        // the panel, which a render would wipe; an added field is done.
        if (added) p.render();
        else p._renderWhenIdle();
    }
}
