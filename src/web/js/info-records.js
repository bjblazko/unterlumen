// info-records.js — what is recorded beside a photo, in the info panel: its
// title and fields, which can be edited, and the galleries it is in or
// waits for. The panel's meta context (setMetaContext) holds the entries and
// how to change them.

class InfoRecords {
    constructor(panel) {
        this._panel = panel;
    }

    // The title, from the sidecar or the file itself, marked when it lies
    // beside the file. Without a context to write it, it is only read.
    titleField(ctx, exif) {
        const entry = (ctx?.entries || []).find(e => e.key === 'title');
        const title = entry?.value || (exif?.tags?.Title || '').replace(/^"|"$/g, '');
        const mark = infoSourceMarkHTML(exif, 'Title');
        if (!ctx) {
            return title ? `<div class="info-meta-row info-title-row"><span class="info-label">${t('Title')}</span>${mark}` +
                `<span class="info-meta-val info-value info-title-val">${escapeHtml(title)}</span></div>` : '';
        }
        const val = escapeHtml(title);
        return `<div class="info-meta-row info-title-row">` +
            `<span class="info-label">${t('Title')}</span>` + mark +
            `<span class="info-meta-val info-value info-title-val" contenteditable="true" ` +
                `data-key="title"${!val ? ` data-placeholder="${t('Add title…')}"` : ''}>` +
                val +
            `</span>` +
            (entry ? `<button class="info-meta-del" title="${t('Remove')}" data-key="title">×</button>` : '') +
        `</div>`;
    }

    // The fields a person added, each editable, and a row to add one. A
    // video has none (ADR-0061).
    fieldsSection(ctx) {
        if (ctx.fields === false) return '';
        const fields = (ctx.entries || []).filter(e =>
            e.key !== 'title' && !e.key.startsWith('built:') && !e.key.startsWith('pending:'));
        const rows = fields.map(e =>
            `<div class="info-meta-row" data-meta-key="${escapeHtml(e.key)}">` +
                `<span class="info-label">${escapeHtml(e.key)}</span>` +
                `<span class="info-meta-val info-value" contenteditable="true" data-key="${escapeHtml(e.key)}">${escapeHtml(e.value)}</span>` +
                `<button class="info-meta-del" title="${t('Remove')}" data-key="${escapeHtml(e.key)}">×</button>` +
            `</div>`);
        rows.push(
            `<div class="info-meta-add">` +
                `<input class="info-meta-key-input" type="text" placeholder="${t('Key')}" autocomplete="off">` +
                `<input class="info-meta-val-input" type="text" placeholder="${t('Value')}" autocomplete="off">` +
                `<button class="btn btn-sm info-meta-add-btn">${t('Add')}</button>` +
            `</div>`);
        return this._panel.section('Own fields', rows);
    }

    // The galleries the photo is published in, and those it waits for.
    galleriesSection(ctx) {
        if (!ctx.entries) return '';
        const cards = [
            ...publishedAlbums(ctx.entries).map(a => this._publishedCard(a, ctx)),
            ...pendingAlbums(ctx.entries).map(a => this._pendingCard(a)),
        ];
        return cards.length ? this._panel.section('Galleries', cards) : '';
    }

    // A published gallery is a place, so its card leads there. Only a files
    // destination's gallery can be left from here (the server drops the
    // sidecar record and the exported file too); a gallery or a site would
    // keep the photo on its pages, and the next scan brought it back.
    _publishedCard(a, ctx) {
        const title = a.title ? escapeHtml(a.title) : '';
        const titleHTML = a.postID
            ? `<a class="info-pub-title info-pub-link" href="#galleries" data-slug="${escapeHtml(a.slug)}" data-post="${escapeHtml(a.postID)}">${title || t('Open the gallery')}</a>`
            : (title ? `<div class="info-pub-title">${title}</div>` : '');
        const leave = ctx.filesDestinations?.has(a.slug)
            ? `<button class="info-meta-del" title="${t('Remove from this gallery and delete its exported file')}" aria-label="${t('Remove from this gallery')}" data-key="${escapeHtml(a.key)}">×</button>`
            : '';
        return `<div class="info-pub-card" data-key="${escapeHtml(a.key)}">` +
            `<div class="info-pub-card-header"><span class="info-pub-channel">${escapeHtml(a.channelName)}</span>${leave}</div>` +
            `<div class="info-pub-date">${escapeHtml(this._panel.formatDate(a.date))}</div>` +
            titleHTML +
        `</div>`;
    }

    _pendingCard(p) {
        return `<div class="info-pub-card info-pub-card--pending">` +
            `<div class="info-pub-card-header">` +
                `<span class="info-pub-channel">${escapeHtml(p.channelName)}</span>` +
                `<button class="info-meta-del" title="${t('Remove from pending')}" data-key="${escapeHtml(p.key)}">×</button>` +
            `</div>` +
            `<div class="info-pub-date">${t('pending')}</div>` +
            (p.title ? `<div class="info-pub-title">${escapeHtml(p.title)}</div>` : '') +
        `</div>`;
    }

    attachEvents(container, ctx) {
        container.querySelectorAll('.info-meta-val').forEach(el => this._wireEditable(el, ctx));
        container.querySelectorAll('.info-meta-del').forEach(btn => {
            btn.addEventListener('click', () => this._remove(btn.dataset.key, ctx));
        });
        container.querySelectorAll('.info-pub-link').forEach(link => {
            link.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
                e.preventDefault();
                App.showGallery(link.dataset.slug, link.dataset.post);
            });
        });
        container.querySelector('.info-meta-add-btn')?.addEventListener('click', () => this._add(container, ctx));
    }

    // A title or field lives in the sidecar beside the photo; when that cannot
    // be written nothing is saved, so the old value comes back.
    _wireEditable(el, ctx) {
        let saved = el.textContent;
        el.addEventListener('blur', () => {
            const value = el.textContent.trim();
            if (value === saved) return;
            const previous = saved;
            saved = value;
            ctx.onUpsert(el.dataset.key, value).catch(err => {
                el.textContent = previous;
                saved = previous;
                App.showToast(t('Could not save it: ') + err.message);
            });
        });
        el.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') { e.preventDefault(); el.blur(); }
        });
    }

    // For a gallery's keys the backend removes several related ones (title,
    // account, and for an album its qualified siblings), so the entries are
    // read again instead of filtering out just one.
    async _remove(key, ctx) {
        try {
            await ctx.onDelete(key);
            const publication = key.startsWith('built:') || key.startsWith('pending:');
            ctx.entries = publication && ctx.refresh ? await ctx.refresh() : ctx.entries.filter(e => e.key !== key);
            this._panel.render();
        } catch (err) {
            App.showToast(t('Could not delete it: ') + err.message);
        }
    }

    async _add(container, ctx) {
        const key = container.querySelector('.info-meta-key-input').value.trim();
        const val = container.querySelector('.info-meta-val-input').value.trim();
        if (!key || !val) return;
        try {
            await ctx.onUpsert(key, val);
            ctx.entries = await ctx.refresh();
            this._panel.render();
        } catch (err) {
            App.showToast(t('Could not save it: ') + err.message);
        }
    }
}

// sidecarNotesContext is the meta context of a photo outside a library —
// in Folders, Organize, the viewer: its title and own fields, read and
// written beside it through Metadata's API (ADR-0061), so a library that
// covers it follows. path is relative to the boundary; null when it cannot
// be read.
async function sidecarNotesContext(path) {
    const read = async () => {
        const res = await API.readMetadata([path]);
        const cur = res.files?.[0]?.current;
        if (!cur) throw new Error(res.files?.[0]?.error || t('it could not be read'));
        return cur;
    };
    const entriesOf = (cur) => [
        ...(cur.fields?.title?.value ? [{ key: 'title', value: cur.fields.title.value }] : []),
        ...Object.entries(cur.own || {}).map(([key, value]) => ({ key, value })),
    ];
    const write = async (key, value) => {
        const change = key === 'title' ? { set: { title: value } } : { own: { [key]: { value } } };
        const res = await API.writeMetadata(path, 'sidecar', change);
        if (!res.success) throw new Error(res.error || t('it was not written'));
    };
    let cur;
    try {
        cur = await read();
    } catch {
        return null;
    }
    return {
        entries: entriesOf(cur),
        fields: !cur.video,
        onUpsert: write,
        onDelete: (key) => write(key, ''),
        refresh: async () => entriesOf(await read()),
    };
}

// One card per album, not per channel: a channel can hold many unrelated
// albums, and the unqualified `built:<slug>:title` key only ever remembers
// the most recently published one. Qualified `built:<slug>:<postID>` keys
// are preferred; the unqualified pair is the fallback for photos published
// before those keys existed, and for plain-export channels with no album.
function publishedAlbums(entries) {
    // built:<slug>:title / :postid / :account are field names, not album
    // ids — they also have two segments, so they must be filtered out or
    // they render as albums whose "date" is really a title string.
    const RESERVED = new Set(['title', 'postid', 'account']);
    const titleOf = key => entries.find(e => e.key === key)?.value || '';
    const album = (e, slug, postID, titleKey) => ({
        key: e.key, slug, postID, channelName: humanizeChannelSlug(slug), date: e.value, title: titleOf(titleKey),
    });
    const out = [];
    const covered = new Set();
    for (const e of entries) {
        const parts = e.key.startsWith('built:') ? e.key.slice('built:'.length).split(':') : [];
        if (parts.length !== 2 || RESERVED.has(parts[1])) continue;
        covered.add(parts[0]);
        out.push(album(e, parts[0], parts[1], `built:${parts[0]}:${parts[1]}:title`));
    }
    for (const e of entries) {
        const slug = e.key.startsWith('built:') ? e.key.slice('built:'.length) : ':';
        if (slug.includes(':') || covered.has(slug)) continue;
        out.push(album(e, slug, '', `built:${slug}:title`));
    }
    return out;
}

// Pending keys mirror the published ones: `pending:<slug>:<draftID>` holds
// the album title as its value, so a photo collected into two albums of one
// channel shows two cards.
function pendingAlbums(entries) {
    const out = [];
    const covered = new Set();
    for (const e of entries) {
        const parts = e.key.startsWith('pending:') ? e.key.slice('pending:'.length).split(':') : [];
        if (parts.length !== 2) continue;
        covered.add(parts[0]);
        out.push({ key: e.key, channelName: humanizeChannelSlug(parts[0]), title: e.value });
    }
    for (const e of entries) {
        const slug = e.key.startsWith('pending:') ? e.key.slice('pending:'.length) : ':';
        if (slug.includes(':') || covered.has(slug)) continue;
        out.push({ key: e.key, channelName: humanizeChannelSlug(slug), title: '' });
    }
    return out;
}

function humanizeChannelSlug(slug) {
    return slug.replace(/[-_]/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}
