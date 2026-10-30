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
                ${placeHeaderHTML({ title: t('Your data') })}
                <div class="reading-body">
                    <p class="reading-intro">${t('Unterlumen runs on your computer, or on your own server. It has no account, collects no usage data and does not check for updates: it never reports what you do, and it never looks for a new version — you update it yourself.')}</p>
                    <div class="reading-text">
                        <section class="guide-section">
                            <h2>${t('What stays here')}</h2>
                            <p>${t('Your photos stay in their folders. Libraries, thumbnails and settings live in Unterlumen\'s data folder. Titles, fields, publications, the names you give faces and the things found in a photo are written beside a photo into its XMP sidecar, and a location or a crop into the photo itself — only when you set one.')}</p>
                            <p>${t('The short clips a video plays in its tile are made on this computer and kept in Unterlumen\'s cache beside the thumbnails. Videos themselves are never changed.')}</p>
                        </section>
                        <section class="guide-section">
                            <h2>${t('What leaves, and when')}</h2>
                            <ul class="privacy-list">
                                <li><strong>${t('Maps.')}</strong> ${t('Whenever a map is on screen — the Map, the info panel of a photo with a location, Set location… — your browser loads map tiles from {link} ({host}). It sees your IP address and which part of the world the map shows, which is near where those photos were taken. Nothing else of your photos is sent. The coastlines in Space and time come with the app and load nothing.', { link: link('https://openfreemap.org', 'OpenFreeMap'), host: '<span class="mono">tiles.openfreemap.org</span>' })}</li>
                                <li><strong>${t('Publishing.')}</strong> ${t('Publish sends a gallery\'s photos to its destination, and only there: over SSH and rsync to the server you set up for a website, or into a folder for a files destination. Galleries then checks that the published addresses answer, when it opens and after a publish, by asking your own site.')}</li>
                                <li><strong>${t('Helper programs.')}</strong> ${t('Only when you choose Install the missing ones in Settings does Unterlumen download ffmpeg, exiftool and cwebp — from Homebrew, winget or your package manager, or from ffmpeg.martin-riedl.de, exiftool.org through SourceForge, and Google.')}</li>
                                <li><strong>${t('Face recognition.')}</strong> ${t('Only when you choose Download and turn on in People does Unterlumen download ONNX Runtime from Microsoft’s releases on GitHub ({github}) and two face models from OpenCV’s model zoo on Hugging Face ({hf}), once. Faces are then found and compared on this computer; no photo, face or name is sent anywhere.', { github: '<span class="mono">github.com</span>', hf: '<span class="mono">huggingface.co</span>' })}</li>
                                <li><strong>${t('Things.')}</strong> ${t('Only when you choose Download and turn on in Statistics, Things, does Unterlumen download Google’s SigLIP 2 from Hugging Face ({hf}), an object detector from OpenCV’s model zoo there, and ONNX Runtime from GitHub ({github}) unless face recognition already did, once. Things are then found, and searches by words compared, on this computer; no photo and no search is sent anywhere.', { github: '<span class="mono">github.com</span>', hf: '<span class="mono">huggingface.co</span>' })}</li>
                                <li><strong>${t('Links you open.')}</strong> ${t('GitHub, huepattl.de, OpenStreetMap and the projects under Licenses and thanks open in your browser only when you click them.')}</li>
                            </ul>
                        </section>
                        <section class="guide-section">
                            <h2>${t('On a server')}</h2>
                            <p>${t('Unterlumen has no login. Everyone who can reach it — on your network, or beyond it if you bind it there — sees every photo it serves and can change them. Keep it inside your own network, or put a password in front of it.')}</p>
                        </section>
                        <section class="guide-section">
                            <h2>${t('Websites you publish')}</h2>
                            <p>${t('The pages Unterlumen builds load nothing from other servers; they link to Unterlumen\'s page. Whether a photo keeps its location is set by the destination. A public website is yours: in Germany, for example, it needs an imprint and a privacy notice of its own, which Unterlumen does not write for you.')}</p>
                        </section>
                    </div>
                </div>
            </div>`;
    }
}
