// API client functions

const API = {
    async browse(path = '', sort = 'name', order = 'asc', signal) {
        const params = new URLSearchParams({ path, sort, order });
        const resp = await fetch(`/api/browse?${params}`, signal ? { signal } : undefined);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    thumbnailURL(path, size) {
        const params = new URLSearchParams({
            path,
            quality: localStorage.getItem('thumbnail-quality') || 'standard',
        });
        if (size) params.set('size', size);
        return `/api/thumbnail?${params.toString()}`;
    },

    imageURL(path) {
        return `/api/image?path=${encodeURIComponent(path)}`;
    },

    async copy(files, destination) {
        const resp = await fetch('/api/copy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files, destination }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async move(files, destination) {
        const resp = await fetch('/api/move', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files, destination }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async info(path) {
        const resp = await fetch(`/api/info?path=${encodeURIComponent(path)}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async detectLibrary(path) {
        const resp = await fetch(`/api/library/detect?path=${encodeURIComponent(path)}`);
        if (!resp.ok) return {};
        return resp.json();
    },

    async browseDates(path = '') {
        const params = new URLSearchParams({ path });
        const resp = await fetch(`/api/browse/dates?${params}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async browseMeta(path = '') {
        const params = new URLSearchParams({ path });
        const resp = await fetch(`/api/browse/meta?${params}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async browseRecursive(path = '') {
        const params = new URLSearchParams({ path });
        const resp = await fetch(`/api/browse/recursive?${params}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async browseDirs(path = '') {
        const params = new URLSearchParams({ path });
        const resp = await fetch(`/api/browse/dirs?${params}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async delete(files) {
        const resp = await fetch('/api/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    // The setup (#setup): what config.json says, whether a folder holds a
    // shared folder, and saving a new choice.
    async setup() {
        const resp = await fetch('/api/setup');
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async setupShared(photosPath) {
        const resp = await fetch(`/api/setup/shared?${new URLSearchParams({ path: photosPath })}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    // A server's sharing (server mode): where destinations are shared, and
    // starting to share them through the photo folder.
    async sharing() {
        const resp = await fetch('/api/setup/sharing');
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async share() {
        const resp = await fetch('/api/setup/share', { method: 'POST' });
        if (!resp.ok) throw new Error(await resp.text());
    },

    async saveSetup(choice) {
        const resp = await fetch('/api/setup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(choice),
        });
        if (!resp.ok) throw new Error(await resp.text());
    },

    // Opens the system's folder dialog on the computer Unterlumen runs on
    // and waits for it: the chosen folder's absolute path, or null when it
    // was cancelled.
    async folderDialog(prompt) {
        const resp = await fetch('/api/folder-dialog', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ prompt }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        const result = await resp.json();
        return result.cancelled ? null : result.path;
    },

    // Installs the missing helper programs and waits until that is done.
    async installTools() {
        const resp = await fetch('/api/tools/install', { method: 'POST' });
        if (!resp.ok) throw new Error(await resp.text());
    },

    async toolsCheck() {
        const resp = await fetch('/api/tools/check');
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async removeLocation(files) {
        const resp = await fetch('/api/remove-location', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async setLocation(files, latitude, longitude) {
        const resp = await fetch('/api/set-location', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files, latitude, longitude }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async mkdir(path) {
        const resp = await fetch('/api/mkdir', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async rename(path, name) {
        const resp = await fetch('/api/rename', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path, name }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async listRecursive(path) {
        const resp = await fetch('/api/list-recursive', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async batchRenamePreview(files, pattern) {
        const resp = await fetch('/api/batch-rename/preview', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files, pattern }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async batchRenameExecute(files, pattern) {
        const resp = await fetch('/api/batch-rename/execute', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files, pattern }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    config() {
        return fetch('/api/config').then(r => r.json());
    },

    cacheInfo() {
        return fetch('/api/cache/info').then(r => r.json());
    },

    async cacheClear() {
        const resp = await fetch('/api/cache/clear', { method: 'POST' });
        if (!resp.ok) throw new Error(await resp.text());
    },

    async cacheEvict(paths) {
        const resp = await fetch('/api/cache/evict', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ paths }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async exportEstimate(payload, signal) {
        const resp = await fetch('/api/export/estimate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
            signal,
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async exportZip(payload) {
        const resp = await fetch('/api/export/zip', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.blob();
    },

    async exportZipDownload(token) {
        const resp = await fetch(`/api/export/zip-download?token=${encodeURIComponent(token)}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.blob();
    },

    async exportSave(payload) {
        const resp = await fetch('/api/export/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    async crop(path, x, y, width, height) {
        const resp = await fetch('/api/crop', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path, x, y, width, height }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },
};

// readEventStream reads a server-sent event stream and calls onEvent with
// each JSON data event, in order, until the stream ends — or until onEvent
// returns true. An event that is not JSON, or whose handler throws, is
// skipped.
async function readEventStream(response, onEvent) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
        const { done, value } = await reader.read();
        if (done) return;
        buffer += decoder.decode(value, { stream: true });
        const blocks = buffer.split('\n\n');
        buffer = blocks.pop() ?? '';
        for (const block of blocks) {
            const line = block.split('\n').find(l => l.startsWith('data:'));
            if (!line) continue;
            try {
                if (onEvent(JSON.parse(line.slice(5).trim())) === true) return;
            } catch { /* skip malformed */ }
        }
    }
}

function escapeHtml(s) {
    return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

// Converts an absolute filesystem path to a path relative to the server's
// browse boundary (App.config.boundary), which is what every path-taking API
// endpoint validates against. Returns null if absPath isn't under boundary
// at all. Naively stripping the leading "/" only happens to work when
// boundary is "/" itself (no navigation restriction) — with any other
// boundary (e.g. "/photos") it produces a bogus, doubled path that the
// server correctly rejects as invalid.
// The end of a path says which folder it is; the start rarely does. A long
// path keeps its last few segments, so two "Auswahl" folders stay apart.
function shortenPath(path, keep = 3) {
    const parts = (path || '').split('/').filter(Boolean);
    return parts.length > keep ? `…/${parts.slice(-keep).join('/')}` : (path || '');
}

function absPathRelativeToBoundary(absPath, boundary) {
    const b = (boundary || '').replace(/\/$/, '');
    const p = (absPath || '').replace(/\/$/, '');
    if (p === b) return '';
    if (!b) return p.replace(/^\//, '');
    if (p.startsWith(b + '/')) return p.substring(b.length + 1);
    return null;
}
