// LibraryAPI — the /api/library/* calls: libraries, photos, filters, jobs.

const LibraryAPI = {
    async list() {
        const r = await fetch('/api/library/');
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async create(name, description, sourcePath) {
        const r = await fetch('/api/library/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description, sourcePath }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async get(id) {
        const r = await fetch(`/api/library/${id}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async delete(id) {
        const r = await fetch(`/api/library/${id}`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
    },
    async update(id, name, description) {
        const r = await fetch(`/api/library/${id}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description })
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async setOrder(ids) {
        const r = await fetch('/api/library-order', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ order: ids })
        });
        if (!r.ok) throw new Error(await r.text());
    },
    async getSettings() {
        const r = await fetch('/api/settings');
        if (!r.ok) return {};
        return r.json();
    },
    async patchSettings(patch) {
        const r = await fetch('/api/settings', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(patch)
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async photos(id, { q = '', offset = 0, limit = 100, ...filters } = {}) {
        const params = new URLSearchParams({ offset, limit });
        if (q) params.set('q', q);
        for (const [k, v] of Object.entries(filters)) params.set(k, v);
        const r = await fetch(`/api/library/${id}/photos?${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async exifRanges(id) {
        const r = await fetch(`/api/library/${id}/exif-ranges`);
        if (!r.ok) return {};
        return r.json();
    },
    async globalExifRanges(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/exif-ranges${params}`);
        if (!r.ok) return {};
        return r.json();
    },
    async exifValues(field, ids) {
        const params = new URLSearchParams({ field });
        if (ids) params.set('ids', ids);
        const r = await fetch(`/api/library/exif-values?${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async metaKeys(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/meta-keys${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async metaValues(key, ids) {
        const params = new URLSearchParams({ key });
        if (ids) params.set('ids', ids);
        const r = await fetch(`/api/library/meta-values?${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async albumTitles(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/album-titles${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async exifFields(ids) {
        const params = ids ? `?ids=${ids}` : '';
        const r = await fetch(`/api/library/exif-fields${params}`);
        if (!r.ok) return [];
        return r.json();
    },
    async search({ ids, limit = 100, offset = 0, ...rest } = {}) {
        const params = new URLSearchParams({ limit, offset });
        if (ids) params.set('ids', ids);
        for (const [k, v] of Object.entries(rest)) params.set(k, v);
        const r = await fetch(`/api/library/search?${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async statistics(ids) {
        const params = ids?.length ? `?ids=${ids.join(',')}` : '';
        const r = await fetch(`/api/library/statistics${params}`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async deletePhoto(libID, photoID) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}`, { method: 'DELETE' });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    // Every located photo of every library: { libraries: [{ id, points:
    // [[photoID, lat, lon, taken, filename], …] }] } (ADR-0039).
    async geo() {
        const r = await fetch('/api/library/geo');
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    thumbURL(libID, photoID) {
        return `/api/library/${libID}/thumb/${photoID}`;
    },
    photoURL(libID, photoID) {
        return `/api/library/${libID}/photo/${photoID}`;
    },
    async photoIDByPath(libID, relPath) {
        const r = await fetch(`/api/library/${libID}/photo-id-by-path?path=${encodeURIComponent(relPath)}`);
        if (!r.ok) return null;
        const { photoID } = await r.json();
        return photoID;
    },
    async getMeta(libID, photoID) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}/meta`);
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async buildDownload(libID, { photoIDs, channel, recordXMP }) {
        const r = await fetch(`/api/library/${libID}/build-download`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, channel, recordXMP }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.blob();
    },
    async collect(libID, slug, { photoIDs, draftID, postID, title, unlisted, account }) {
        const r = await fetch(`/api/library/${libID}/channels/${slug}/drafts`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ photoIDs, draftID, postID, title, unlisted, account }),
        });
        if (!r.ok) throw new Error(await r.text());
        return r.json();
    },
    async upsertMeta(libID, photoID, key, value) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}/meta`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key, value }),
        });
        if (!r.ok) throw new Error(await r.text());
    },
    async deleteMeta(libID, photoID, key) {
        const r = await fetch(`/api/library/${libID}/photo/${photoID}/meta?key=${encodeURIComponent(key)}`, {
            method: 'DELETE',
        });
        if (!r.ok) throw new Error(await r.text());
    },
    reindex(id, onProgress, subfolder) { return LibraryAPI._libraryJob(id, 'reindex', onProgress, subfolder); },
    cleanup(id, onProgress, subfolder) { return LibraryAPI._libraryJob(id, 'cleanup', onProgress, subfolder); },
    regenMissingPreviews(id, onProgress, subfolder) { return LibraryAPI._libraryJob(id, 'regen-previews-missing', onProgress, subfolder); },
    rebuildAllPreviews(id, onProgress, subfolder) { return LibraryAPI._libraryJob(id, 'regen-previews-all', onProgress, subfolder); },
    analyseAgain(id, onProgress, subfolder) { return LibraryAPI._libraryJob(id, 'analyse', onProgress, subfolder); },
    scanNew(id, onProgress, subfolder) { return LibraryAPI._libraryJob(id, 'scan-new', onProgress, subfolder); },
    // _libraryJob starts a job on a library, or on one of its folders, and
    // reports each progress event. It resolves when the job says it finished
    // or the stream ends.
    _libraryJob(id, action, onProgress, subfolder) {
        return new Promise((resolve, reject) => {
            const url = subfolder
                ? `/api/library/${id}/${action}?subfolder=${encodeURIComponent(subfolder)}`
                : `/api/library/${id}/${action}`;
            fetch(url, { method: 'POST' })
                .then(async r => {
                    if (!r.ok) { reject(new Error(await r.text())); return; }
                    await readEventStream(r, (p) => {
                        onProgress(p);
                        return !!p.finished;
                    });
                    resolve();
                })
                .catch(err => { if (err.name !== 'AbortError') reject(err); });
        });
    },
};
