// things-words.js — the things' words in the interface's language
// (ADR-0056). A thing is kept by its ID, its English name, everywhere — the
// index, the sidecars' record, addresses; only what is shown and typed is in
// the interface's language: ThingWords.name('dog') is "Hund" in German.

const ThingWords = {
    _byID: new Map(),
    _loading: null,

    // load reads the words once; until they are there, names are the IDs.
    load() {
        this._loading ??= fetch(`/api/things/words?lang=${I18n.lang}`)
            .then(r => (r.ok ? r.json() : []))
            .then(words => { for (const w of words) this._byID.set(w.thing, w); })
            .catch(() => {});
        return this._loading;
    },

    name(id) {
        return this._byID.get(id)?.name || id;
    },

    // kind is the thing's category in the interface's language.
    kind(id) {
        return this._byID.get(id)?.kind || '';
    },

    // matches lists the things whose name, other words or ID hold text,
    // without regard to case: "Hu" finds dog in German, "dog" too.
    matches(text) {
        const low = text.toLowerCase();
        const out = [];
        for (const w of this._byID.values()) {
            const words = [w.name, w.thing, ...(w.also || [])];
            if (words.some(x => x.toLowerCase().includes(low))) out.push(w.thing);
        }
        return out;
    },
};
