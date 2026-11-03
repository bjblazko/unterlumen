// MemoriesAPI — the Memories place's data (ADR-0064): the overview of a day
// and one memory of it. day is the browser's own date, YYYY-MM-DD; scope is
// ScopeState.query({ time: false }).

const MemoriesAPI = {
    async overview(day, scope) {
        return memoriesJSON(await fetch(`/api/memories?day=${day}${scope ? '&' + scope : ''}`));
    },
    async detail(day, kind, key, scope) {
        // memory=, not kind=: kind= is the shared filter's photos or videos.
        const q = `day=${day}&memory=${encodeURIComponent(kind)}&key=${encodeURIComponent(key)}`;
        return memoriesJSON(await fetch(`/api/memories/detail?${q}${scope ? '&' + scope : ''}`));
    },
};

async function memoriesJSON(r) {
    if (!r.ok) throw Object.assign(new Error((await r.text()).trim() || r.statusText), { status: r.status });
    return r.json();
}
