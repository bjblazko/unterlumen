// API client functions

const API = {
    async browse(path = '', sort = 'name', order = 'asc') {
        const params = new URLSearchParams({ path, sort, order });
        const resp = await fetch(`/api/browse?${params}`);
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },

    thumbnailURL(path) {
        return `/api/thumbnail?path=${encodeURIComponent(path)}`;
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

    async delete(files) {
        const resp = await fetch('/api/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ files }),
        });
        if (!resp.ok) throw new Error(await resp.text());
        return resp.json();
    },
};
