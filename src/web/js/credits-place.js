// credits-place.js — "Licenses and thanks": what Unterlumen is built on,
// under which license, and where to support it. A place, not a dialog
// (ADR-0033): you read it. The list is web/licenses/credits.json, which a Go
// test keeps complete; the license texts travel inside the binary with it.

const CREDITS_SECTIONS = [
    ['inside', 'Built into Unterlumen', 'These are inside the program. Their license texts come with it.'],
    ['programs', 'Programs it calls', 'Unterlumen does not contain these; it calls them when they are installed, and Settings says which ones it finds. The container image includes them from Debian, whose sources are at <a href="https://sources.debian.org" target="_blank" rel="noopener noreferrer">sources.debian.org</a>.'],
    ['data', 'Maps and data', 'The map tiles are loaded from OpenFreeMap while a map is shown.'],
];

function creditsLink(href, label) {
    return `<a href="${escapeHtml(href)}" target="_blank" rel="noopener noreferrer">${label}</a>`;
}

function creditsItem(c) {
    const links = [creditsLink(c.home, 'Website')];
    if (c.support) links.push(creditsLink(c.support, 'Support the project'));
    if (c.text) links.push(creditsLink(`/licenses/${c.text}`, 'License text'));
    return `<li class="credits-item">
        <div class="credits-name">${escapeHtml(c.name)}</div>
        <div class="credits-use">${escapeHtml(c.use)}</div>
        <div class="credits-license mono">${escapeHtml(c.license)}</div>
        <div class="credits-links">${links.join('')}</div>
    </li>`;
}

function creditsSections(credits) {
    return CREDITS_SECTIONS.map(([key, title, note]) => `
        <section class="guide-section">
            <h2>${title}</h2>
            <p>${note}</p>
            <ul class="credits-list">${credits[key].map(creditsItem).join('')}</ul>
        </section>`).join('');
}

class CreditsPane {
    constructor(container) {
        this.container = container;
    }

    async render() {
        if (this.container.firstChild) return; // nothing on it changes
        this.container.innerHTML = `
            <div class="guide-pane">
                <div class="gal-head">
                    <h1 class="gal-title">Licenses and thanks</h1>
                </div>
                <div class="reading-body">
                    <p class="reading-intro">Unterlumen stands on the work of the projects below. Each keeps its own license; Unterlumen itself is under the ${creditsLink('https://github.com/bjblazko/unterlumen/blob/main/LICENSE', 'Apache License 2.0')}. Thank you to everyone who makes them. If Unterlumen is useful to you, consider supporting them too — where a project takes support, its entry links there.</p>
                    <div class="reading-text"></div>
                </div>
            </div>`;
        await this._load(this.container.querySelector('.reading-text'));
    }

    async _load(host) {
        const activity = Activity.in(host, 'Reading the list…');
        try {
            const resp = await fetch('/licenses/credits.json');
            if (!resp.ok) throw new Error(`the server answered ${resp.status}`);
            host.innerHTML = creditsSections(await resp.json());
        } catch (err) {
            activity.fail(`The list could not be read: ${err.message}. Reload the page to try again.`);
        }
    }
}
