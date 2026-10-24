// destinations.js — Destinations: where galleries go and how their files are
// made (ADR-0029's vocabulary; the backend calls them channels).
//
// This used to be a modal reachable only from an opened library, with a form
// that showed every field for every type — slug, handler key/values and
// accounts included. It is now a place of its own, and the form asks for the
// type first and then shows only what that type needs.

// The three kinds of destination, in the words a person would use. Each maps
// onto the backend's existing flags; nothing about the storage changed.
const DESTINATION_TYPES = [
    {
        id: 'share',
        label: t('Share links'),
        hint: t('Unrelated galleries, each under its own unguessable link. For family and friends.'),
        matches: ch => !!ch.galleryExport && !ch.siteExport,
    },
    {
        id: 'site',
        label: t('Website'),
        hint: t('Related albums with an index page, an about page and a sitemap.'),
        matches: ch => !!ch.siteExport,
    },
    {
        id: 'files',
        label: t('Files'),
        hint: t('Image files in a folder, sized for a platform. You post them yourself.'),
        matches: ch => !ch.galleryExport && !ch.siteExport,
    },
];

function destinationType(ch) {
    return (DESTINATION_TYPES.find(dt => dt.matches(ch)) || DESTINATION_TYPES[2]).id;
}

function destinationTypeLabel(ch) {
    return t(DESTINATION_TYPES.find(dt => dt.id === destinationType(ch)).label);
}

function slugify(name) {
    return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

class DestinationsPane {
    constructor(container) {
        this.container = container;
        this._editing = undefined; // undefined = list, null = new, object = edit
    }

    render() {
        this._load();
    }

    // Opens one destination's form directly — the Galleries screen links here.
    async openBySlug(slug) {
        if (!this._channels) await this._load();
        const ch = (this._channels || []).find(c => c.slug === slug);
        if (!ch) return;
        this._editing = ch;
        this._renderForm(ch);
    }

    async _load() {
        this.container.innerHTML = '<div class="dest-pane"></div>';
        Activity.in(this.container.firstChild, t('Reading the destinations…'), { area: true });
        try {
            [this._channels, this._galleries] = await Promise.all([
                ChannelAPI.list(),
                PublishedGalleryAPI.list().catch(() => []),
            ]);
        } catch (err) {
            this.container.innerHTML = `<div class="dest-pane"><div class="gal-error">${t('Could not load your destinations: {error}', { error: escapeHtml(err.message) })}</div></div>`;
            return;
        }
        if (this._editing === undefined) this._renderList();
        else this._renderForm(this._editing);
    }

    _galleryCount(slug) {
        return (this._galleries || []).filter(g => g.channelSlug === slug).length;
    }

    _uploadDesc(ch) {
        if (ch.handler === 'rsync') {
            const cfg = ch.handlerConfig || {};
            return `rsync · ${cfg.user ? cfg.user + '@' : ''}${cfg.host || ''}${cfg.remotePath ? ':' + cfg.remotePath : ''}`;
        }
        if (ch.handler) return ch.handler;
        return destinationType(ch) === 'files' ? t('No upload — you post the files yourself') : t('Not set up');
    }

    // For an online destination the address is its public URL; for a files
    // destination it is the folder the files land in — the resolved one, not
    // the possibly-relative string in the configuration.
    _addressDesc(ch) {
        const base = _deployBaseURL(ch);
        if (base) return base.url.replace(/^https?:\/\//, '') + (base.guessed ? ' (guessed)' : '');
        return ch.outputDir || ch.outputPath || '—';
    }

    // One row per destination, in the grid of the galleries: what it is
    // and where it goes, how its files are made, and its galleries.
    _renderList() {
        const channels = this._channels || [];
        this.container.innerHTML = `
            <div class="dest-pane">
                ${placeHeaderHTML({
                    title: t('Destinations'),
                    meta: countHTML(channels.length, 'destination'),
                    actions: '<button class="btn btn-sm" id="dest-new">' + t('New destination…') + '</button>',
                    lede: placeLede(t('A destination is where {galleries} go — a website, share links for family and friends, or plain files you post yourself — and how their files are made.', { galleries: placeLink('published', 'galleries', t('galleries')) })),
                })}
                <div class="gal-body">
                    ${channels.length === 0
                        ? '<p class="gal-empty">' + t('No destinations yet. Choose New destination… to say where your first gallery should go.') + '</p>'
                        : `<div class="dest-list">${channels.map(ch => this._rowHTML(ch)).join('')}</div>`}
                </div>
            </div>`;
        this._wireList();
    }

    // One destination: its name, type and address, how it uploads and what
    // its files are, and the way to its galleries.
    _rowHTML(ch) {
        const type = destinationTypeLabel(ch);
        const base = _deployBaseURL(ch);
        const address = base
            ? `<a href="${escapeHtml(base.url)}" target="_blank" rel="noopener" class="dest-visit">${escapeHtml(this._addressDesc(ch))}</a>`
            : escapeHtml(this._addressDesc(ch));
        const files = [ch.format.toUpperCase(), `q${ch.quality}`, _scaleDesc(ch.scale)].filter(Boolean).join(' · ');
        const n = this._galleryCount(ch.slug);
        return `
            <div class="gal-row dest-row" data-slug="${escapeHtml(ch.slug)}" tabindex="0" role="link">
                <span class="gal-row-main">
                    <span class="gal-row-title">${escapeHtml(ch.name)}</span>
                    <span class="gal-row-sub">${type.toLowerCase() !== ch.name.toLowerCase() ? `${escapeHtml(type)} · ` : ''}<span class="dest-mono">${address}</span></span>
                </span>
                <span class="gal-row-state">
                    <span class="gal-row-sub">${escapeHtml(this._uploadDesc(ch))}</span>
                    <span class="gal-row-sub dest-mono">${escapeHtml(files)}</span>
                </span>
                <span class="gal-row-actions">${n > 0
                    ? `<button class="btn btn-sm dest-galleries-link" data-slug="${escapeHtml(ch.slug)}">${countHTML(n, 'gallery', 'galleries')}</button>`
                    : '<span class="gal-row-sub">' + t('No galleries') + '</span>'}</span>
            </div>`;
    }

    _wireList() {
        this.container.querySelector('#dest-new').addEventListener('click', () => {
            this._editing = null;
            this._renderForm(null);
        });
        for (const link of this.container.querySelectorAll('.dest-galleries-link')) {
            link.addEventListener('click', (e) => {
                e.stopPropagation();
                App.showGalleriesForChannel(link.dataset.slug);
            });
        }
        for (const row of this.container.querySelectorAll('.dest-row')) {
            const open = () => {
                this._editing = this._channels.find(c => c.slug === row.dataset.slug) || null;
                this._renderForm(this._editing);
            };
            row.addEventListener('click', (e) => { if (!e.target.closest('a, button')) open(); });
            row.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open(); }
            });
        }
    }

    // The form: type first, then only the fields that type needs.
    _renderForm(existing) {
        const isNew = !existing;
        const ch = existing || {
            slug: '', name: '', format: 'jpeg', quality: 85, exifMode: 'keep_no_gps',
            scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 1920 },
            handler: '', handlerConfig: {}, accounts: [],
            galleryExport: false, siteExport: false, siteTitle: '', siteTheme: 'light', siteURL: '',
            siteAbout: '', siteImprint: '', siteContactEmail: '', siteContactURL: '',
            outputMode: '', outputPath: '',
        };
        const type = isNew ? 'share' : destinationType(ch);

        this.container.innerHTML = `
            <div class="dest-pane dest-form-pane">
                ${placeHeaderHTML({
                    title: `<nav class="gal-crumbs" aria-label="${t('Breadcrumb')}"><button class="link-btn" id="dest-back">${t('Destinations')}</button><span class="gal-crumb-sep">/</span><span class="gal-crumb-here">${isNew ? t('New destination') : escapeHtml(ch.name)}</span></nav>`,
                    actions: `${isNew ? '' : '<a class="btn btn-sm" id="dest-show-photos" href="#libraries">' + t('Show photos') + '</a>'}
                        <button class="btn btn-sm" id="dest-cancel">${t('Cancel')}</button>
                        <button class="btn btn-sm btn-accent" id="dest-save">${isNew ? t('Create destination') : t('Save')}</button>`,
                })}
                <div class="gal-body dest-form" id="dest-form" data-type="${type}">
${_destTypeFieldsetHTML(isNew, type)}

${_destNameFieldsetHTML(ch)}

${_destUploadFieldsetHTML(ch, isNew)}

${_destImageFieldsetHTML(ch)}

${_destOutputFieldsetHTML(ch)}

${_destSiteFieldsetHTML(ch)}

${_destAdvancedHTML(ch, isNew)}

                    <div class="gal-detail-error" id="chf-error" hidden></div>
                </div>
            </div>`;

        this._wireForm(ch, isNew);
    }

    _wireForm(ch, isNew) {
        const root = this.container;
        const form = root.querySelector('#dest-form');
        const back = () => { this._editing = undefined; this._load(); };
        root.querySelector('#dest-back').addEventListener('click', back);
        root.querySelector('#dest-cancel').addEventListener('click', back);
        wireDestinationFields(form, isNew);
        const images = [new SiteImageField(form, ch, SITE_IMAGES.logo), new SiteImageField(form, ch, SITE_IMAGES.avatar)];
        if (isNew) {
            images.forEach(f => f.disableUntilCreated());
        } else {
            showPhotosLink(root.querySelector('#dest-show-photos'), { channel: ch.slug });
            this._wireConnectionTest(form, ch);
            images.forEach(f => f.wire());
            root.querySelector('#dest-reveal').addEventListener('click', () => ChannelAPI.reveal(ch.slug));
            this._wireDelete(root, ch);
            if (ch.siteExport) wireRebuildAlbums(root, ch);
        }
        this._wireSave(root, form, ch, isNew);
    }

    _wireSave(root, form, ch, isNew) {
        const saveBtn = root.querySelector('#dest-save');
        saveBtn.addEventListener('click', async () => {
            const errEl = form.querySelector('#chf-error');
            const { payload, error } = _readDestinationForm(form, isNew, ch.slug);
            if (error) {
                errEl.textContent = error;
                errEl.hidden = false;
                return;
            }
            const restore = Activity.button(saveBtn, t('Saving…'));
            try {
                if (isNew) await ChannelAPI.create(payload);
                else await ChannelAPI.update(payload.slug, payload);
                this._editing = undefined;
                await this._load();
            } catch (err) {
                restore();
                errEl.textContent = err.message;
                errEl.hidden = false;
            }
        });
    }

    // The test hits the destination's *saved* handler config server-side, so
    // the form is saved first — otherwise the test would quietly check an
    // older setting than the one on screen.
    _wireConnectionTest(form, ch) {
        const testBtn = form.querySelector('#chf-rsync-test');
        const resultEl = form.querySelector('#chf-rsync-test-result');
        testBtn.addEventListener('click', async () => {
            const { payload, error } = _readDestinationForm(form, false, ch.slug);
            if (error) {
                resultEl.textContent = error;
                resultEl.className = 'ch-rsync-test-result ch-rsync-test-fail';
                return;
            }
            const restore = Activity.button(testBtn, t('Testing…'));
            resultEl.textContent = '';
            resultEl.className = 'ch-rsync-test-result';
            try {
                await ChannelAPI.update(ch.slug, payload);
                const res = await ChannelAPI.deployTest(ch.slug);
                resultEl.textContent = res.ok ? t('Connection OK.') : res.error;
                resultEl.className = 'ch-rsync-test-result ' + (res.ok ? 'ch-rsync-test-ok' : 'ch-rsync-test-fail');
            } catch (err) {
                resultEl.textContent = err.message;
                resultEl.className = 'ch-rsync-test-result ch-rsync-test-fail';
            } finally {
                restore();
            }
        });
    }

    _wireDelete(root, ch) {
        const wrap = root.querySelector('#dest-delete-confirm');
        root.querySelector('#dest-delete').addEventListener('click', () => {
            const count = this._galleryCount(ch.slug);
            const keeps = count === 0 ? t('It holds no galleries.')
                : count === 1 ? t('Its gallery stays on disk and stays online; it just loses the destination that describes it.')
                : t('Its {n} galleries stay on disk and stay online; they just lose the destination that describes them.', { n: formatCount(count) });
            wrap.innerHTML = `
                <p class="gal-danger-question">${t('Delete “{name}”?', { name: escapeHtml(ch.name) })} ${keeps}</p>
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="dest-delete-cancel">${t('Keep it')}</button>
                    <button class="btn btn-sm btn-danger" id="dest-delete-confirm-btn">${t('Delete destination')}</button>
                </div>`;
            wrap.querySelector('#dest-delete-cancel').addEventListener('click', () => { wrap.innerHTML = ''; });
            wrap.querySelector('#dest-delete-confirm-btn').addEventListener('click', (e) => this._delete(wrap, ch, e.currentTarget));
        });
    }

    async _delete(wrap, ch, btn) {
        Activity.button(btn, t('Deleting…'));
        wrap.querySelector('#dest-delete-cancel').disabled = true;
        try {
            await ChannelAPI.delete(ch.slug);
            this._editing = undefined;
            await this._load();
        } catch (err) {
            wrap.innerHTML = `<div class="gal-detail-error">${t('Could not delete it: {error}', { error: escapeHtml(err.message) })}</div>`;
        }
    }
}
