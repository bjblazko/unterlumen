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
    // Everyone named in any library, put together by name.
    async everyone() {
        return peopleJSON(await fetch('/api/people'));
    },
    // The person of that name in every library, and who elsewhere may be them.
    async someone(name) {
        return peopleJSON(await fetch(`/api/people/someone?name=${encodeURIComponent(name)}`));
    },
    // The birthdays by name; the request fails where none are kept.
    async birthdays() {
        return peopleJSON(await fetch('/api/people/birthdays'));
    },
    async setBirthday(name, birthday) {
        const r = await fetch('/api/people/birthdays', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, birthday }),
        });
        if (!r.ok) throw new Error((await r.text()).trim());
    },
    // What waits for a name in each library: groups, photos to look at.
    async toName() {
        return peopleJSON(await fetch('/api/people/to-name'));
    },
    // Who is most often in the same photos as the person of that name.
    async oftenWith(name) {
        return peopleJSON(await fetch(`/api/people/with?name=${encodeURIComponent(name)}`));
    },
    // The names given in any library, for name fields to offer.
    async allNames() {
        return peopleJSON(await fetch('/api/library/person-names'));
    },
    async overview(lib) {
        return peopleJSON(await fetch(`/api/library/${lib}/people`));
    },
    async groupFaces(lib, grp) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/groups/${grp}`));
    },
    // Who a group looks like: { suggestion } with a person, or null.
    async groupSuggestion(lib, grp) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/groups/${grp}/suggestion`));
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
    // The faces hidden in lib, and taking them out of hiding.
    async hidden(lib) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/hidden`));
    },
    // The faces hidden in every library: [{ ...face, library }].
    async hiddenEverywhere() {
        return peopleJSON(await fetch('/api/people/hidden'));
    },
    async showAgain(lib, faces) {
        return peoplePost(`/api/library/${lib}/faces/show`, { faces });
    },
    // A No: the faces are not that person (their ID in lib), and are not
    // suggested as them again.
    async not(lib, faces, person) {
        const r = await fetch(`/api/library/${lib}/faces/not`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ faces, person }),
        });
        if (!r.ok) throw new Error((await r.text()).trim());
    },
    // The unnamed faces that look most like the person of that name, in
    // every library where they are named: [{ ...face, library, person }].
    async maybe(name) {
        return peopleJSON(await fetch(`/api/people/maybe?name=${encodeURIComponent(name)}`));
    },
    // Marks a face that was not found: box { x, y, w, h } in fractions of
    // the upright photo, and name.
    async markFace(lib, photo, box) {
        return peoplePost(`/api/library/${lib}/photo/${photo}/faces`, box);
    },
    // The faces of one photo, with the library's names to choose from.
    async photoFaces(lib, photo) {
        return peopleJSON(await fetch(`/api/library/${lib}/photo/${photo}/faces`));
    },
    // The face shown for the person, on this installation only.
    async chooseCover(lib, person, face) {
        const r = await fetch(`/api/library/${lib}/people/${person}/cover`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ face }),
        });
        if (!r.ok) throw new Error((await r.text()).trim());
    },
    async forget(lib, person) {
        return peopleJSON(await fetch(`/api/library/${lib}/people/${person}`, { method: 'DELETE' }));
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
