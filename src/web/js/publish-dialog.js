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
            title: 'Publish',
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
        body.innerHTML = '<div class="channel-loading">Loading the collected photos…</div>';
        let draft = null;
        if (this._row.draftID) {
            try {
                const drafts = await ChannelAPI.listDrafts(this._row.channelSlug);
                draft = drafts.find(d => d.id === this._row.draftID) || null;
            } catch (err) {
                body.innerHTML = `<div class="channel-error">Could not load the collected photos: ${escapeHtml(err.message)}</div>`;
                return;
            }
        }
        this._draft = draft;
        const pending = draft?.photos?.length ?? 0;
        const willUpload = this._row.channelHandler === 'rsync';

        // The date belongs to the gallery, not to this publish run: default to
        // the date it already carries, so publishing changes does not silently
        // re-date a gallery (ADR-0029).
        const existing = this._row.postID && this._row.publishedAt ? new Date(this._row.publishedAt) : null;
        const validExisting = existing && !isNaN(existing) && existing.getFullYear() > 1970 ? existing : null;
        const dateValue = (validExisting || new Date()).toISOString().slice(0, 10);

        const steps = [
            pending > 0 ? `Export ${pending} photo${pending !== 1 ? 's' : ''}` : 'Rebuild the exported photos',
            'Build the pages',
            willUpload ? `Upload to ${escapeHtml(this._uploadHost() || 'the configured host')}` : null,
            this._row.url ? 'Check the link' : null,
        ].filter(Boolean);

        body.innerHTML = `
            <p class="form-hint">${pending > 0
                ? `${pending} photo${pending !== 1 ? 's are' : ' is'} waiting to go online.`
                : 'Nothing is waiting. Publishing rebuilds this gallery as it is.'}</p>
            <ol class="publish-plan">${steps.map(s => `<li>${s}</li>`).join('')}</ol>
            ${willUpload ? '' : '<p class="form-hint">This destination has no upload configured, so the files stay in the local output folder.</p>'}
            <label class="form-label" for="pub-date">${this._row.postID ? 'Date shown on the gallery' : 'Published date'}</label>
            <input class="form-input" id="pub-date" type="date" value="${dateValue}">
            <span class="build-date-note">${this._row.postID
                ? 'Stays as it is unless you change it here. Stored in XMP sidecars on the newly added photos.'
                : 'Sets album order in the built site and is stored in XMP sidecars.'}</span>
            `;

        this._dialog.setActions([
            { label: 'Cancel', id: 'pub-cancel', onClick: () => this.close() },
            {
                label: pending > 0 ? `Publish ${pending} photo${pending !== 1 ? 's' : ''}` : 'Publish',
                kind: 'primary',
                id: 'pub-run',
                onClick: () => this._run(),
            },
        ]);
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

        this._steps = [
            { id: 'export', label: 'Exporting photos' },
            { id: 'build', label: 'Building the pages' },
            ...(willUpload ? [{ id: 'upload', label: 'Uploading' }] : []),
            ...(this._row.url ? [{ id: 'check', label: 'Checking the link' }] : []),
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

        this._setStep('export', 'doing');
        const draftID = this._row.draftID || '-';
        let result;
        try {
            result = await ChannelAPI.generateStream(
                this._row.channelSlug, draftID, this._row.postID, { publishedAt },
                (evt) => {
                    if (evt.step === 'photo') {
                        this._setStep('export', 'doing', `${evt.done} of ${evt.total}`);
                    } else {
                        this._setStep('export', 'done');
                        this._setStep('build', 'doing', evt.file || '');
                    }
                },
            );
        } catch (err) {
            this._setStep('export', 'failed');
            this._fail('The export did not finish.', err.message, [{ label: 'Try again', run: () => this._run() }]);
            return;
        }
        this._result = result;
        this._setStep('export', 'done');
        this._setStep('build', 'done');

        const failed = (result.results || []).filter(r => r.Error || r.error);
        if (failed.length) {
            this._setStep('build', 'done', `${failed.length} photo${failed.length !== 1 ? 's' : ''} could not be exported`);
        }

        if (willUpload) {
            const ok = await this._upload();
            if (!ok) return;
        }

        if (this._row.url) await this._check();

        this._done = true;
        this._renderResult(failed);
    }

    async _upload() {
        this._setStep('upload', 'doing');
        try {
            const res = await ChannelAPI.deploy(this._row.channelSlug);
            if (!res.ok) throw new Error(res.error || 'the upload reported no reason');
            this._setStep('upload', 'done');
            return true;
        } catch (err) {
            this._setStep('upload', 'failed');
            // The build survives a failed upload, which is exactly the state
            // "Built, not uploaded" — so the retry only uploads.
            this._fail(
                'The pages are built, but the upload did not go through.',
                err.message,
                [{ label: 'Retry upload', run: async () => { this._el.querySelector('#pub-error').hidden = true; if (await this._upload()) { if (this._row.url) await this._check(); this._done = true; this._renderResult([]); } } }],
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
            this._setStep('check', reachable ? 'done' : 'failed', reachable ? 'answered' : 'no answer');
        } catch {
            this._reachable = false;
            this._setStep('check', 'failed', 'could not be checked');
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
            { label: 'Close', onClick: () => this.close() },
            ...actions.map(action => ({ label: action.label, kind: 'primary', onClick: () => action.run() })),
        ]);
    }

    _renderResult(failed) {
        const body = this._el.querySelector('#pub-body');
        const localPath = this._result.sitePath || this._result.galleryPath;
        const showsHTML = !!(this._result.sitePath || this._result.galleryPath);
        const willUpload = this._row.channelHandler === 'rsync';

        const summary = !willUpload
            ? 'Exported to the local output folder. Putting the files anywhere is up to you.'
            : this._row.url
                ? (this._reachable
                    ? 'Online. The link answered just now.'
                    : 'Uploaded, but the link did not answer. It can take a moment for the host to serve the new files.')
                : 'Uploaded.';

        this._el.querySelector('#pub-error')?.setAttribute('hidden', '');
        body.insertAdjacentHTML('beforeend', `
            <p class="publish-summary">${escapeHtml(summary)}</p>
            ${failed.length ? `<p class="form-hint">${failed.length} photo${failed.length !== 1 ? 's' : ''} could not be exported and stayed in the gallery's collected photos.</p>` : ''}
            <div class="publish-step-review">
                ${this._row.url ? `<div class="build-destination">${escapeHtml(this._row.url)}</div>` : `<div class="build-destination">${escapeHtml(localPath)}</div>`}
                <div class="publish-step-review-actions">
                    ${this._row.url ? `<a class="btn btn-sm" href="${escapeHtml(this._row.url)}" target="_blank" rel="noopener">Open in browser</a>` : ''}
                    <button class="btn btn-sm" id="pub-copy-path">Copy path</button>
                    <button class="btn btn-sm" id="pub-open-folder">Open in Finder/Explorer</button>
                    ${showsHTML ? '<button class="btn btn-sm" id="pub-preview">Preview locally</button>' : ''}
                </div>
            </div>`);

        this._dialog.setActions([{ label: 'Done', kind: 'primary', id: 'pub-done', onClick: () => this.close() }]);
        body.querySelector('#pub-copy-path').addEventListener('click', () => navigator.clipboard.writeText(localPath));
        // ChannelAPI.reveal(slug) reveals the channel's own output folder in
        // Finder/Explorer — it takes no path argument (see channels.js).
        body.querySelector('#pub-open-folder').addEventListener('click', () => ChannelAPI.reveal(this._row.channelSlug));
        body.querySelector('#pub-preview')?.addEventListener('click', () => {
            window.open('file://' + localPath + '/index.html', '_blank');
        });
    }
}
