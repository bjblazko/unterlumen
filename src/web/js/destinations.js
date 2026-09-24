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
        label: 'Share links',
        hint: 'Unrelated galleries, each under its own unguessable link. For family and friends.',
        matches: ch => !!ch.galleryExport && !ch.siteExport,
    },
    {
        id: 'site',
        label: 'Website',
        hint: 'Related albums with an index page, an about page and a sitemap.',
        matches: ch => !!ch.siteExport,
    },
    {
        id: 'files',
        label: 'Files',
        hint: 'Image files in a folder, sized for a platform. You post them yourself.',
        matches: ch => !ch.galleryExport && !ch.siteExport,
    },
];

function destinationType(ch) {
    return (DESTINATION_TYPES.find(t => t.matches(ch)) || DESTINATION_TYPES[2]).id;
}

function destinationTypeLabel(ch) {
    return DESTINATION_TYPES.find(t => t.id === destinationType(ch)).label;
}

function slugify(name) {
    return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}


// Reads the Destinations form. The type radios replace the old export-mode
// select, and the slug is derived from the name rather than typed: changing a
// slug would move published output and break links already shared.
function _readDestinationForm(form, isNew, existingSlug) {
    const name = form.querySelector('#chf-name').value.trim();
    const slug = isNew ? form.querySelector('#chf-slug').value.trim() : existingSlug;
    if (!name) return { error: 'The destination needs a name.' };
    if (!slug) return { error: 'The name must contain at least one letter or digit.' };

    const type = form.dataset.type;
    const isSite = type === 'site';
    const isOnline = type !== 'files';

    const handlerVal = form.querySelector('#chf-handler').value;
    let handlerConfig;
    if (handlerVal === 'rsync') {
        // Keys must match deploy.TargetFromConfig (src/internal/deploy/rsync.go):
        // host, user, port, remotePath, identityFile.
        const host = form.querySelector('#chf-rsync-host').value.trim();
        const port = form.querySelector('#chf-rsync-port').value.trim();
        const user = form.querySelector('#chf-rsync-user').value.trim();
        const remotePath = form.querySelector('#chf-rsync-remote-path').value.trim();
        const identityFile = form.querySelector('#chf-rsync-identity').value.trim();
        if (!host || !user || !remotePath) {
            return { error: 'Uploading over SSH needs a host, a user and a remote folder.' };
        }
        handlerConfig = { host, user, remotePath };
        if (port) handlerConfig.port = port;
        if (identityFile) handlerConfig.identityFile = identityFile;
    } else {
        handlerConfig = _readKVEditor(form.querySelector('#chf-hconfig')) || undefined;
    }

    return {
        payload: {
            slug,
            name,
            format:           form.querySelector('#chf-format').value,
            quality:          parseInt(form.querySelector('#chf-quality').value, 10),
            exifMode:         form.querySelector('#chf-exif').value,
            scale:            _readScaleOpts(form),
            galleryExport:    type === 'share' ? true : undefined,
            siteExport:       isSite ? true : undefined,
            siteTitle:        isSite ? (form.querySelector('#chf-site-title').value.trim() || undefined) : undefined,
            siteTheme:        isSite ? (form.querySelector('#chf-site-theme').value || undefined) : undefined,
            siteURL:          isOnline ? (form.querySelector('#chf-site-url').value.trim() || undefined) : undefined,
            siteAbout:        isSite ? (form.querySelector('#chf-site-about').value.trim() || undefined) : undefined,
            siteImprint:      isSite ? (form.querySelector('#chf-site-imprint').value.trim() || undefined) : undefined,
            siteContactEmail: isSite ? (form.querySelector('#chf-site-contact-email').value.trim() || undefined) : undefined,
            siteContactURL:   isSite ? (form.querySelector('#chf-site-contact-url').value.trim() || undefined) : undefined,
            handler:          handlerVal || undefined,
            handlerConfig,
            accounts:         _readAccountsEditor(form.querySelector('#chf-accounts')),
            outputMode:       form.querySelector('#chf-output-mode').value === 'download' ? 'download' : undefined,
            outputPath:       form.querySelector('#chf-output-path')?.value.trim() || undefined,
        },
    };
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
        this.container.innerHTML = '<div class="dest-pane"><div class="gal-loading">Loading…</div></div>';
        try {
            [this._channels, this._galleries] = await Promise.all([
                ChannelAPI.list(),
                PublishedGalleryAPI.list().catch(() => []),
            ]);
        } catch (err) {
            this.container.innerHTML = `<div class="dest-pane"><div class="gal-error">Could not load your destinations: ${escapeHtml(err.message)}</div></div>`;
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
        return destinationType(ch) === 'files' ? 'No upload — you post the files yourself' : 'Not set up';
    }

    // For an online destination the address is its public URL; for a files
    // destination it is the folder the files land in — the resolved one, not
    // the possibly-relative string in the configuration.
    _addressDesc(ch) {
        const base = _deployBaseURL(ch);
        if (base) return base.url.replace(/^https?:\/\//, '') + (base.guessed ? ' (guessed)' : '');
        return ch.outputDir || ch.outputPath || '—';
    }

    _renderList() {
        const rows = (this._channels || []).map(ch => `
            <tr class="dest-row" data-slug="${escapeHtml(ch.slug)}" tabindex="0" role="link">
                <td class="dest-name">${escapeHtml(ch.name)}</td>
                <td>${escapeHtml(destinationTypeLabel(ch))}</td>
                <td class="dest-mono">${(() => {
                    const base = _deployBaseURL(ch);
                    return base
                        ? `<a href="${escapeHtml(base.url)}" target="_blank" rel="noopener" class="dest-visit">${escapeHtml(this._addressDesc(ch))}</a>`
                        : escapeHtml(this._addressDesc(ch));
                })()}</td>
                <td class="${ch.handler || destinationType(ch) === 'files' ? '' : 'dest-quiet'}">${escapeHtml(this._uploadDesc(ch))}</td>
                <td class="dest-mono dest-quiet">${escapeHtml(_scaleDesc(ch.scale) ? `${ch.format.toUpperCase()} · q${ch.quality} · ${_scaleDesc(ch.scale)}` : `${ch.format.toUpperCase()} · q${ch.quality}`)}</td>
                <td class="dest-num">${this._galleryCount(ch.slug) > 0
                    ? `<button class="link-btn dest-galleries-link" data-slug="${escapeHtml(ch.slug)}">${this._galleryCount(ch.slug)}</button>`
                    : '0'}</td>
            </tr>`).join('');

        this.container.innerHTML = `
            <div class="dest-pane">
                <div class="gal-head">
                    <h1 class="gal-title">Destinations</h1>
                    <span class="gal-group-spacer"></span>
                    <button class="btn btn-sm" id="dest-new">New destination…</button>
                </div>
                <div class="gal-body">
                    <p class="dest-intro">A destination is where galleries go and how their files are made. Galleries are created when you add photos to them.</p>
                    ${(this._channels || []).length === 0
                        ? '<p class="gal-empty">No destinations yet. Add one to publish your first gallery.</p>'
                        : `<table class="dest-table">
                            <thead>
                                <tr>
                                    <th>Destination</th><th>Type</th><th>Address</th>
                                    <th>Upload</th><th>Image files</th><th class="dest-num">Galleries</th>
                                </tr>
                            </thead>
                            <tbody>${rows}</tbody>
                        </table>`}
                </div>
            </div>`;

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
            row.addEventListener('click', (e) => {
                if (e.target.closest('a, button')) return;
                open();
            });
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
                <div class="gal-head">
                    <nav class="gal-crumbs" aria-label="Breadcrumb">
                        <button class="link-btn" id="dest-back">Destinations</button>
                        <span class="gal-crumb-sep">/</span>
                        <span class="gal-crumb-here">${isNew ? 'New destination' : escapeHtml(ch.name)}</span>
                    </nav>
                    <span class="gal-group-spacer"></span>
                    <button class="btn btn-sm" id="dest-cancel">Cancel</button>
                    <button class="btn btn-sm btn-accent" id="dest-save">${isNew ? 'Create destination' : 'Save'}</button>
                </div>
                <div class="gal-body dest-form" id="dest-form" data-type="${type}">
                    <fieldset class="dest-fieldset">
                        <legend>Type</legend>
                        <div class="dest-choices">
                            ${DESTINATION_TYPES.map(t => `
                                <label class="dest-choice">
                                    <input type="radio" name="dest-type" value="${t.id}" ${t.id === type ? 'checked' : ''} ${isNew ? '' : 'disabled'}>
                                    <span><strong>${escapeHtml(t.label)}</strong><span>${escapeHtml(t.hint)}</span></span>
                                </label>`).join('')}
                        </div>
                        ${isNew ? '' : '<p class="form-hint">The type is fixed after the first publish: it decides how the files are laid out and what the links look like.</p>'}
                    </fieldset>

                    <fieldset class="dest-fieldset">
                        <legend>Name and address</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-name">Name</label>
                                <input class="form-input" id="chf-name" value="${escapeHtml(ch.name)}" placeholder="e.g. Family share links">
                                <span class="form-hint">Folder name: <span class="dest-mono" id="chf-slug-preview">${escapeHtml(ch.slug || slugify(ch.name) || '—')}</span></span>
                                <input type="hidden" id="chf-slug" value="${escapeHtml(ch.slug)}">
                            </div>
                            <div class="form-field dest-when-online">
                                <label class="form-label" for="chf-site-url">Public address</label>
                                <input class="form-input dest-mono" id="chf-site-url" value="${escapeHtml(ch.siteURL || '')}" placeholder="https://example.com">
                                <span class="form-hint">Used for the links you share and for the reachability check.</span>
                            </div>
                        </div>
                    </fieldset>

                    <fieldset class="dest-fieldset dest-when-online">
                        <legend>Upload</legend>
                        <div class="dest-choices">
                            <label class="dest-choice">
                                <input type="radio" name="dest-upload" value="rsync" ${ch.handler === 'rsync' ? 'checked' : ''}>
                                <span><strong>Upload over SSH</strong><span>Unterlumen copies changes to your server with rsync when you publish.</span></span>
                            </label>
                            <label class="dest-choice">
                                <input type="radio" name="dest-upload" value="" ${ch.handler === 'rsync' ? '' : 'checked'}>
                                <span><strong>Keep in a local folder</strong><span>You upload the folder yourself. Unterlumen cannot tell when it is online.</span></span>
                            </label>
                        </div>
                        <div id="chf-rsync-wrap" ${ch.handler === 'rsync' ? '' : 'hidden'}>
                            <div class="dest-grid">
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-host">Host</label>
                                    <input class="form-input dest-mono" id="chf-rsync-host" value="${escapeHtml(ch.handlerConfig?.host || '')}" placeholder="example.com">
                                </div>
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-user">User</label>
                                    <input class="form-input dest-mono" id="chf-rsync-user" value="${escapeHtml(ch.handlerConfig?.user || '')}" placeholder="deploy">
                                </div>
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-remote-path">Remote folder</label>
                                    <input class="form-input dest-mono" id="chf-rsync-remote-path" value="${escapeHtml(ch.handlerConfig?.remotePath || '')}" placeholder="/var/www/photos">
                                </div>
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-identity">SSH key</label>
                                    <input class="form-input dest-mono" id="chf-rsync-identity" value="${escapeHtml(ch.handlerConfig?.identityFile || '')}" placeholder="~/.ssh/id_ed25519">
                                    <span class="form-hint">Optional. Leave empty to use your default key or agent; the port lives under Advanced.</span>
                                </div>
                            </div>
                            <div class="ch-rsync-test-row">
                                <button type="button" class="btn btn-sm" id="chf-rsync-test"${isNew ? ' disabled' : ''}>Test connection</button>
                                <span class="ch-rsync-test-result" id="chf-rsync-test-result">${isNew ? 'Create the destination first, then test the connection.' : ''}</span>
                            </div>
                            <div class="form-hint ch-rsync-help">
                                Deploy uses your system <code>ssh</code>/<code>rsync</code>, authenticated by key — no passwords are stored.
                                <ol>
                                    <li>No key yet? <code>ssh-keygen -t ed25519</code>.</li>
                                    <li>Copy it to the server: <code>ssh-copy-id -i ~/.ssh/id_ed25519.pub user@host</code>.</li>
                                    <li>Connect once by hand — <code>ssh user@host</code> — so the host key is trusted.</li>
                                </ol>
                            </div>
                        </div>
                    </fieldset>

                    <fieldset class="dest-fieldset">
                        <legend>Image files</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-format">Format</label>
                                <select class="form-select" id="chf-format">
                                    <option value="jpeg" ${ch.format === 'jpeg' ? 'selected' : ''}>JPEG</option>
                                    <option value="png" ${ch.format === 'png' ? 'selected' : ''}>PNG</option>
                                    <option value="webp" ${ch.format === 'webp' ? 'selected' : ''}>WebP</option>
                                </select>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-quality">Quality</label>
                                <input class="form-input" id="chf-quality" type="number" min="1" max="100" value="${ch.quality}">
                                <span class="form-hint">1–100.</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-scale-mode">Size</label>
                                <select class="form-select" id="chf-scale-mode">
                                    <option value="none" ${ch.scale?.mode === 'none' ? 'selected' : ''}>Original size</option>
                                    <option value="max_dim" ${ch.scale?.mode === 'max_dim' ? 'selected' : ''}>Max dimension</option>
                                    <option value="percent" ${ch.scale?.mode === 'percent' ? 'selected' : ''}>Percent</option>
                                </select>
                                <div id="chf-scale-opts" class="form-scale-opts">${_scaleOptsHTML(ch.scale)}</div>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-exif">Metadata</label>
                                <select class="form-select" id="chf-exif">
                                    <option value="keep_no_gps" ${ch.exifMode === 'keep_no_gps' ? 'selected' : ''}>Keep, without GPS</option>
                                    <option value="keep" ${ch.exifMode === 'keep' ? 'selected' : ''}>Keep all</option>
                                    <option value="strip" ${ch.exifMode === 'strip' ? 'selected' : ''}>Remove all</option>
                                </select>
                            </div>
                        </div>
                    </fieldset>

                    <fieldset class="dest-fieldset dest-when-files">
                        <legend>Output</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-output-mode">Where the files go</label>
                                <select class="form-select" id="chf-output-mode">
                                    <option value="save" ${(ch.outputMode || 'save') === 'save' ? 'selected' : ''}>Into a folder</option>
                                    <option value="download" ${ch.outputMode === 'download' ? 'selected' : ''}>Download as a ZIP</option>
                                </select>
                            </div>
                            <div class="form-field" id="chf-output-path-wrap" ${ch.outputMode === 'download' ? 'hidden' : ''}>
                                <label class="form-label" for="chf-output-path">Folder</label>
                                <div class="export-destination-wrap">
                                    <input class="form-input export-destination-input dest-mono" id="chf-output-path"
                                           value="${escapeHtml(ch.outputPath ? (ch.outputDir || ch.outputPath) : '')}"
                                           placeholder="~/.unterlumen/channels/${escapeHtml(ch.slug || '<name>')}/">
                                    <button type="button" class="btn btn-sm" id="chf-output-pick" title="Browse folders">…</button>
                                </div>
                            </div>
                        </div>
                    </fieldset>

                    <fieldset class="dest-fieldset dest-when-site">
                        <legend>Website</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-site-title">Site title</label>
                                <input class="form-input" id="chf-site-title" value="${escapeHtml(ch.siteTitle || '')}" placeholder="e.g. My Photography">
                                <span class="form-hint">Shown in the header on every page.</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-site-theme">Default theme</label>
                                <select class="form-select" id="chf-site-theme">
                                    <option value="light" ${(ch.siteTheme || 'light') === 'light' ? 'selected' : ''}>Light</option>
                                    <option value="dark" ${ch.siteTheme === 'dark' ? 'selected' : ''}>Dark</option>
                                </select>
                                <span class="form-hint">Visitors can switch.</span>
                            </div>
                        </div>
                        <div class="form-field">
                            <span class="form-label">Site logo</span>
                            <div class="ch-avatar-wrap" id="chf-logo-wrap">
                                <span class="ch-avatar-status" id="chf-logo-status">Loading…</span>
                                <input type="file" id="chf-logo-file" accept="image/*" style="display:none">
                                <button class="btn btn-sm" id="chf-logo-upload">Upload logo</button>
                                <button class="btn btn-sm" id="chf-logo-remove" style="display:none">Remove</button>
                            </div>
                        </div>
                        <div class="form-field">
                            <label class="form-label" for="chf-site-about">About page</label>
                            <textarea class="form-input dest-textarea" id="chf-site-about" rows="6" placeholder="Write a short introduction about yourself and your photography…">${escapeHtml(ch.siteAbout || '')}</textarea>
                            <span class="form-hint">Markdown. Becomes about.html.</span>
                        </div>
                        <div class="form-field">
                            <span class="form-label">Author photo</span>
                            <div class="ch-avatar-wrap" id="chf-avatar-wrap">
                                <span class="ch-avatar-status" id="chf-avatar-status">Loading…</span>
                                <input type="file" id="chf-avatar-file" accept="image/*" style="display:none">
                                <button class="btn btn-sm" id="chf-avatar-upload">Upload photo</button>
                                <button class="btn btn-sm" id="chf-avatar-remove" style="display:none">Remove</button>
                            </div>
                            <span class="form-hint">Shown on the About page.</span>
                        </div>
                        <div class="form-field">
                            <label class="form-label" for="chf-site-imprint">Legal / imprint</label>
                            <textarea class="form-input dest-textarea" id="chf-site-imprint" rows="6" placeholder="Responsible for this website:&#10;Your Name&#10;Your Address…">${escapeHtml(ch.siteImprint || '')}</textarea>
                            <span class="form-hint">Markdown. Becomes legal.html.</span>
                        </div>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-site-contact-email">Contact email</label>
                                <input class="form-input" id="chf-site-contact-email" type="email" value="${escapeHtml(ch.siteContactEmail || '')}" placeholder="you@example.com">
                                <span class="form-hint">Shown in the footer of every page.</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-site-contact-url">Website or social address</label>
                                <input class="form-input" id="chf-site-contact-url" type="url" value="${escapeHtml(ch.siteContactURL || '')}" placeholder="https://yoursite.com">
                            </div>
                        </div>
                    </fieldset>

                    <details class="dest-advanced">
                        <summary>Advanced: accounts, custom handler settings, rebuild</summary>
                        <div class="dest-advanced-body">
                            <div class="form-field">
                                <label class="form-label" for="chf-handler">Handler</label>
                                <select class="form-select" id="chf-handler">
                                    <option value="" ${!ch.handler ? 'selected' : ''}>None</option>
                                    <option value="rsync" ${ch.handler === 'rsync' ? 'selected' : ''}>rsync (deploy over SSH)</option>
                                    ${(ch.handler && ch.handler !== 'rsync') ? `<option value="${escapeHtml(ch.handler)}" selected>${escapeHtml(ch.handler)} (existing)</option>` : ''}
                                </select>
                                <span class="form-hint">Set by the Upload section above; change it here only for a handler Unterlumen has no form for.</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-rsync-port">Port</label>
                                <input class="form-input" id="chf-rsync-port" type="number" min="1" max="65535" value="${escapeHtml(ch.handlerConfig?.port || '')}" placeholder="22">
                            </div>
                            <div class="form-field" id="chf-generic-hconfig-wrap" ${ch.handler === 'rsync' ? 'hidden' : ''}>
                                <span class="form-label">Handler config</span>
                                <div id="chf-hconfig" class="kv-editor">${_kvEditorHTML(ch.handler === 'rsync' ? {} : (ch.handlerConfig || {}))}</div>
                                <button class="btn btn-sm" id="chf-hconfig-add">+ Add config entry</button>
                            </div>
                            <div class="form-field">
                                <span class="form-label">Accounts</span>
                                <div id="chf-accounts" class="accounts-editor">${_accountsEditorHTML(ch.accounts || [])}</div>
                                <button class="btn btn-sm" id="chf-account-add">+ Add account</button>
                                <span class="form-hint">Named sub-accounts, e.g. two Mastodon logins.</span>
                            </div>
                            ${isNew ? '' : `
                            <div class="form-field">
                                <span class="form-label">Maintenance</span>
                                <div class="dest-maintenance">
                                    <button class="btn btn-sm" id="dest-reveal">Show output in Finder/Explorer</button>
                                    <button class="btn btn-sm btn-danger" id="dest-delete">Delete destination…</button>
                                </div>
                                <span class="form-hint">Deleting removes the destination from your configuration. Galleries already published to it stay where they are.</span>
                                <div id="dest-delete-confirm"></div>
                            </div>`}
                        </div>
                    </details>

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

        // Type drives which sections exist at all.
        for (const radio of form.querySelectorAll('input[name="dest-type"]')) {
            radio.addEventListener('change', () => { form.dataset.type = radio.value; });
        }

        // Name derives the folder name; the slug itself is not an editable
        // field any more, because changing it would move published output.
        const nameEl = form.querySelector('#chf-name');
        const slugEl = form.querySelector('#chf-slug');
        const slugPreview = form.querySelector('#chf-slug-preview');
        if (isNew) {
            nameEl.addEventListener('input', () => {
                slugEl.value = slugify(nameEl.value);
                slugPreview.textContent = slugEl.value || '—';
            });
        }

        // Upload choice and the handler select are two views of one setting.
        const handlerEl = form.querySelector('#chf-handler');
        const rsyncWrap = form.querySelector('#chf-rsync-wrap');
        const genericWrap = form.querySelector('#chf-generic-hconfig-wrap');
        const syncHandler = (value) => {
            handlerEl.value = value;
            rsyncWrap.hidden = value !== 'rsync';
            genericWrap.hidden = value === 'rsync';
        };
        for (const radio of form.querySelectorAll('input[name="dest-upload"]')) {
            radio.addEventListener('change', () => syncHandler(radio.value));
        }
        handlerEl.addEventListener('change', () => {
            const isRsync = handlerEl.value === 'rsync';
            rsyncWrap.hidden = !isRsync;
            genericWrap.hidden = isRsync;
            const radio = form.querySelector(`input[name="dest-upload"][value="${isRsync ? 'rsync' : ''}"]`);
            if (radio) radio.checked = true;
        });

        const scaleMode = form.querySelector('#chf-scale-mode');
        scaleMode.addEventListener('change', () => {
            form.querySelector('#chf-scale-opts').innerHTML = _scaleOptsHTML({ mode: scaleMode.value });
        });

        const outputModeEl = form.querySelector('#chf-output-mode');
        outputModeEl.addEventListener('change', () => {
            form.querySelector('#chf-output-path-wrap').hidden = outputModeEl.value === 'download';
        });
        form.querySelector('#chf-output-pick').addEventListener('click', async () => {
            const current = form.querySelector('#chf-output-path').value.trim();
            const chosen = await new FolderPicker().open(current || '', { title: 'Output folder' });
            if (chosen !== null) form.querySelector('#chf-output-path').value = chosen;
        });

        form.querySelector('#chf-hconfig-add').addEventListener('click', () => {
            form.querySelector('#chf-hconfig').insertAdjacentHTML('beforeend', _kvRowHTML('', ''));
        });
        form.querySelector('#chf-account-add').addEventListener('click', () => {
            form.querySelector('#chf-accounts').insertAdjacentHTML('beforeend', _accountRowHTML({ id: '', label: '', config: {} }));
        });

        if (!isNew) {
            this._wireConnectionTest(form, ch);
            this._initLogoUI(form, ch);
            this._initAvatarUI(form, ch);
            root.querySelector('#dest-reveal').addEventListener('click', () => ChannelAPI.reveal(ch.slug));
            this._wireDelete(root, ch);
        } else {
            form.querySelector('#chf-logo-status').textContent = 'Create the destination first, then upload a logo.';
            form.querySelector('#chf-logo-upload').disabled = true;
            form.querySelector('#chf-avatar-status').textContent = 'Create the destination first, then upload a photo.';
            form.querySelector('#chf-avatar-upload').disabled = true;
        }

        root.querySelector('#dest-save').addEventListener('click', async () => {
            const errEl = form.querySelector('#chf-error');
            const { payload, error } = _readDestinationForm(form, isNew, ch.slug);
            if (error) {
                errEl.textContent = error;
                errEl.hidden = false;
                return;
            }
            try {
                if (isNew) await ChannelAPI.create(payload);
                else await ChannelAPI.update(payload.slug, payload);
                this._editing = undefined;
                await this._load();
            } catch (err) {
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
            testBtn.disabled = true;
            const label = testBtn.textContent;
            testBtn.textContent = 'Testing…';
            resultEl.textContent = '';
            resultEl.className = 'ch-rsync-test-result';
            try {
                await ChannelAPI.update(ch.slug, payload);
                const res = await ChannelAPI.deployTest(ch.slug);
                resultEl.textContent = res.ok ? 'Connection OK.' : res.error;
                resultEl.className = 'ch-rsync-test-result ' + (res.ok ? 'ch-rsync-test-ok' : 'ch-rsync-test-fail');
            } catch (err) {
                resultEl.textContent = err.message;
                resultEl.className = 'ch-rsync-test-result ch-rsync-test-fail';
            } finally {
                testBtn.disabled = false;
                testBtn.textContent = label;
            }
        });
    }

    _wireDelete(root, ch) {
        const wrap = root.querySelector('#dest-delete-confirm');
        root.querySelector('#dest-delete').addEventListener('click', () => {
            const count = this._galleryCount(ch.slug);
            wrap.innerHTML = `
                <p class="gal-danger-question">Delete "${escapeHtml(ch.name)}"? ${count > 0
                    ? `Its ${count} gallerie${count !== 1 ? 's' : ''} stay on disk and stay online; they just lose the destination that describes them.`
                    : 'It holds no galleries.'}</p>
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="dest-delete-cancel">Keep it</button>
                    <button class="btn btn-sm btn-danger" id="dest-delete-confirm-btn">Delete destination</button>
                </div>`;
            wrap.querySelector('#dest-delete-cancel').addEventListener('click', () => { wrap.innerHTML = ''; });
            wrap.querySelector('#dest-delete-confirm-btn').addEventListener('click', async () => {
                try {
                    await ChannelAPI.delete(ch.slug);
                    this._editing = undefined;
                    await this._load();
                } catch (err) {
                    wrap.innerHTML = `<div class="gal-detail-error">Could not delete it: ${escapeHtml(err.message)}</div>`;
                }
            });
        });
    }

    async _initLogoUI(form, ch) {
        const slug = ch.slug;
        const statusEl  = form.querySelector('#chf-logo-status');
        const uploadBtn = form.querySelector('#chf-logo-upload');
        const removeBtn = form.querySelector('#chf-logo-remove');
        const fileInput = form.querySelector('#chf-logo-file');

        const refresh = async () => {
            try {
                const { exists } = await ChannelAPI.logoStatus(slug);
                statusEl.textContent = exists ? 'Logo uploaded.' : 'No logo set.';
                removeBtn.style.display = exists ? '' : 'none';
            } catch {
                statusEl.textContent = 'Could not check logo.';
            }
        };

        const rebuildIfSite = async () => {
            if (!ch.siteExport) return;
            statusEl.textContent = 'Rebuilding site…';
            try { await ChannelAPI.rebuildSite(slug); } catch {
                statusEl.textContent = 'Logo saved, but the site didn\'t rebuild — open the Published tab and click Publish on the affected album to regenerate it.';
            }
        };

        await refresh();

        uploadBtn.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async () => {
            const file = fileInput.files[0];
            if (!file) return;
            statusEl.textContent = 'Uploading…';
            uploadBtn.disabled = true;
            try {
                await ChannelAPI.uploadLogo(slug, file);
                await rebuildIfSite();
                await refresh();
            } catch (err) {
                statusEl.textContent = 'Upload failed: ' + err.message;
            } finally {
                uploadBtn.disabled = false;
                fileInput.value = '';
            }
        });

        removeBtn.addEventListener('click', async () => {
            removeBtn.disabled = true;
            try {
                await ChannelAPI.deleteLogo(slug);
                await rebuildIfSite();
                await refresh();
            } catch (err) {
                statusEl.textContent = 'Remove failed: ' + err.message;
            } finally {
                removeBtn.disabled = false;
            }
        });
    }

    async _initAvatarUI(form, ch) {
        const slug = ch.slug;
        const statusEl  = form.querySelector('#chf-avatar-status');
        const uploadBtn = form.querySelector('#chf-avatar-upload');
        const removeBtn = form.querySelector('#chf-avatar-remove');
        const fileInput = form.querySelector('#chf-avatar-file');

        const refresh = async () => {
            try {
                const { exists } = await ChannelAPI.avatarStatus(slug);
                statusEl.textContent = exists ? 'Portrait uploaded.' : 'No portrait set.';
                removeBtn.style.display = exists ? '' : 'none';
            } catch {
                statusEl.textContent = 'Could not check avatar.';
            }
        };

        const rebuildIfSite = async () => {
            if (!ch.siteExport) return;
            statusEl.textContent = 'Rebuilding site…';
            try { await ChannelAPI.rebuildSite(slug); } catch {
                statusEl.textContent = 'Portrait saved, but the site didn\'t rebuild — open the Published tab and click Publish on the affected album to regenerate it.';
            }
        };

        await refresh();

        uploadBtn.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async () => {
            const file = fileInput.files[0];
            if (!file) return;
            statusEl.textContent = 'Uploading…';
            uploadBtn.disabled = true;
            try {
                await ChannelAPI.uploadAvatar(slug, file);
                await rebuildIfSite();
                await refresh();
            } catch (err) {
                statusEl.textContent = 'Upload failed: ' + err.message;
            } finally {
                uploadBtn.disabled = false;
                fileInput.value = '';
            }
        });

        removeBtn.addEventListener('click', async () => {
            removeBtn.disabled = true;
            try {
                await ChannelAPI.deleteAvatar(slug);
                await rebuildIfSite();
                await refresh();
            } catch (err) {
                statusEl.textContent = 'Remove failed: ' + err.message;
            } finally {
                removeBtn.disabled = false;
            }
        });
    }
}
