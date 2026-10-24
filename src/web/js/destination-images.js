// destination-images.js — the site's own images of a destination: its logo
// and the author's portrait, each uploaded, shown as set or not, and removed.
// A site is built again after each change, so its pages show it.

const SITE_IMAGES = {
    logo: {
        id: 'logo',
        status: slug => ChannelAPI.logoStatus(slug),
        upload: (slug, file) => ChannelAPI.uploadLogo(slug, file),
        remove: slug => ChannelAPI.deleteLogo(slug),
        set: 'Logo uploaded.', unset: 'No logo set.', unknown: 'Could not check logo.',
        notRebuilt: 'Logo saved, but the site did not rebuild. Open Galleries and publish one of its galleries again to rebuild it.',
        first: 'Create the destination first, then upload a logo.',
    },
    avatar: {
        id: 'avatar',
        status: slug => ChannelAPI.avatarStatus(slug),
        upload: (slug, file) => ChannelAPI.uploadAvatar(slug, file),
        remove: slug => ChannelAPI.deleteAvatar(slug),
        set: 'Portrait uploaded.', unset: 'No portrait set.', unknown: 'Could not check avatar.',
        notRebuilt: 'Portrait saved, but the site did not rebuild. Open Galleries and publish one of its galleries again to rebuild it.',
        first: 'Create the destination first, then upload a photo.',
    },
};

class SiteImageField {
    // kind is SITE_IMAGES.logo or .avatar; the form holds its #chf-<id>-…
    // status, file input and buttons.
    constructor(form, ch, kind) {
        this.ch = ch;
        this.kind = kind;
        const el = name => form.querySelector(`#chf-${kind.id}-${name}`);
        this.statusEl = el('status');
        this.uploadBtn = el('upload');
        this.removeBtn = el('remove');
        this.fileInput = el('file');
    }

    // A destination not created yet has no folder to keep an image in.
    disableUntilCreated() {
        this.statusEl.textContent = t(this.kind.first);
        this.uploadBtn.disabled = true;
    }

    async wire() {
        await this._refresh();
        this.uploadBtn.addEventListener('click', () => this.fileInput.click());
        this.fileInput.addEventListener('change', () => this._upload());
        this.removeBtn.addEventListener('click', () => this._remove());
    }

    async _refresh() {
        try {
            const { exists } = await this.kind.status(this.ch.slug);
            this.statusEl.textContent = t(exists ? this.kind.set : this.kind.unset);
            this.removeBtn.style.display = exists ? '' : 'none';
        } catch {
            this.statusEl.textContent = t(this.kind.unknown);
        }
    }

    async _rebuildIfSite() {
        if (!this.ch.siteExport) return;
        this.statusEl.textContent = t('Rebuilding the site…');
        try { await ChannelAPI.rebuildSite(this.ch.slug); } catch {
            this.statusEl.textContent = t(this.kind.notRebuilt);
        }
    }

    async _upload() {
        const file = this.fileInput.files[0];
        if (!file) return;
        this.statusEl.textContent = t('Uploading…');
        this.uploadBtn.disabled = true;
        try {
            await this.kind.upload(this.ch.slug, file);
            await this._rebuildIfSite();
            await this._refresh();
        } catch (err) {
            this.statusEl.textContent = t('Upload failed: ') + err.message;
        } finally {
            this.uploadBtn.disabled = false;
            this.fileInput.value = '';
        }
    }

    async _remove() {
        this.removeBtn.disabled = true;
        try {
            await this.kind.remove(this.ch.slug);
            await this._rebuildIfSite();
            await this._refresh();
        } catch (err) {
            this.statusEl.textContent = t('Remove failed: ') + err.message;
        } finally {
            this.removeBtn.disabled = false;
        }
    }
}
