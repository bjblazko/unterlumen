// originals.js — "Download" on a selection: the photos as the files they
// are. One photo is one file; several come as a ZIP the server packs
// without converting anything, shown in the status line while it works.

const Originals = {
    // files: selected photo paths of `pane`; dirs: selected folders, packed
    // with everything in them. sourcePath: the library's source folder when
    // the paths are relative to it.
    async download(files, pane, { dirs = [], sourcePath = null } = {}) {
        if (files.length === 1 && dirs.length === 0) {
            saveURL(originalURL(pane, files[0]));
            return;
        }
        const btn = document.querySelector('.selection-bar [data-action="download"]');
        const restore = btn ? Activity.button(btn, 'Packing…') : () => {};
        try {
            const token = await packOriginals(zipRequest(files, dirs, pane, sourcePath));
            saveURL(`/api/export/zip-download?token=${encodeURIComponent(token)}&name=${encodeURIComponent(zipName(pane, files, dirs))}`);
        } catch (err) {
            App.showToast(`The photos could not be packed: ${err.message}`);
        } finally {
            restore();
        }
    },
};

// Library photos go by library and ID, which the server looks up in the
// index: filter results carry absolute paths, and a server refuses those.
function zipRequest(files, dirs, pane, sourcePath) {
    const photos = [];
    const paths = [];
    for (const f of files) {
        const meta = pane.getLibraryMeta?.(f);
        if (meta) photos.push({ library: meta.libID, id: meta.photoID });
        else paths.push(f);
    }
    return { format: 'original', files: paths, dirs, photos, sourcePath: sourcePath || undefined };
}

// The pane knows where its photos come from (a folder or a library); the
// image routes hand out the file itself with ?download=1.
function originalURL(pane, path) {
    const url = pane.viewerImageURL ? pane.viewerImageURL(path) : API.imageURL(path);
    return url + (url.includes('?') ? '&' : '?') + 'download=1';
}

async function packOriginals(request) {
    const resp = await fetch('/api/export/zip-stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
    });
    if (!resp.ok) throw new Error(await resp.text());
    let token = null;
    let lastError = null;
    await readEventStream(resp, (evt) => {
        if (evt.complete) token = evt.token;
        else if (evt.error) lastError = evt.file ? `${evt.file}: ${evt.error}` : evt.error;
    });
    if (!token) throw new Error(lastError || 'the server stopped before the ZIP was ready');
    return token;
}

// 2024.zip for the one folder 2024; Travel.zip for photos from Travel;
// Photos.zip for filter results.
function zipName(pane, files, dirs) {
    const last = (p) => (p || '').split('/').filter(Boolean).pop();
    const name = (dirs.length === 1 && files.length === 0) ? last(dirs[0]) : last(pane.path);
    return `${name || 'Photos'}.zip`;
}

// A link with `download` lets the browser save the file itself, streamed to
// disk rather than held in the page.
function saveURL(url) {
    const a = document.createElement('a');
    a.href = url;
    a.download = '';
    document.body.appendChild(a);
    a.click();
    a.remove();
}
