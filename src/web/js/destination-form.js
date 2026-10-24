// destination-form.js — the form of a destination: what it reads back and the
// fieldsets it is made of, each shown only for the types that need it.

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
                            ${_destHandlerFieldsHTML(ch)}
                            <div class="form-field">
                                <span class="form-label">${t('Accounts')}</span>
                                <div id="chf-accounts" class="accounts-editor">${_accountsEditorHTML(ch.accounts || [])}</div>
                                <button class="btn btn-sm" id="chf-account-add">${t('+ Add account')}</button>
                                <span class="form-hint">${t('Named sub-accounts, e.g. two Mastodon logins.')}</span>
                            </div>
                            ${isNew || !ch.siteExport ? '' : _destAlbumListFieldHTML()}
                            ${isNew ? '' : _destMaintenanceFieldHTML()}
                        </div>
                    </details>`;
}

function _destHandlerFieldsHTML(ch) {
    const rsync = ch.handler === 'rsync';
    const other = ch.handler && !rsync;
    return `<div class="form-field">
                                <label class="form-label" for="chf-handler">${t('Handler')}</label>
                                <select class="form-select" id="chf-handler">
                                    <option value="" ${!ch.handler ? 'selected' : ''}>${t('None')}</option>
                                    <option value="rsync" ${rsync ? 'selected' : ''}>${t('rsync (deploy over SSH)')}</option>
                                    ${other ? `<option value="${escapeHtml(ch.handler)}" selected>${t('{handler} (existing)', { handler: escapeHtml(ch.handler) })}</option>` : ''}
                                </select>
                                <span class="form-hint">${t('Set by the Upload section above; change it here only for a handler Unterlumen has no form for.')}</span>
                            </div>
                            <div class="form-field">
                                <label class="form-label" for="chf-rsync-port">${t('Port')}</label>
                                <input class="form-input" id="chf-rsync-port" type="number" min="1" max="65535" value="${escapeHtml(ch.handlerConfig?.port || '')}" placeholder="22">
                            </div>
                            <div class="form-field" id="chf-generic-hconfig-wrap" ${rsync ? 'hidden' : ''}>
                                <span class="form-label">${t('Handler config')}</span>
                                <div id="chf-hconfig" class="kv-editor">${_kvEditorHTML(rsync ? {} : (ch.handlerConfig || {}))}</div>
                                <button class="btn btn-sm" id="chf-hconfig-add">${t('+ Add config entry')}</button>
                            </div>`;
}

function _destAlbumListFieldHTML() {
    return `
                            <div class="form-field">
                                <span class="form-label">${t('Album list')}</span>
                                <div class="dest-maintenance">
                                    <button class="btn btn-sm" id="dest-rebuild-albums">${t('Rebuild album list')}</button>
                                </div>
                                <span class="form-hint">${t('Adds albums that are missing from the list shared by your installations, from what the photos themselves record. Albums already listed stay as they are. Only photos in a library that has been scanned are looked at.')}</span>
                                <div id="dest-rebuild-albums-report" class="dest-report" role="status" aria-live="polite"></div>
                            </div>`;
}

function _destMaintenanceFieldHTML() {
    return `
                            <div class="form-field">
                                <span class="form-label">${t('Maintenance')}</span>
                                <div class="dest-maintenance">
                                    <button class="btn btn-sm" id="dest-reveal">${t('Show output in Finder/Explorer')}</button>
                                    <button class="btn btn-sm btn-danger" id="dest-delete">${t('Delete destination…')}</button>
                                </div>
                                <span class="form-hint">${t('Deleting removes the destination from your configuration. Galleries already published to it stay where they are.')}</span>
                                <div id="dest-delete-confirm"></div>
                            </div>`;
}

// How the form's fields follow each other: the type decides which sections
// exist, the name the folder name, the upload choice the handler.
function wireDestinationFields(form, isNew) {
    for (const radio of form.querySelectorAll('input[name="dest-type"]')) {
        radio.addEventListener('change', () => { form.dataset.type = radio.value; });
    }
    // The slug is not an editable field: changing it would move published
    // output, so a new destination's folder name follows its name.
    if (isNew) {
        const nameEl = form.querySelector('#chf-name');
        nameEl.addEventListener('input', () => {
            form.querySelector('#chf-slug').value = slugify(nameEl.value);
            form.querySelector('#chf-slug-preview').textContent = slugify(nameEl.value) || '—';
        });
    }
    wireUploadChoice(form);
    wireOutputFields(form);
    form.querySelector('#chf-hconfig-add').addEventListener('click', () => {
        form.querySelector('#chf-hconfig').insertAdjacentHTML('beforeend', _kvRowHTML('', ''));
    });
    form.querySelector('#chf-account-add').addEventListener('click', () => {
        form.querySelector('#chf-accounts').insertAdjacentHTML('beforeend', _accountRowHTML({ id: '', label: '', config: {} }));
    });
}

// The upload choice and the handler select are two views of one setting.
function wireUploadChoice(form) {
    const handlerEl = form.querySelector('#chf-handler');
    const show = (isRsync) => {
        form.querySelector('#chf-rsync-wrap').hidden = !isRsync;
        form.querySelector('#chf-generic-hconfig-wrap').hidden = isRsync;
    };
    for (const radio of form.querySelectorAll('input[name="dest-upload"]')) {
        radio.addEventListener('change', () => {
            handlerEl.value = radio.value;
            show(radio.value === 'rsync');
        });
    }
    handlerEl.addEventListener('change', () => {
        const isRsync = handlerEl.value === 'rsync';
        show(isRsync);
        const radio = form.querySelector(`input[name="dest-upload"][value="${isRsync ? 'rsync' : ''}"]`);
        if (radio) radio.checked = true;
    });
}

// The size options follow the scale mode; the folder fields, the output mode.
function wireOutputFields(form) {
    const scaleMode = form.querySelector('#chf-scale-mode');
    scaleMode.addEventListener('change', () => {
        form.querySelector('#chf-scale-opts').innerHTML = _scaleOptsHTML({ mode: scaleMode.value });
    });
    const outputModeEl = form.querySelector('#chf-output-mode');
    outputModeEl.addEventListener('change', () => {
        form.querySelector('#chf-output-path-wrap').hidden = outputModeEl.value === 'download';
    });
    form.querySelector('#chf-output-pick').addEventListener('click', async () => {
        const pathEl = form.querySelector('#chf-output-path');
        const chosen = await new FolderPicker().open(pathEl.value.trim() || '', { title: t('Output folder') });
        if (chosen !== null) pathEl.value = chosen;
    });
}
