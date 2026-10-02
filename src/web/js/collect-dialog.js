// collect-dialog.js — "Add to gallery…": pick a gallery, not a channel.
//
// People think in galleries ("Pauls 2. Geburtstag"), not in destinations, so
// the dialog is a list of galleries with their destination and state, plus
// one way to start a new one. The old two-dropdown form (channel, then "Add
// to") asked for the destination first, which is the part you rarely care
// about (ADR-0029's vocabulary).
//
// The frame, Escape, the scrim and the focus are the Dialog's (ADR-0033);
// this file only knows about galleries.

// collectPhotoGroups adds library photos — [{ libID, photoIDs }], from one
// library or several — to the gallery chosen in the dialog, and returns how
// many were added.
async function collectPhotoGroups(groups, { slug, draftID, postID, title, unlisted, account }) {
    groups = groups.filter(g => g.photoIDs.length > 0);
    if (groups.length === 0) throw new Error('No matching library photos found for this selection.');

    // The first call creates the draft, the rest append to it.
    // Passing the title to every call would instead create one
    // gallery per library, all with the same name.
    let total = 0;
    let currentDraft = draftID;
    for (const g of groups) {
        const draft = await LibraryAPI.collect(g.libID, slug, {
            photoIDs: g.photoIDs,
            draftID: currentDraft,
            postID: !currentDraft ? postID : undefined,
            title: !currentDraft && !postID ? title : undefined,
            unlisted: currentDraft ? undefined : unlisted,
            account: currentDraft ? undefined : account,
        });
        currentDraft = draft?.id || currentDraft;
        total += g.photoIDs.length;
    }
    return total;
}

class CollectDialog {
    // count:    how many photos are being added (for the title).
    // onCollect: async ({ slug, draftID, postID, title, unlisted, account })
    //            → the caller knows which photos and libraries are involved.
    constructor({ count, onCollect }) {
        this._count = count;
        this._onCollect = onCollect;
        this._pick = null;    // { slug, draftID, postID, label } or null = new
        this._newMode = false;
    }

    async open() {
        this._dialog = new Dialog({
            title: `Add ${this._count} photo${this._count !== 1 ? 's' : ''} to a gallery`,
            size: 'md',
            className: 'collect-dialog',
            body: `
                <div id="collect-body"></div>
                <div class="build-error" id="collect-error" hidden></div>`,
            actions: [
                { label: 'Cancel', id: 'collect-cancel', onClick: () => this.close() },
                { label: 'Add to gallery', kind: 'primary', id: 'collect-confirm', disabled: true, onClick: () => this._confirm() },
            ],
            onClose: () => { this._el = null; },
        });
        this._el = this._dialog.open();
        Activity.in(this._el.querySelector('#collect-body'), 'Reading your galleries…');

        try {
            [this._galleries, this._channels] = await Promise.all([
                PublishedGalleryAPI.list(),
                ChannelAPI.list(),
            ]);
        } catch (err) {
            this._el.querySelector('#collect-body').innerHTML =
                `<div class="channel-error">Could not load your galleries: ${escapeHtml(err.message)}</div>`;
            return;
        }
        if (this._channels.length === 0) {
            this._el.querySelector('#collect-body').innerHTML =
                '<div class="channel-empty">No destinations configured yet. Add one under Channels first, then collect photos into a gallery of it.</div>';
            return;
        }
        this._renderPicker();
    }

    close() {
        this._dialog?.close(null);
        this._el = null;
    }

    _channelBySlug(slug) {
        return this._channels.find(c => c.slug === slug);
    }

    // Galleries you can still add to: a generated album or a pending draft,
    // at any destination — a Files destination (an Instagram folder) has only
    // drafts. Newest first, which is the order the API already returns.
    _candidates() {
        return this._galleries.filter(g => !!this._channelBySlug(g.channelSlug));
    }

    _renderPicker() {
        const body = this._el.querySelector('#collect-body');
        const rows = this._candidates();
        body.innerHTML = `
            <div class="collect-search-row">
                <input class="form-input" id="collect-search" type="search" placeholder="Search your galleries" autocomplete="off">
            </div>
            <div class="collect-list" id="collect-list" role="listbox" aria-label="Galleries">
                <button class="collect-item collect-item--new" data-new="1" role="option" aria-selected="false">
                    <span class="collect-item-title">New gallery…</span>
                    <span class="collect-item-meta">Start a new one at a destination you choose</span>
                </button>
                ${rows.map(g => this._itemHTML(g)).join('')}
            </div>
            <div class="collect-new" id="collect-new" hidden></div>`;

        body.querySelector('#collect-search').addEventListener('input', (e) => {
            const q = e.target.value.trim().toLowerCase();
            for (const el of body.querySelectorAll('.collect-item:not(.collect-item--new)')) {
                el.hidden = q !== '' && !el.dataset.search.includes(q);
            }
        });
        body.querySelector('#collect-list').addEventListener('click', (e) => {
            const item = e.target.closest('.collect-item');
            if (!item) return;
            this._select(item);
        });
    }

    _itemHTML(g) {
        const ch = this._channelBySlug(g.channelSlug);
        const state = galleryState(g, ch);
        const target = g.status === 'draft' ? `draft:${g.draftID}` : `post:${g.postID}`;
        const search = `${galleryTitle(g)} ${ch?.name || g.channelName}`.toLowerCase();
        return `
            <button class="collect-item" role="option" aria-selected="false"
                    data-slug="${escapeHtml(g.channelSlug)}" data-target="${escapeHtml(target)}"
                    data-label="${escapeHtml(galleryTitle(g))}"
                    data-search="${escapeHtml(search)}">
                <span class="collect-item-title">${escapeHtml(galleryTitle(g))}</span>
                <span class="collect-item-meta">${escapeHtml(ch?.name || g.channelName)} · ${escapeHtml(state.label)} · ${g.status === 'draft' ? g.pendingCount : g.photoCount} photo${(g.status === 'draft' ? g.pendingCount : g.photoCount) !== 1 ? 's' : ''}</span>
            </button>`;
    }

    _select(item) {
        for (const el of this._el.querySelectorAll('.collect-item')) {
            el.classList.toggle('selected', el === item);
            el.setAttribute('aria-selected', el === item ? 'true' : 'false');
        }
        const newWrap = this._el.querySelector('#collect-new');
        this._newMode = item.dataset.new === '1';
        if (this._newMode) {
            this._pick = null;
            newWrap.hidden = false;
            this._renderNewForm(newWrap);
        } else {
            const [kind, id] = item.dataset.target.split(':');
            this._pick = {
                slug: item.dataset.slug,
                draftID: kind === 'draft' ? id : undefined,
                postID: kind === 'post' ? id : undefined,
                label: item.dataset.label,
            };
            newWrap.hidden = true;
        }
        this._updateConfirm();
    }

    _renderNewForm(wrap) {
        if (wrap.dataset.rendered) { this._updateNewDefaults(); return; }
        wrap.dataset.rendered = '1';
        wrap.innerHTML = `
            <div class="form-field">
                <label class="form-label" for="collect-new-title">Title</label>
                <input class="form-input" id="collect-new-title" placeholder="e.g. Herbst im Siebengebirge" autocomplete="off">
            </div>
            <div class="form-field">
                <label class="form-label" for="collect-new-dest">Destination</label>
                <select class="form-select" id="collect-new-dest">
                    ${this._channels.map(c => `<option value="${escapeHtml(c.slug)}">${escapeHtml(c.name)}</option>`).join('')}
                </select>
            </div>
            <div class="form-field" id="collect-new-account-wrap" hidden>
                <label class="form-label" for="collect-new-account">Account</label>
                <select class="form-select" id="collect-new-account"></select>
            </div>
            <div class="form-field" id="collect-new-visibility-wrap">
                <span class="form-label">Search engines</span>
                <div id="collect-new-visibility"></div>
                <span class="form-hint" id="collect-new-visibility-hint"></span>
            </div>`;
        wrap.querySelector('#collect-new-title').addEventListener('input', () => this._updateConfirm());
        wrap.querySelector('#collect-new-dest').addEventListener('change', () => this._updateNewDefaults());
        // Three visible labels, as every toggle carries (ADR-0019).
        this._visibilityToggle = Toggle.create(wrap.querySelector('#collect-new-visibility'), {
            initial: true, labelOn: 'Hidden', labelOff: 'Allowed',
        });
        this._updateNewDefaults();
        wrap.querySelector('#collect-new-title').focus();
    }

    // A share-link gallery is meant for one group of people, so it starts
    // hidden from search engines; a site album is part of a public site. A
    // Files destination puts image files in a folder: nothing is online, so
    // there is nothing for a search engine to find.
    _updateNewDefaults() {
        const wrap = this._el.querySelector('#collect-new');
        const ch = this._channelBySlug(wrap.querySelector('#collect-new-dest').value);
        if (!ch) return;
        const online = !!(ch.galleryExport || ch.siteExport);
        wrap.querySelector('#collect-new-visibility-wrap').hidden = !online;
        this._visibilityToggle.setState(!!ch.galleryExport);
        wrap.querySelector('#collect-new-visibility-hint').textContent = ch.siteExport
            ? 'Hidden keeps the album off the site index and sitemap, and adds a noindex tag.'
            : 'Hidden adds a noindex tag. The link itself stays unguessable either way.';
        const accounts = ch.accounts || [];
        const accountWrap = wrap.querySelector('#collect-new-account-wrap');
        accountWrap.hidden = accounts.length === 0;
        if (accounts.length) {
            wrap.querySelector('#collect-new-account').innerHTML =
                accounts.map(a => `<option value="${escapeHtml(a.id)}">${escapeHtml(a.label || a.id)}</option>`).join('');
        }
        this._updateConfirm();
    }

    _updateConfirm() {
        const btn = this._el.querySelector('#collect-confirm');
        if (this._newMode) {
            btn.disabled = !this._el.querySelector('#collect-new-title')?.value.trim();
        } else {
            btn.disabled = !this._pick;
        }
    }

    async _confirm() {
        const btn = this._el.querySelector('#collect-confirm');
        const errEl = this._el.querySelector('#collect-error');
        const wrap = this._el.querySelector('#collect-new');
        const params = this._newMode
            ? {
                slug: wrap.querySelector('#collect-new-dest').value,
                title: wrap.querySelector('#collect-new-title').value.trim(),
                unlisted: wrap.querySelector('#collect-new-visibility-wrap').hidden ? false : this._visibilityToggle.state(),
                account: wrap.querySelector('#collect-new-account-wrap').hidden
                    ? undefined
                    : wrap.querySelector('#collect-new-account').value || undefined,
                label: wrap.querySelector('#collect-new-title').value.trim(),
            }
            : this._pick;

        btn.disabled = true;
        btn.textContent = 'Adding…';
        errEl.hidden = true;
        try {
            const total = await this._onCollect(params);
            this.close();
            // Say what happened and where it went — a silent success leaves
            // people wondering whether anything was published.
            App.showToast(`Added ${total} photo${total !== 1 ? 's' : ''} to "${params.label}". Nothing is online until you publish it in Galleries.`);
        } catch (err) {
            errEl.textContent = err.message;
            errEl.hidden = false;
            btn.disabled = false;
            btn.textContent = 'Add to gallery';
        }
    }
}
