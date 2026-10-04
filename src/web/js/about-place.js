// about-place.js — About Unterlumen: what it is, who makes it, and the way
// to its topics (How it works, Your data, No warranty, Licenses and thanks).
// A place, not a dialog (ADR-0033): you read it. The topics are places of
// their own, listed under About in the sidebar while one of them is open.

const ABOUT_TOPICS = [
    ['guide', 'guide', 'How Unterlumen works', 'Folders, libraries, galleries and destinations, and how they fit together.'],
    ['privacy', 'privacy', 'Your data', 'What stays on your computer, and the few things that leave it, and when.'],
    ['warranty', 'warranty', 'No warranty', 'Unterlumen changes your files when you tell it to. Keep a backup.'],
    ['licenses', 'licenses', 'Licenses and thanks', 'The projects Unterlumen is built on, and where to support them.'],
];

// The places that belong to About: the sidebar shows its topics while one is open.
const ABOUT_PLACES = new Set(['about', ...ABOUT_TOPICS.map(([mode]) => mode)]);

class AboutPane {
    constructor(container) {
        this.container = container;
    }

    render() {
        if (this.container.firstChild) return; // nothing on it changes
        const version = App.config?.version;
        this.container.innerHTML = `
            <div class="guide-pane">
                <div class="gal-head">
                    <h1 class="gal-title">About Unterlumen</h1>
                </div>
                <div class="reading-body">
                    <div class="about-logo-row">
                        <svg class="about-logo" viewBox="0 0 36 28" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                            <rect x="0" y="0" width="25" height="4" fill="currentColor"/>
                            <rect x="26" y="0" width="10" height="4" fill="currentColor" fill-opacity="0.3"/>
                            <polygon points="0,8 36,8 18,28" fill="var(--accent)"/>
                        </svg>
                        <div>
                            <div class="about-app-name">Unterlumen</div>
                            <div class="about-app-tagline">Photo browser, culler, and digital asset manager</div>
                            ${version ? `<div class="about-app-version mono">${escapeHtml(version)}</div>` : ''}
                        </div>
                    </div>
                    <ul class="about-topics">
                        ${ABOUT_TOPICS.map(([mode, hash, label, text]) => `
                            <li>${placeLink(mode, hash, label)}<span>${text}</span></li>`).join('')}
                    </ul>
                    <section class="guide-section">
                        <h2>Made by</h2>
                        <p>Timo Böwing, in his spare time — <a href="https://huepattl.de" target="_blank" rel="noopener noreferrer">huepattl.de</a>, <a href="mailto:timo.boewing@posteo.de">timo.boewing@posteo.de</a>. The source is on <a href="https://github.com/bjblazko/unterlumen" target="_blank" rel="noopener noreferrer">GitHub</a>, under the Apache License 2.0.</p>
                    </section>
                </div>
            </div>`;
    }
}
