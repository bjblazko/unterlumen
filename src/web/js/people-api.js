// PeopleAPI — face recognition and the people named in a library (ADR-0051).

const PeopleAPI = {
    async status() {
        return peopleJSON(await fetch('/api/faces'));
    },
    // Resolves when ONNX Runtime and the models are downloaded and checked.
    async install() {
        const r = await fetch('/api/faces/install', { method: 'POST' });
        if (!r.ok) throw new Error(await r.text());
    },
    async overview(lib) {
        return peopleJSON(await fetch(`/api/library/${lib}/people`));
    },
    async groupFaces(lib, grp) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/groups/${grp}`));
    },
    async personFaces(lib, person) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/${person}`));
    },
    faceURL(lib, face) {
        return `/api/library/${lib}/face/${face}`;
    },
    // Each change answers { skipped }: photos whose names stay in this
    // library only, because another program keeps their face regions.
    async name(lib, faces, name) {
        return peoplePost(`/api/library/${lib}/faces/name`, { faces, name });
    },
    async unname(lib, faces) {
        return peoplePost(`/api/library/${lib}/faces/unname`, { faces });
    },
    async hide(lib, faces) {
        return peoplePost(`/api/library/${lib}/faces/hide`, { faces });
    },
    async rename(lib, person, name) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/${person}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name }),
        }));
    },
};

async function peopleJSON(r) {
    if (!r.ok) throw new Error((await r.text()).trim());
    return r.json();
}

async function peoplePost(url, body) {
    return peopleJSON(await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    }));
}
