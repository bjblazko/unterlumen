// publish-dialog.js — Publish is one action (ADR-0029).
//
// "Generate" and "Deploy" are technique, not intent. One press makes the
// online version equal to what you see: export the photos, build the pages,
// upload them where an upload is configured, then check the link. Progress is
// determinate and comes from the buildStream events; errors appear in the
// sheet, never in an alert().
//
// The frame is the shared Dialog (ADR-0033): Escape, the scrim, the focus and
// the footer belong to it, so this file only knows about publishing.
class PublishDialog {
    constructor() {
        this._el = null;
        this._done = false;
    }

    // row: one item from GET /api/channels/galleries —
    // { channelSlug, channelName, channelHandler, status, postID, draftID,
    //   title, pendingCount, publishedAt, url, urlGuessed, generatedAt,
    //   deployedAt }
    //
    // Resolves once the sheet closes, so the caller can reload its list.
    open(row) {
        this._row = row;
        this._dialog = new Dialog({
            title: t('Publish'),
            subtitle: row.title || row.channelName,
            size: 'md',
            className: 'publish-dialog',
            body: '<div id="pub-body"></div>',
            actions: [],
            onClose: () => {
                this._el = null;
                this._resolveClosed?.({ published: this._done });
            },
        });
        this._el = this._dialog.open();

        this._closed = new Promise(resolve => { this._resolveClosed = resolve; });
        this._renderConfirmStep();
        return this._closed;
    }

    close() {
        this._dialog?.close(null);
    }

    // Everything that is about to happen, in the order it happens, plus the
    // one date field. No step the user has to drive by hand.
    async _renderConfirmStep() {
        const body = this._el.querySelector('#pub-body');
        Activity.in(body, t('Reading the collected photos…'));
        const draft = await this._loadDraft(body);
        if (draft === undefined) return;
        this._draft = draft;
        const pending = draft?.photos?.length ?? 0;
        body.innerHTML = this._planHTML(pending, this._row.channelHandler === 'rsync');
        this._dialog.setActions([
            { label: t('Cancel'), id: 'pub-cancel', onClick: () => this.close() },
            {
                label: pending > 0 ? t('Publish {photos}', { photos: countText(pending, 'photo') }) : t('Publish'),
                kind: 'primary',
                id: 'pub-run',
                onClick: () => this._run(),
            },
        ]);
    }

    // The draft with the photos waiting; null when the gallery has none,
    // undefined when it could not be read (and the body says so).
    async _loadDraft(body) {
        if (!this._row.draftID) return null;
        try {
            const drafts = await ChannelAPI.listDrafts(this._row.channelSlug);
            return drafts.find(d => d.id === this._row.draftID) || null;
        } catch (err) {
            body.innerHTML = `<div class="channel-error">${t('Could not load the collected photos: {error}', { error: escapeHtml(err.message) })}</div>`;
            return undefined;
        }
    }

    // The date belongs to the gallery, not to this publish run: default to
    // the date it already carries, so publishing changes does not silently
    // re-date a gallery (ADR-0029).
    _galleryDate() {
        const existing = this._row.postID && this._row.publishedAt ? new Date(this._row.publishedAt) : null;
        const valid = existing && !isNaN(existing) && existing.getFullYear() > 1970 ? existing : null;
        return (valid || new Date()).toISOString().slice(0, 10);
    }

    _planHTML(pending, willUpload) {
        const steps = [
            pending > 0 ? t('Export {photos}', { photos: countText(pending, 'photo') }) : t('Rebuild the exported photos'),
            t('Build the pages'),
            willUpload ? t('Upload to {host}', { host: escapeHtml(this._uploadHost() || t('the configured host')) }) : null,
            this._row.url ? t('Check the link') : null,
        ].filter(Boolean);
        const waiting = pending === 0 ? t('Nothing is waiting. Publishing rebuilds this gallery as it is.')
            : pending === 1 ? t('1 photo is waiting to go online.')
            : t('{n} photos are waiting to go online.', { n: formatCount(pending) });
        return `
            <p class="form-hint">${waiting}</p>
            <ol class="publish-plan">${steps.map(s => `<li>${s}</li>`).join('')}</ol>
            ${willUpload ? '' : '<p class="form-hint">' + t('This destination has no upload configured, so the files stay in the local output folder.') + '</p>'}
            <label class="form-label" for="pub-date">${this._row.postID ? t('Date shown on the gallery') : t('Published date')}</label>
            <input class="form-input" id="pub-date" type="date" value="${this._galleryDate()}">
            <span class="build-date-note">${this._row.postID
                ? t('Stays as it is unless you change it here. Stored in XMP sidecars on the newly added photos.')
                : t('Sets album order in the built site and is stored in XMP sidecars.')}</span>
            `;
    }

    _uploadHost() {
        return this._row.url ? this._row.url.replace(/^https?:\/\//, '').split('/')[0] : '';
    }

    // One run: export → build → upload → check. Each step reports what it is
    // doing; nothing is faked, and a step that is not part of this run is not
    // shown.
    async _run() {
        const body = this._el.querySelector('#pub-body');
        const dateVal = body.querySelector('#pub-date')?.value;
        const publishedAt = dateVal ? new Date(dateVal + 'T12:00:00Z').toISOString() : undefined;
        const willUpload = this._row.channelHandler === 'rsync';
        this._showSteps(body, willUpload);

        const result = await this._export(publishedAt);
        if (!result) return;
        this._result = result;
        const failed = (result.results || []).filter(r => r.Error || r.error);
        if (failed.length) {
            this._setStep('export', 'failed', t('{photos} could not be exported', { photos: countText(failed.length, 'photo') }));
        }
        if (willUpload && !await this._upload()) return;
        if (this._row.url) await this._check();
        this._done = true;
        this._renderResult(failed);
    }

    // The steps of this run, each waiting to report; a step that is not part
    // of the run is not shown.
    _showSteps(body, willUpload) {
        this._steps = [
            { id: 'export', label: t('Exporting photos') },
            { id: 'build', label: t('Building the pages') },
            ...(willUpload ? [{ id: 'upload', label: t('Uploading') }] : []),
            ...(this._row.url ? [{ id: 'check', label: t('Checking the link') }] : []),
        ];
        body.innerHTML = `
            <ol class="publish-steps" id="pub-steps">
                ${this._steps.map(s => `
                    <li class="publish-step" data-step="${s.id}">
                        <span class="publish-step-mark" aria-hidden="true"></span>
                        <span class="publish-step-label">${escapeHtml(s.label)}</span>
                        <span class="publish-step-detail"></span>
                    </li>`).join('')}
            </ol>
            <div class="publish-error" id="pub-error" hidden></div>`;
        this._dialog.setActions([]);
    }

    // Exports the photos and builds the pages, reporting each photo; returns
    // the server's result, or nothing when it did not finish.
    async _export(publishedAt) {
        this._setStep('export', 'doing');
        try {
            const result = await ChannelAPI.generateStream(
                this._row.channelSlug, this._row.draftID || '-', this._row.postID, { publishedAt },
                (evt) => {
                    if (evt.step === 'photo') {
                        this._setStep('export', 'doing', t('{done} of {total}', { done: formatCount(evt.done), total: formatCount(evt.total) }));
                    } else {
                        this._setStep('export', 'done');
                        this._setStep('build', 'doing', evt.file || '');
                    }
                },
            );
            this._setStep('export', 'done');
            this._setStep('build', 'done');
            return result;
        } catch (err) {
            this._setStep('export', 'failed');
            this._fail(t('The export did not finish.'), err.message, [{ label: t('Try again'), run: () => this._run() }]);
            return null;
        }
    }

    async _upload() {
        this._setStep('upload', 'doing');
        try {
            const res = await ChannelAPI.deploy(this._row.channelSlug);
            if (!res.ok) throw new Error(res.error || t('the upload reported no reason'));
            this._setStep('upload', 'done');
            return true;
        } catch (err) {
            this._setStep('upload', 'failed');
            // The build survives a failed upload, which is exactly the state
            // "Built, not uploaded" — so the retry only uploads.
            this._fail(
                t('The pages are built, but the upload did not go through.'),
                err.message,
                [{ label: t('Retry upload'), run: async () => { this._el.querySelector('#pub-error').hidden = true; if (await this._upload()) { if (this._row.url) await this._check(); this._done = true; this._renderResult([]); } } }],
            );
            return false;
        }
    }

    async _check() {
        this._setStep('check', 'doing');
        try {
            let reachable = false;
            await PublishedGalleryAPI.checkReachability(
                [{ channelSlug: this._row.channelSlug, postID: this._row.postID || this._result?.postID || '', rowKey: this._row.rowKey, url: this._row.url }],
                (evt) => { reachable = !!evt.reachable; this._checkError = evt.error; },
            );
            this._reachable = reachable;
            this._setStep('check', reachable ? 'done' : 'failed', reachable ? t('answered') : t('no answer'));
        } catch {
            this._reachable = false;
            this._setStep('check', 'failed', t('could not be checked'));
        }
    }

    _setStep(id, state, detail) {
        const el = this._el?.querySelector(`.publish-step[data-step="${id}"]`);
        if (!el) return;
        el.dataset.state = state;
        if (detail !== undefined) el.querySelector('.publish-step-detail').textContent = detail;
    }

    // What happened, why if known, and what to do next — in the sheet.
    _fail(what, why, actions) {
        const errEl = this._el.querySelector('#pub-error');
        errEl.innerHTML = `<strong>${escapeHtml(what)}</strong><span>${escapeHtml(why || '')}</span>`;
        errEl.hidden = false;
        this._dialog.setActions([
            { label: t('Close'), onClick: () => this.close() },
            ...actions.map(action => ({ label: action.label, kind: 'primary', onClick: () => action.run() })),
        ]);
    }

    _renderResult(failed) {
        const body = this._el.querySelector('#pub-body');
        const localPath = this._result.sitePath || this._result.galleryPath || this._result.outputPath;
        this._el.querySelector('#pub-error')?.setAttribute('hidden', '');
        body.insertAdjacentHTML('beforeend', this._resultHTML(failed, localPath));
        if (failed.length) {
            body.querySelector('.publish-failed')?.appendChild(Activity.errorList(failed.map(r => r.Error || r.error)));
        }
        this._dialog.setActions([{ label: t('Done'), kind: 'primary', id: 'pub-done', onClick: () => this.close() }]);
        body.querySelector('#pub-copy-path').addEventListener('click', () => navigator.clipboard.writeText(localPath));
        // ChannelAPI.reveal(slug) reveals the channel's own output folder in
        // Finder/Explorer — it takes no path argument (see channels.js).
        body.querySelector('#pub-open-folder').addEventListener('click', () => ChannelAPI.reveal(this._row.channelSlug));
        body.querySelector('#pub-preview')?.addEventListener('click', () => {
            window.open('file://' + localPath + '/index.html', '_blank');
        });
    }

    // Where the run ended: in the local folder, online, or uploaded with a
    // link that did not answer yet.
    _resultSummary() {
        if (this._row.channelHandler !== 'rsync') return t('Exported to the local output folder. Putting the files anywhere is up to you.');
        if (!this._row.url) return t('Uploaded.');
        return this._reachable
            ? t('Online. The link answered just now.')
            : t('Uploaded, but the link did not answer. It can take a moment for the host to serve the new files.');
    }

    _resultHTML(failed, localPath) {
        const showsHTML = !!(this._result.sitePath || this._result.galleryPath);
        const url = this._row.url;
        return `
            <p class="publish-summary">${escapeHtml(this._resultSummary())}</p>
            ${failed.length ? `<div class="publish-failed"><p class="form-hint">${t('{photos} could not be exported and stayed in the gallery’s collected photos.', { photos: countText(failed.length, 'photo') })}</p></div>` : ''}
            <div class="publish-step-review">
                <div class="build-destination">${escapeHtml(url || localPath)}</div>
                <div class="publish-step-review-actions">
                    ${url ? `<a class="btn btn-sm" href="${escapeHtml(url)}" target="_blank" rel="noopener">${t('Open in browser')}</a>` : ''}
                    <button class="btn btn-sm" id="pub-copy-path">${t('Copy path')}</button>
                    <button class="btn btn-sm" id="pub-open-folder">${t('Open in Finder/Explorer')}</button>
                    ${showsHTML ? '<button class="btn btn-sm" id="pub-preview">' + t('Preview locally') + '</button>' : ''}
                </div>
            </div>`;
    }
}
