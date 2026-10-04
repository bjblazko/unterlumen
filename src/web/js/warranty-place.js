// warranty-place.js — No warranty: what Unterlumen does to files, that it
// comes without any warranty, and what keeps your photos safe. WarrantyNotice
// says the short version once per browser, at the foot of the sidebar.

class WarrantyPane {
    constructor(container) {
        this.container = container;
    }

    render() {
        if (this.container.firstChild) return; // nothing on it changes
        this.container.innerHTML = `
            <div class="guide-pane">
                <div class="gal-head">
                    <h1 class="gal-title">No warranty</h1>
                </div>
                <div class="reading-body">
                    <p class="reading-intro">Unterlumen is free software, made in spare time and given away under the <a href="https://github.com/bjblazko/unterlumen/blob/main/LICENSE" target="_blank" rel="noopener noreferrer">Apache License 2.0</a>. It comes as it is, without warranty of any kind: it may have faults, and you use it at your own risk.</p>
                    <div class="reading-text">
                        <section class="guide-section">
                            <h2>What it does to your files</h2>
                            <ul class="privacy-list">
                                <li><strong>Deletes</strong> what you marked, when you choose Delete permanently in Marked for deletion. Unterlumen keeps no copy.</li>
                                <li><strong>Moves and renames</strong> files with Rename… and in Organize.</li>
                                <li><strong>Writes into the photo itself</strong> when you crop it or set or remove its location. The original is replaced.</li>
                                <li><strong>Writes beside the photo</strong> an XMP sidecar for titles, fields and publications.</li>
                                <li><strong>Sends</strong> photos to a destination when you publish, and removes them there when you unpublish.</li>
                            </ul>
                        </section>
                        <section class="guide-section">
                            <h2>Keep a backup</h2>
                            <p>Only a backup brings back a photo that is gone or changed. Make one before you use Unterlumen on photos you care about, and keep making them — on another disk, or another place.</p>
                        </section>
                        <section class="guide-section">
                            <h2>Liability</h2>
                            <p>As far as the law allows, the authors are not liable for any damage that comes from using Unterlumen, including lost or changed files. The Apache License 2.0, sections 7 and 8, says this in full.</p>
                        </section>
                    </div>
                </div>
            </div>`;
    }
}

// WarrantyNotice — the short version, once per browser, at the foot of the
// sidebar, so it is beside every place without covering any. It does not
// block; it stays until you say you understood. A phone, which changes no
// files, has no sidebar and does not show it.
const WARRANTY_NOTICE_KEY = 'warranty-notice-seen';

const WarrantyNotice = {
    // Put above the status line at the foot of the sidebar.
    show() {
        if (WarrantyNotice._seen()) return;
        const el = document.createElement('div');
        el.className = 'warranty-notice';
        el.setAttribute('role', 'note');
        el.innerHTML = `
            <p><strong>Unterlumen changes your files when you tell it to</strong> — it deletes, moves, renames and writes into photos. It comes without any warranty. Keep a backup of your photos. ${placeLink('warranty', 'warranty', 'No warranty')}</p>
            <button type="button" class="btn btn-sm">Understood</button>`;
        el.querySelector('button').addEventListener('click', () => {
            try { localStorage.setItem(WARRANTY_NOTICE_KEY, '1'); } catch { /* shown again next time */ }
            el.remove();
        });
        const line = document.getElementById('status-line');
        line.parentNode.insertBefore(el, line);
    },

    _seen() {
        try { return localStorage.getItem(WARRANTY_NOTICE_KEY) === '1'; } catch { return false; }
    },
};
