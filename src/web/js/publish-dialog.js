// publish-dialog.js — PublishDialog walks one gallery/album/draft through the
// collect→publish lifecycle's second half: review pending photos, Generate
// (export + build HTML), review the generated artifact, and Deploy if the
// channel has a handler. Reused for both a channel's first publish and any
// later republish.
//
// Uses the .modal-backdrop keyboard-guard pattern (see CLAUDE.md's Dialogs &
// keyboard guard section): the header — including #pub-close — is rendered
// once in open() and never replaced; only #pub-body's innerHTML changes
// between steps. The global keyboard guard (app-keyboard.js) handles Escape
// centrally by clicking #pub-close, so this dialog must not add its own
// keydown listener.
class PublishDialog {
    constructor() {
        this._el = null;
    }

    // row: one item from GET /api/channels/galleries (Task 4's extended shape) —
    // { channelSlug, channelName, channelHandler, status, postID, draftID,
    //   title, pendingCount, url, urlGuessed }
    async open(row) {
        this._row = row;
        this._el = document.createElement('div');
        this._el.className = 'modal-backdrop';
        this._el.innerHTML = `
            <div class="modal publish-dialog">
                <div class="modal-header">
                    <span class="modal-title">Publish — ${escapeHtml(row.title || row.channelName)}</span>
                    <button class="modal-close" id="pub-close">&times;</button>
                </div>
                <div class="modal-body" id="pub-body"></div>
            </div>`;
        document.body.appendChild(this._el);
        this._el.querySelector('#pub-close').addEventListener('click', () => this.close());
        this._el.addEventListener('click', e => { if (e.target === this._el) this.close(); });
        await this._renderReviewStep();
    }

    close() {
        this._el?.remove();
        this._el = null;
    }

    async _renderReviewStep() {
        const body = this._el.querySelector('#pub-body');
        body.innerHTML = '<div class="channel-loading">Loading pending photos…</div>';
        let drafts = [];
        try {
            drafts = await ChannelAPI.listDrafts(this._row.channelSlug);
        } catch (err) {
            body.innerHTML = `<div class="channel-error">Failed to load: ${escapeHtml(err.message)}</div>`;
            return;
        }
        const draft = drafts.find(d => d.id === this._row.draftID) || null;
        this._draft = draft;

        const photoRows = (draft?.photos || []).map(p => `
            <div class="publish-step-photo" data-lib="${escapeHtml(p.libraryID)}" data-photo="${escapeHtml(p.photoID)}">
                <img class="publish-step-thumb" src="/api/library/${encodeURIComponent(p.libraryID)}/thumb/${encodeURIComponent(p.photoID)}" alt="">
                <button class="publish-step-remove" title="Remove from this publish">×</button>
            </div>`).join('');

        // The date belongs to the gallery, not to this publish run: default to
        // the date it already carries, so publishing changes does not silently
        // re-date a gallery (ADR-0029). Only a gallery that has never been
        // published starts at today.
        const existing = this._row.postID && this._row.publishedAt ? new Date(this._row.publishedAt) : null;
        const validExisting = existing && !isNaN(existing) && existing.getFullYear() > 1970 ? existing : null;
        const dateValue = (validExisting || new Date()).toISOString().slice(0, 10);
        const dateLabel = this._row.postID ? 'Date shown on the gallery' : 'Published date';
        const dateNote = this._row.postID
            ? 'Stays as it is unless you change it here. Stored in XMP sidecars on the newly added photos.'
            : 'Sets album order in the built site and is stored in XMP sidecars.';

        body.innerHTML = `
            <p class="form-hint">${draft ? draft.photos.length : 0} photo${(draft?.photos.length ?? 0) !== 1 ? 's' : ''} pending for "${escapeHtml(this._row.title || this._row.channelName)}".</p>
            <div class="publish-step-photos">${photoRows || '<span class="channel-empty">Nothing pending — Generate will just refresh the current gallery.</span>'}</div>
            <label class="form-label">${dateLabel}</label>
            <input class="form-input" id="pub-date" type="date" value="${dateValue}">
            <span class="build-date-note">${dateNote}</span>
            <div class="modal-footer">
                <button class="btn" id="pub-cancel">Cancel</button>
                <button class="btn btn-accent" id="pub-generate">Generate</button>
            </div>`;

        body.querySelectorAll('.publish-step-remove').forEach(btn => {
            btn.addEventListener('click', async () => {
                const card = btn.closest('.publish-step-photo');
                try {
                    await ChannelAPI.removeDraftPhoto(this._row.channelSlug, draft.id, card.dataset.lib, card.dataset.photo);
                    await this._renderReviewStep();
                } catch (err) {
                    alert('Remove failed: ' + err.message);
                }
            });
        });
        body.querySelector('#pub-cancel').addEventListener('click', () => this.close());
        body.querySelector('#pub-generate').addEventListener('click', () => this._runGenerate());
    }

    // _runGenerate has two paths: a real pending draft (this._row.draftID set —
    // the normal collect-then-publish case), or a Live gallery/album with
    // nothing newly collected (this._row.draftID empty) — re-publishing an
    // existing gallery with no new photos, which subsumes the old "Rebuild"
    // action. The backend's generateDraft handler accepts the literal string
    // "-" as a draftID sentinel for this second case, reading the target
    // postID from a query parameter instead of a stored draft (see Task 3).
    async _runGenerate() {
        const body = this._el.querySelector('#pub-body');
        const dateVal = body.querySelector('#pub-date')?.value;
        const publishedAt = dateVal ? new Date(dateVal + 'T12:00:00Z').toISOString() : undefined;
        const draftID = this._row.draftID || '-';
        body.innerHTML = '<div class="channel-loading" id="pub-progress">Generating…</div>';
        const progressEl = body.querySelector('#pub-progress');
        try {
            const result = await ChannelAPI.generateStream(
                this._row.channelSlug, draftID, this._row.postID,
                { publishedAt },
                (evt) => {
                    if (evt.step === 'photo') progressEl.textContent = `Exporting photo ${evt.done} of ${evt.total}…`;
                    else if (evt.file) progressEl.textContent = evt.file;
                }
            );
            this._result = result;
            await this._renderArtifactStep();
        } catch (err) {
            body.innerHTML = `<div class="channel-error">Generate failed: ${escapeHtml(err.message)}</div>
                <div class="modal-footer"><button class="btn" id="pub-cancel">Close</button></div>`;
            body.querySelector('#pub-cancel').addEventListener('click', () => this.close());
        }
    }

    async _renderArtifactStep() {
        const body = this._el.querySelector('#pub-body');
        const isSite = !!this._result.sitePath;
        const localPath = this._result.sitePath || this._result.galleryPath;
        const showsHTML = isSite || !!this._result.galleryPath; // gallery/site export channels only
        // channelHandler mirrors Channel.Handler ("rsync", or empty) — see
        // PublishedGallery.ChannelHandler in galleries_overview.go. The row
        // never carries a plain "handler" field.
        const canDeploy = this._row.channelHandler === 'rsync';

        body.innerHTML = `
            <p class="form-hint">Generated. Nothing has been published live yet.</p>
            <div class="publish-step-review">
                <div class="build-destination">${escapeHtml(localPath)}</div>
                <div class="publish-step-review-actions">
                    <button class="btn btn-sm" id="pub-copy-path">Copy path</button>
                    <button class="btn btn-sm" id="pub-open-folder">Open in Finder/Explorer</button>
                    ${showsHTML ? '<button class="btn btn-sm" id="pub-open-browser">Open in browser</button>' : ''}
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn" id="pub-close-2">${canDeploy ? 'Not now' : 'Done'}</button>
                ${canDeploy ? '<button class="btn btn-accent" id="pub-deploy">Deploy</button>' : ''}
            </div>`;

        body.querySelector('#pub-copy-path').addEventListener('click', () => navigator.clipboard.writeText(localPath));
        // ChannelAPI.reveal(slug) reveals the channel's own output folder in
        // Finder/Explorer — it takes no path argument (see channels.js).
        body.querySelector('#pub-open-folder').addEventListener('click', () => ChannelAPI.reveal(this._row.channelSlug));
        body.querySelector('#pub-open-browser')?.addEventListener('click', () => {
            window.open('file://' + localPath + '/index.html', '_blank');
        });
        body.querySelector('#pub-close-2').addEventListener('click', () => this.close());
        body.querySelector('#pub-deploy')?.addEventListener('click', () => this._runDeploy());
    }

    async _runDeploy() {
        const body = this._el.querySelector('#pub-body');
        const deployBtn = body.querySelector('#pub-deploy');
        deployBtn.disabled = true;
        deployBtn.textContent = 'Deploying…';
        try {
            const res = await ChannelAPI.deploy(this._row.channelSlug);
            if (res.ok) {
                App.showToast('Deployed.');
                this.close();
            } else {
                deployBtn.disabled = false;
                deployBtn.textContent = 'Deploy';
                alert('Deploy failed: ' + (res.error || 'unknown error'));
            }
        } catch (err) {
            deployBtn.disabled = false;
            deployBtn.textContent = 'Deploy';
            alert('Deploy failed: ' + err.message);
        }
    }
}
