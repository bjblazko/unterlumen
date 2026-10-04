// privacy-place.js — Your data: what stays on this computer and what leaves
// it, and when. Every case named here is a network call in the code; a new
// one must be added here, or this page is no longer true.

class PrivacyPane {
    constructor(container) {
        this.container = container;
    }

    render() {
        if (this.container.firstChild) return; // nothing on it changes
        const link = (href, label) => `<a href="${href}" target="_blank" rel="noopener noreferrer">${label}</a>`;
        this.container.innerHTML = `
            <div class="guide-pane">
                <div class="gal-head">
                    <h1 class="gal-title">Your data</h1>
                </div>
                <div class="reading-body">
                    <p class="reading-intro">Unterlumen runs on your computer, or on your own server. It has no account, collects no usage data and does not check for updates: it never reports what you do, and it never looks for a new version — you update it yourself.</p>
                    <div class="reading-text">
                        <section class="guide-section">
                            <h2>What stays here</h2>
                            <p>Your photos stay in their folders. Libraries, thumbnails and settings live in Unterlumen's data folder. Titles, fields and publications are written beside a photo into its XMP sidecar, and a location or a crop into the photo itself — only when you set one.</p>
                        </section>
                        <section class="guide-section">
                            <h2>What leaves, and when</h2>
                            <ul class="privacy-list">
                                <li><strong>Maps.</strong> Whenever a map is on screen — the Map, the info panel of a photo with a location, Set location… — your browser loads map tiles from ${link('https://openfreemap.org', 'OpenFreeMap')} (<span class="mono">tiles.openfreemap.org</span>). It sees your IP address and which part of the world the map shows, which is near where those photos were taken. Nothing else of your photos is sent. The coastlines in Space and time come with the app and load nothing.</li>
                                <li><strong>Publishing.</strong> Publish sends a gallery's photos to its destination, and only there: over SSH and rsync to the server you set up for a website, or into a folder for a files destination. Galleries then checks that the published addresses answer, when it opens and after a publish, by asking your own site.</li>
                                <li><strong>Helper programs.</strong> Only when you choose Install the missing ones in Settings does Unterlumen download ffmpeg, exiftool and cwebp — from Homebrew, winget or your package manager, or from ffmpeg.martin-riedl.de, exiftool.org through SourceForge, and Google.</li>
                                <li><strong>Links you open.</strong> GitHub, huepattl.de, OpenStreetMap and the projects under Licenses and thanks open in your browser only when you click them.</li>
                            </ul>
                        </section>
                        <section class="guide-section">
                            <h2>On a server</h2>
                            <p>Unterlumen has no login. Everyone who can reach it — on your network, or beyond it if you bind it there — sees every photo it serves and can change them. Keep it inside your own network, or put a password in front of it.</p>
                        </section>
                        <section class="guide-section">
                            <h2>Websites you publish</h2>
                            <p>The pages Unterlumen builds load nothing from other servers; they link to Unterlumen's page. Whether a photo keeps its location is set by the destination. A public website is yours: in Germany, for example, it needs an imprint and a privacy notice of its own, which Unterlumen does not write for you.</p>
                        </section>
                    </div>
                </div>
            </div>`;
    }
}
