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


// Reads the Destinations form. The type radios replace the old export-mode
// select, and the slug is derived from the name rather than typed: changing a
// slug would move published output and break links already shared.
function _readDestinationForm(form, isNew, existingSlug) {
    const name = form.querySelector('#chf-name').value.trim();
    const slug = isNew ? form.querySelector('#chf-slug').value.trim() : existingSlug;
    if (!name) return { error: t('The destination needs a name.') };
    if (!slug) return { error: t('The name must contain at least one letter or digit.') };

    const type = form.dataset.type;
    const handlerVal = form.querySelector('#chf-handler').value;
    const handler = _readHandlerConfig(form, handlerVal);
    if (handler.error) return { error: handler.error };

    return {
        payload: {
            slug,
            name,
            format:           form.querySelector('#chf-format').value,
            quality:          parseInt(form.querySelector('#chf-quality').value, 10),
            exifMode:         form.querySelector('#chf-exif').value,
            scale:            _readScaleOpts(form),
            galleryExport:    type === 'share' ? true : undefined,
            ..._readSiteFields(form, type),
            handler:          handlerVal || undefined,
            handlerConfig:    handler.config,
            accounts:         _readAccountsEditor(form.querySelector('#chf-accounts')),
            outputMode:       form.querySelector('#chf-output-mode').value === 'download' ? 'download' : undefined,
            outputPath:       form.querySelector('#chf-output-path')?.value.trim() || undefined,
        },
    };
}

// The handler's settings: the SSH fields for rsync, the key/value editor for
// any other handler.
function _readHandlerConfig(form, handlerVal) {
    if (handlerVal !== 'rsync') {
        return { config: _readKVEditor(form.querySelector('#chf-hconfig')) || undefined };
    }
    // Keys must match deploy.TargetFromConfig (src/internal/deploy/rsync.go):
    // host, user, port, remotePath, identityFile.
    const field = (id) => form.querySelector(id).value.trim();
    const host = field('#chf-rsync-host');
    const port = field('#chf-rsync-port');
    const user = field('#chf-rsync-user');
    const remotePath = field('#chf-rsync-remote-path');
    const identityFile = field('#chf-rsync-identity');
    if (!host || !user || !remotePath) {
        return { error: t('Uploading over SSH needs a host, a user and a remote folder.') };
    }
    const config = { host, user, remotePath };
    if (port) config.port = port;
    if (identityFile) config.identityFile = identityFile;
    return { config };
}

// The website fields — only a site has them, only an online destination has
// a public address. Empty fields are left out.
function _readSiteFields(form, type) {
    const isSite = type === 'site';
    const isOnline = type !== 'files';
    const text = (id) => form.querySelector(id).value.trim() || undefined;
    return {
        siteExport:       isSite ? true : undefined,
        siteTitle:        isSite ? text('#chf-site-title') : undefined,
        siteTheme:        isSite ? (form.querySelector('#chf-site-theme').value || undefined) : undefined,
        siteURL:          isOnline ? text('#chf-site-url') : undefined,
        siteAbout:        isSite ? text('#chf-site-about') : undefined,
        siteImprint:      isSite ? text('#chf-site-imprint') : undefined,
        siteContactEmail: isSite ? text('#chf-site-contact-email') : undefined,
        siteContactURL:   isSite ? text('#chf-site-contact-url') : undefined,
    };
}

// The type: fixed once the destination exists.
function _destTypeFieldsetHTML(isNew, type) {
    return `                    <fieldset class="dest-fieldset">
                        <legend>${t('Type')}</legend>
                        <div class="dest-choices">
                            ${DESTINATION_TYPES.map(dt => `
                                <label class="dest-choice">
                                    <input type="radio" name="dest-type" value="${dt.id}" ${dt.id === type ? 'checked' : ''} ${isNew ? '' : 'disabled'}>
                                    <span><strong>${escapeHtml(t(dt.label))}</strong><span>${escapeHtml(t(dt.hint))}</span></span>
                                </label>`).join('')}
                        </div>
                        ${isNew ? '' : '<p class="form-hint">' + t('The type is fixed after the first publish: it decides how the files are laid out and what the links look like.') + '</p>'}
                    </fieldset>`;
}

// Name, folder name and public address.
function _destNameFieldsetHTML(ch) {
    return `                    <fieldset class="dest-fieldset">
                        <legend>${t('Name and address')}</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-name">${t('Name')}</label>
                                <input class="form-input" id="chf-name" value="${escapeHtml(ch.name)}" placeholder="e.g. Family share links">
                                <span class="form-hint">${t('Folder name:')} <span class="dest-mono" id="chf-slug-preview">${escapeHtml(ch.slug || slugify(ch.name) || '—')}</span></span>
                                <input type="hidden" id="chf-slug" value="${escapeHtml(ch.slug)}">
                            </div>
                            <div class="form-field dest-when-online">
                                <label class="form-label" for="chf-site-url">${t('Public address')}</label>
                                <input class="form-input dest-mono" id="chf-site-url" value="${escapeHtml(ch.siteURL || '')}" placeholder="https://example.com">
                                <span class="form-hint">${t('Used for the links you share and for the reachability check.')}</span>
                            </div>
                        </div>
                    </fieldset>`;
}

// How an online destination gets its files onto the server.
function _destUploadFieldsetHTML(ch, isNew) {
    return `                    <fieldset class="dest-fieldset dest-when-online">
                        <legend>${t('Upload')}</legend>
                        <div class="dest-choices">
                            <label class="dest-choice">
                                <input type="radio" name="dest-upload" value="rsync" ${ch.handler === 'rsync' ? 'checked' : ''}>
                                <span><strong>${t('Upload over SSH')}</strong><span>${t('Unterlumen copies changes to your server with rsync when you publish.')}</span></span>
                            </label>
                            <label class="dest-choice">
                                <input type="radio" name="dest-upload" value="" ${ch.handler === 'rsync' ? '' : 'checked'}>
                                <span><strong>${t('Keep in a local folder')}</strong><span>${t('You upload the folder yourself. Unterlumen cannot tell when it is online.')}</span></span>
                            </label>
                        </div>
                        <div id="chf-rsync-wrap" ${ch.handler === 'rsync' ? '' : 'hidden'}>
                            <div class="dest-grid">
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-host">${t('Host')}</label>
                                    <input class="form-input dest-mono" id="chf-rsync-host" value="${escapeHtml(ch.handlerConfig?.host || '')}" placeholder="example.com">
                                </div>
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-user">${t('User')}</label>
                                    <input class="form-input dest-mono" id="chf-rsync-user" value="${escapeHtml(ch.handlerConfig?.user || '')}" placeholder="deploy">
                                </div>
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-remote-path">${t('Remote folder')}</label>
                                    <input class="form-input dest-mono" id="chf-rsync-remote-path" value="${escapeHtml(ch.handlerConfig?.remotePath || '')}" placeholder="/var/www/photos">
                                </div>
                                <div class="form-field">
                                    <label class="form-label" for="chf-rsync-identity">${t('SSH key')}</label>
                                    <input class="form-input dest-mono" id="chf-rsync-identity" value="${escapeHtml(ch.handlerConfig?.identityFile || '')}" placeholder="~/.ssh/id_ed25519">
                                    <span class="form-hint">${t('Optional. Leave empty to use your default key or agent; the port lives under Advanced.')}</span>
                                </div>
                            </div>
                            <div class="ch-rsync-test-row">
                                <button type="button" class="btn btn-sm" id="chf-rsync-test"${isNew ? ' disabled' : ''}>${t('Test connection')}</button>
                                <span class="ch-rsync-test-result" id="chf-rsync-test-result">${isNew ? t('Create the destination first, then test the connection.') : ''}</span>
                            </div>
                            <div class="form-hint ch-rsync-help">
                                ${t('Deploy uses your system {ssh}/{rsync}, authenticated by key — no passwords are stored.', { ssh: '<code>ssh</code>', rsync: '<code>rsync</code>' })}
                                <ol>
                                    <li>${t('No key yet?')} <code>ssh-keygen -t ed25519</code>.</li>
                                    <li>${t('Copy it to the server:')} <code>ssh-copy-id -i ~/.ssh/id_ed25519.pub user@host</code>.</li>
                                    <li>${t('Connect once by hand — {command} — so the host key is trusted.', { command: '<code>ssh user@host</code>' })}</li>
                                </ol>
                            </div>
                        </div>
                    </fieldset>`;
}

// Format, quality, size and metadata of the exported files.
function _destImageFieldsetHTML(ch) {
    return `                    <fieldset class="dest-fieldset">
                        <legend>${t('Image files')}</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-format">${t('Format')}</label>
                                <select class="form-select" id="chf-format">
                                    <option value="jpeg" ${ch.format === 'jpeg' ? 'selected' : ''}>${t('JPEG')}</option>
                                    <option value="png" ${ch.format === 'png' ? 'selected' : ''}>${t('PNG')}</option>
                                    <option value="webp" ${ch.format === 'webp' ? 'selected' : ''}>${t('WebP')}</option>
                                </select>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-quality">${t('Quality')}</label>
                                <input class="form-input" id="chf-quality" type="number" min="1" max="100" value="${ch.quality}">
                                <span class="form-hint">1–100.</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-scale-mode">${t('Size')}</label>
                                <select class="form-select" id="chf-scale-mode">
                                    <option value="none" ${ch.scale?.mode === 'none' ? 'selected' : ''}>${t('Original size')}</option>
                                    <option value="max_dim" ${ch.scale?.mode === 'max_dim' ? 'selected' : ''}>${t('Max dimension')}</option>
                                    <option value="percent" ${ch.scale?.mode === 'percent' ? 'selected' : ''}>${t('Percent')}</option>
                                </select>
                                <div id="chf-scale-opts" class="form-scale-opts">${_scaleOptsHTML(ch.scale)}</div>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-exif">${t('Metadata')}</label>
                                <select class="form-select" id="chf-exif">
                                    <option value="keep_no_gps" ${ch.exifMode === 'keep_no_gps' ? 'selected' : ''}>${t('Keep, without GPS')}</option>
                                    <option value="keep" ${ch.exifMode === 'keep' ? 'selected' : ''}>${t('Keep all')}</option>
                                    <option value="strip" ${ch.exifMode === 'strip' ? 'selected' : ''}>${t('Remove all')}</option>
                                </select>
                            </div>
                        </div>
                    </fieldset>`;
}

// Where a files destination puts its files.
function _destOutputFieldsetHTML(ch) {
    return `                    <fieldset class="dest-fieldset dest-when-files">
                        <legend>${t('Output')}</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-output-mode">${t('Where the files go')}</label>
                                <select class="form-select" id="chf-output-mode">
                                    <option value="save" ${(ch.outputMode || 'save') === 'save' ? 'selected' : ''}>${t('Into a folder')}</option>
                                    <option value="download" ${ch.outputMode === 'download' ? 'selected' : ''}>${t('Download as a ZIP')}</option>
                                </select>
                            </div>
                            <div class="form-field" id="chf-output-path-wrap" ${ch.outputMode === 'download' ? 'hidden' : ''}>
                                <label class="form-label" for="chf-output-path">${t('Folder')}</label>
                                <div class="export-destination-wrap">
                                    <input class="form-input export-destination-input dest-mono" id="chf-output-path"
                                           value="${escapeHtml(ch.outputPath ? (ch.outputDir || ch.outputPath) : '')}"
                                           placeholder="~/.unterlumen/channels/${escapeHtml(ch.slug || '<name>')}/">
                                    <button type="button" class="btn btn-sm" id="chf-output-pick" title="${t('Browse folders')}">…</button>
                                </div>
                            </div>
                        </div>
                    </fieldset>`;
}

// Title, theme, logo and pages of a website.
function _destSiteFieldsetHTML(ch) {
    return `                    <fieldset class="dest-fieldset dest-when-site">
                        <legend>${t('Website')}</legend>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-site-title">${t('Site title')}</label>
                                <input class="form-input" id="chf-site-title" value="${escapeHtml(ch.siteTitle || '')}" placeholder="e.g. My Photography">
                                <span class="form-hint">${t('Shown in the header on every page.')}</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-site-theme">${t('Default theme')}</label>
                                <select class="form-select" id="chf-site-theme">
                                    <option value="light" ${(ch.siteTheme || 'light') === 'light' ? 'selected' : ''}>${t('Light')}</option>
                                    <option value="dark" ${ch.siteTheme === 'dark' ? 'selected' : ''}>${t('Dark')}</option>
                                </select>
                                <span class="form-hint">${t('Visitors can switch.')}</span>
                            </div>
                        </div>
                        <div class="form-field">
                            <span class="form-label">${t('Site logo')}</span>
                            <div class="ch-avatar-wrap" id="chf-logo-wrap">
                                <span class="ch-avatar-status" id="chf-logo-status"></span>
                                <input type="file" id="chf-logo-file" accept="image/*" style="display:none">
                                <button class="btn btn-sm" id="chf-logo-upload">${t('Upload logo')}</button>
                                <button class="btn btn-sm" id="chf-logo-remove" style="display:none">${t('Remove')}</button>
                            </div>
                        </div>
                        <div class="form-field">
                            <label class="form-label" for="chf-site-about">${t('About page')}</label>
                            <textarea class="form-input dest-textarea" id="chf-site-about" rows="6" placeholder="${t('Write a short introduction about yourself and your photography…')}">${escapeHtml(ch.siteAbout || '')}</textarea>
                            <span class="form-hint">${t('Markdown. Becomes about.html.')}</span>
                        </div>
                        <div class="form-field">
                            <span class="form-label">${t('Author photo')}</span>
                            <div class="ch-avatar-wrap" id="chf-avatar-wrap">
                                <span class="ch-avatar-status" id="chf-avatar-status"></span>
                                <input type="file" id="chf-avatar-file" accept="image/*" style="display:none">
                                <button class="btn btn-sm" id="chf-avatar-upload">${t('Upload photo')}</button>
                                <button class="btn btn-sm" id="chf-avatar-remove" style="display:none">${t('Remove')}</button>
                            </div>
                            <span class="form-hint">${t('Shown on the About page.')}</span>
                        </div>
                        <div class="form-field">
                            <label class="form-label" for="chf-site-imprint">${t('Legal / imprint')}</label>
                            <textarea class="form-input dest-textarea" id="chf-site-imprint" rows="6" placeholder="${t('Responsible for this website:&#10;Your Name&#10;Your Address…')}">${escapeHtml(ch.siteImprint || '')}</textarea>
                            <span class="form-hint">${t('Markdown. Becomes legal.html.')}</span>
                        </div>
                        <div class="dest-grid">
                            <div class="form-field">
                                <label class="form-label" for="chf-site-contact-email">${t('Contact email')}</label>
                                <input class="form-input" id="chf-site-contact-email" type="email" value="${escapeHtml(ch.siteContactEmail || '')}" placeholder="you@example.com">
                                <span class="form-hint">${t('Shown in the footer of every page.')}</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-site-contact-url">${t('Website or social address')}</label>
                                <input class="form-input" id="chf-site-contact-url" type="url" value="${escapeHtml(ch.siteContactURL || '')}" placeholder="https://yoursite.com">
                            </div>
                        </div>
                    </fieldset>`;
}

// Handler settings, accounts and maintenance.
function _destAdvancedHTML(ch, isNew) {
    return `                    <details class="dest-advanced">
                        <summary>${t('Advanced: accounts, custom handler settings, rebuild')}</summary>
                        <div class="dest-advanced-body">
                            <div class="form-field">
                                <label class="form-label" for="chf-handler">${t('Handler')}</label>
                                <select class="form-select" id="chf-handler">
                                    <option value="" ${!ch.handler ? 'selected' : ''}>${t('None')}</option>
                                    <option value="rsync" ${ch.handler === 'rsync' ? 'selected' : ''}>rsync (deploy over SSH)</option>
                                    ${(ch.handler && ch.handler !== 'rsync') ? `<option value="${escapeHtml(ch.handler)}" selected>${escapeHtml(ch.handler)} (existing)</option>` : ''}
                                </select>
                                <span class="form-hint">${t('Set by the Upload section above; change it here only for a handler Unterlumen has no form for.')}</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-rsync-port">${t('Port')}</label>
                                <input class="form-input" id="chf-rsync-port" type="number" min="1" max="65535" value="${escapeHtml(ch.handlerConfig?.port || '')}" placeholder="22">
                            </div>
                            <div class="form-field" id="chf-generic-hconfig-wrap" ${ch.handler === 'rsync' ? 'hidden' : ''}>
                                <span class="form-label">${t('Handler config')}</span>
                                <div id="chf-hconfig" class="kv-editor">${_kvEditorHTML(ch.handler === 'rsync' ? {} : (ch.handlerConfig || {}))}</div>
                                <button class="btn btn-sm" id="chf-hconfig-add">+ Add config entry</button>
                            </div>
                            <div class="form-field">
                                <span class="form-label">${t('Accounts')}</span>
                                <div id="chf-accounts" class="accounts-editor">${_accountsEditorHTML(ch.accounts || [])}</div>
                                <button class="btn btn-sm" id="chf-account-add">+ Add account</button>
                                <span class="form-hint">${t('Named sub-accounts, e.g. two Mastodon logins.')}</span>
                            </div>
                            ${(isNew || !ch.siteExport) ? '' : `
                            <div class="form-field">
                                <span class="form-label">${t('Album list')}</span>
                                <div class="dest-maintenance">
                                    <button class="btn btn-sm" id="dest-rebuild-albums">${t('Rebuild album list')}</button>
                                </div>
                                <span class="form-hint">${t('Adds albums that are missing from the list shared by your installations, from what the photos themselves record. Albums already listed stay as they are. Only photos in a library that has been scanned are looked at.')}</span>
                                <div id="dest-rebuild-albums-report" class="dest-report" role="status" aria-live="polite"></div>
                            </div>`}
                            ${isNew ? '' : `
                            <div class="form-field">
                                <span class="form-label">${t('Maintenance')}</span>
                                <div class="dest-maintenance">
                                    <button class="btn btn-sm" id="dest-reveal">${t('Show output in Finder/Explorer')}</button>
                                    <button class="btn btn-sm btn-danger" id="dest-delete">${t('Delete destination…')}</button>
                                </div>
                                <span class="form-hint">${t('Deleting removes the destination from your configuration. Galleries already published to it stay where they are.')}</span>
                                <div id="dest-delete-confirm"></div>
                            </div>`}
                        </div>
                    </details>`;
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
        const rows = (this._channels || []).map(ch => {
            const type = destinationTypeLabel(ch);
            const base = _deployBaseURL(ch);
            const address = base
                ? `<a href="${escapeHtml(base.url)}" target="_blank" rel="noopener" class="dest-visit">${escapeHtml(this._addressDesc(ch))}</a>`
                : escapeHtml(this._addressDesc(ch));
            const files = _scaleDesc(ch.scale) ? `${ch.format.toUpperCase()} · q${ch.quality} · ${_scaleDesc(ch.scale)}` : `${ch.format.toUpperCase()} · q${ch.quality}`;
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
                    ? `<button class="btn btn-sm dest-galleries-link" data-slug="${escapeHtml(ch.slug)}">${n} ${n === 1 ? 'gallery' : 'galleries'}</button>`
                    : '<span class="gal-row-sub">' + t('No galleries') + '</span>'}</span>
            </div>`;
        }).join('');

        this.container.innerHTML = `
            <div class="dest-pane">
                ${placeHeaderHTML({
                    title: t('Destinations'),
                    meta: `${(this._channels || []).length} ${(this._channels || []).length === 1 ? 'destination' : 'destinations'}`,
                    actions: '<button class="btn btn-sm" id="dest-new">' + t('New destination…') + '</button>',
                    lede: placeLede(t('A destination is where {galleries} go — a website, share links for family and friends, or plain files you post yourself — and how their files are made.', { galleries: placeLink('published', 'galleries', t('galleries')) })),
                })}
                <div class="gal-body">
                    ${(this._channels || []).length === 0
                        ? '<p class="gal-empty">' + t('No destinations yet. Choose New destination… to say where your first gallery should go.') + '</p>'
                        : `<div class="dest-list">${rows}</div>`}
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
        if (!isNew) showPhotosLink(root.querySelector('#dest-show-photos'), { channel: ch.slug });

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
            const chosen = await new FolderPicker().open(current || '', { title: t('Output folder') });
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
            if (ch.siteExport) this._wireRebuildAlbums(root, ch);
        } else {
            form.querySelector('#chf-logo-status').textContent = t('Create the destination first, then upload a logo.');
            form.querySelector('#chf-logo-upload').disabled = true;
            form.querySelector('#chf-avatar-status').textContent = t('Create the destination first, then upload a photo.');
            form.querySelector('#chf-avatar-upload').disabled = true;
        }

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

    _wireRebuildAlbums(root, ch) {
        const btn = root.querySelector('#dest-rebuild-albums');
        const out = root.querySelector('#dest-rebuild-albums-report');
        btn.addEventListener('click', async () => {
            const restore = Activity.button(btn, t('Rebuilding…'));
            Activity.in(out, t('Reading the photos…'));
            try {
                out.innerHTML = this._albumReportHTML(await ChannelAPI.rebuildAlbumList(ch.slug));
            } catch (err) {
                out.innerHTML = `<div class="gal-detail-error">${t('Could not rebuild the album list: {error}', { error: escapeHtml(err.message) })}</div>`;
            } finally {
                restore();
            }
        });
    }

    // What the rebuild found, in plain sentences: what it added, what it
    // completed, and what it left alone and why.
    _albumReportHTML(r) {
        const plural = (n, one, many) => countText(n, one, many);
        const found = r.added.length + r.present + r.unreadable.length + r.deleted;
        if (found === 0 && r.photos.length === 0 && !r.sidecarsCompleted && !r.unreachable) {
            return '<p>' + t('No albums found in the photos of this destination.') + '</p>';
        }
        const left = r.deleted ? ' ' + t('{n} deleted earlier, left out.', { n: formatCount(r.deleted) }) : '';
        const parts = [`<p>${t('Found {albums} in the photos. Added {added}; {present} already listed.', { albums: plural(found, 'album', 'albums'), added: formatCount(r.added.length), present: formatCount(r.present) })}${left}</p>`];
        if (r.added.length) {
            parts.push(`<ul>${r.added.map(a => `<li>${escapeHtml(a.title)} <span class="dest-report-data">/${escapeHtml(a.slug)}</span>, ${plural(a.photos, 'photo', 'photos')}</li>`).join('')}</ul>`);
        }
        if (r.sidecarsCompleted) {
            parts.push(`<p>${t('Completed {sidecars} with the listed albums’ membership and address, so those albums can be restored from the photos too.', { sidecars: plural(r.sidecarsCompleted, 'photo sidecar', 'photo sidecars') })}</p>`);
        }
        if (r.unreachable) {
            parts.push(`<p>${t('{photos} of the listed albums could not be reached from here (not in a scanned library, not mounted, or an old entry without a photo). Their sidecars were not touched.', { photos: plural(r.unreachable, 'photo', 'photos') })}</p>`);
        }
        if (r.unreadable.length) {
            // Publications the list does not know and that carry no address:
            // in practice albums that were deleted or replaced before deleting
            // also cleaned up the photos. Left as they are, grouped by title.
            const n = r.unreadable.length;
            const groups = new Map(); // label (HTML) -> count; an ID is data, a title is text
            for (const u of r.unreadable) {
                const label = u.title
                    ? escapeHtml(u.title)
                    : `<span class="dest-report-data">${escapeHtml(u.postID)}</span>`;
                groups.set(label, (groups.get(label) || 0) + 1);
            }
            parts.push(`<p>${n === 1 ? t('1 older publication is not in the list and has no recorded address, so it cannot be restored without inventing one. Usually these are albums that were deleted or replaced. They are left as they are.') : t('{n} older publications are not in the list and have no recorded address, so they cannot be restored without inventing one. Usually these are albums that were deleted or replaced. They are left as they are.', { n: formatCount(n) })}</p>`);
            parts.push(`<details class="dest-report-details"><summary>${n === 1 ? t('Show it') : t('Show them')}</summary><ul>${[...groups].map(([label, count]) => `<li>${label}${count > 1 ? ` <span class="dest-report-data">×${count}</span>` : ''}</li>`).join('')}</ul></details>`);
        }
        if (r.photos.length) {
            const shown = r.photos.slice(0, 10);
            parts.push(`<p class="dest-report-warning">${t('{photos} could not be read:', { photos: plural(r.photos.length, 'photo', 'photos') })}</p>`);
            parts.push(`<ul>${shown.map(p => `<li><span class="dest-report-data">${escapeHtml(p.filename)}</span>: ${escapeHtml(p.reason)}</li>`).join('')}${r.photos.length > shown.length ? `<li>${t('and {n} more', { n: formatCount(r.photos.length - shown.length) })}</li>` : ''}</ul>`);
        }
        return parts.join('');
    }

    _wireDelete(root, ch) {
        const wrap = root.querySelector('#dest-delete-confirm');
        root.querySelector('#dest-delete').addEventListener('click', () => {
            const count = this._galleryCount(ch.slug);
            wrap.innerHTML = `
                <p class="gal-danger-question">Delete "${escapeHtml(ch.name)}"? ${count > 0
                    ? `Its ${count} gallerie${count !== 1 ? 's' : ''} stay on disk and stay online; they just lose the destination that describes them.`
                    : t('It holds no galleries.')}</p>
                <div class="gal-danger-actions">
                    <button class="btn btn-sm" id="dest-delete-cancel">${t('Keep it')}</button>
                    <button class="btn btn-sm btn-danger" id="dest-delete-confirm-btn">${t('Delete destination')}</button>
                </div>`;
            wrap.querySelector('#dest-delete-cancel').addEventListener('click', () => { wrap.innerHTML = ''; });
            wrap.querySelector('#dest-delete-confirm-btn').addEventListener('click', async (e) => {
                Activity.button(e.currentTarget, t('Deleting…'));
                wrap.querySelector('#dest-delete-cancel').disabled = true;
                try {
                    await ChannelAPI.delete(ch.slug);
                    this._editing = undefined;
                    await this._load();
                } catch (err) {
                    wrap.innerHTML = `<div class="gal-detail-error">${t('Could not delete it: {error}', { error: escapeHtml(err.message) })}</div>`;
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
                statusEl.textContent = exists ? t('Logo uploaded.') : t('No logo set.');
                removeBtn.style.display = exists ? '' : 'none';
            } catch {
                statusEl.textContent = t('Could not check logo.');
            }
        };

        const rebuildIfSite = async () => {
            if (!ch.siteExport) return;
            statusEl.textContent = t('Rebuilding the site…');
            try { await ChannelAPI.rebuildSite(slug); } catch {
                statusEl.textContent = t('Logo saved, but the site did not rebuild. Open Galleries and publish one of its galleries again to rebuild it.');
            }
        };

        await refresh();

        uploadBtn.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async () => {
            const file = fileInput.files[0];
            if (!file) return;
            statusEl.textContent = t('Uploading…');
            uploadBtn.disabled = true;
            try {
                await ChannelAPI.uploadLogo(slug, file);
                await rebuildIfSite();
                await refresh();
            } catch (err) {
                statusEl.textContent = t('Upload failed: ') + err.message;
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
                statusEl.textContent = t('Remove failed: ') + err.message;
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
                statusEl.textContent = exists ? t('Portrait uploaded.') : t('No portrait set.');
                removeBtn.style.display = exists ? '' : 'none';
            } catch {
                statusEl.textContent = t('Could not check avatar.');
            }
        };

        const rebuildIfSite = async () => {
            if (!ch.siteExport) return;
            statusEl.textContent = t('Rebuilding the site…');
            try { await ChannelAPI.rebuildSite(slug); } catch {
                statusEl.textContent = t('Portrait saved, but the site did not rebuild. Open Galleries and publish one of its galleries again to rebuild it.');
            }
        };

        await refresh();

        uploadBtn.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async () => {
            const file = fileInput.files[0];
            if (!file) return;
            statusEl.textContent = t('Uploading…');
            uploadBtn.disabled = true;
            try {
                await ChannelAPI.uploadAvatar(slug, file);
                await rebuildIfSite();
                await refresh();
            } catch (err) {
                statusEl.textContent = t('Upload failed: ') + err.message;
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
                statusEl.textContent = t('Remove failed: ') + err.message;
            } finally {
                removeBtn.disabled = false;
            }
        });
    }
}
