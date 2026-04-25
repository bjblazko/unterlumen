// API client functions

const API = {
    async browse(path = '', sort = 'name', order = 'asc') {
        const params = new URLSearchParams({ path, sort, order });
        const resp = await fetch(`/api/browse?${params}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    thumbnailURL(path, size) {
        let url = `/api/thumbnail?path=${encodeURIComponent(path)}`;
        if (size) url += `&size=${size}`;
        return url;
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

    async delete(files) {
        const resp = await fetch('/api/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
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
};

function escapeHtml(s) {
    return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}
