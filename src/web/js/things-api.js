// ThingsAPI — what finds things in photos, and the things in one photo
// (ADR-0054). The things in the libraries are LibraryAPI.thingTotals.

const ThingsAPI = {
    async status() {
        return thingsJSON(await fetch('/api/things'));
    },
    // Resolves when ONNX Runtime and the models are downloaded and checked.
    async install() {
        const r = await fetch('/api/things/install', { method: 'POST' });
        if (!r.ok) throw new Error((await r.text()).trim());
    },
    async photoThings(lib, photo) {
        return thingsJSON(await fetch(`/api/library/${lib}/photo/${photo}/things`));
    },
    // Takes a thing away from the photo for good: its sidecar remembers.
    async remove(lib, photo, thing) {
        const r = await fetch(`/api/library/${lib}/photo/${photo}/things/${encodeURIComponent(thing)}`, { method: 'DELETE' });
        if (!r.ok) throw new Error((await r.text()).trim());
    },
};

async function thingsJSON(r) {
    if (!r.ok) throw new Error((await r.text()).trim());
    return r.json();
}
