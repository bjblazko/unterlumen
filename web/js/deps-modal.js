// Dependency check modal — shows status of external tools with install instructions

class DepsModal {
    constructor() {
        this.overlay = null;
        this._onKeyDown = (e) => { if (e.key === 'Escape') this.close(); };
    }

    open(status) {
        this._build(status);
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);
    }

    close() {
        document.removeEventListener('keydown', this._onKeyDown);
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
        }
    }

    _build(status) {
        const platform = status ? status.platform : 'unknown';

        const deps = this._deps(status, platform);

        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        this.overlay.innerHTML = `
            <div class="modal" style="max-width:520px">
                <div class="modal-header">
                    <span class="modal-title">Dependencies</span>
                    <button class="info-collapse-btn modal-close-btn">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="deps-list">${deps.map(d => this._renderDep(d)).join('')}</div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-accent" id="deps-close-btn">Close</button>
                </div>
            </div>`;

        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('#deps-close-btn').addEventListener('click', () => this.close());
    }

    _deps(status, platform) {
        const ffmpeg = status && status.ffmpeg || {};
        const exiftool = status && status.exiftool || {};
        const sips = status && status.sips || {};

        const install = {
            ffmpeg: {
                darwin: 'brew install ffmpeg',
                linux: 'sudo apt install ffmpeg   # Debian/Ubuntu\nsudo dnf install ffmpeg   # Fedora/RHEL',
                windows: 'Download from https://ffmpeg.org/download.html and add to PATH',
            },
            exiftool: {
                darwin: 'brew install exiftool',
                linux: 'sudo apt install libimage-exiftool-perl   # Debian/Ubuntu\nsudo dnf install perl-Image-ExifTool   # Fedora/RHEL',
                windows: 'Download from https://exiftool.org and add to PATH',
            },
        };

        const get = (map) => map[platform] || map['linux'] || '';

        const deps = [];

        // ffmpeg
        if (!ffmpeg.available) {
            deps.push({
                name: 'ffmpeg',
                desc: 'Required for HEIF/HEIC image display and WebP export',
                ok: false,
                note: 'Not installed — HEIF/HEIC images cannot be displayed and WebP export is unavailable.',
                install: get(install.ffmpeg),
            });
        } else if (!ffmpeg.heifSupport) {
            deps.push({
                name: 'ffmpeg',
                desc: 'Required for HEIF/HEIC image display and WebP export',
                ok: false,
                note: 'Installed, but built without HEVC/HEIF decoder. HEIF/HEIC images cannot be displayed.',
                install: get(install.ffmpeg),
            });
        } else {
            deps.push({
                name: 'ffmpeg',
                desc: 'Required for HEIF/HEIC image display and WebP export',
                ok: true,
            });
        }

        // exiftool
        deps.push({
            name: 'exiftool',
            desc: 'Required for GPS metadata editing and EXIF stripping on export',
            ok: exiftool.available,
            note: exiftool.available ? null : 'Not installed — GPS location editing and EXIF stripping on export are unavailable.',
            install: exiftool.available ? null : get(install.exiftool),
        });

        // sips (macOS only)
        if (platform === 'darwin') {
            deps.push({
                name: 'sips',
                desc: 'Built-in macOS image tool used as fallback for HEIF conversion',
                ok: sips.available,
                note: sips.available ? null : 'Not found — should be present on all macOS systems.',
                install: null,
            });
        }

        return deps;
    }

    _renderDep(dep) {
        const icon = dep.ok
            ? `<span class="dep-icon dep-ok">&#10003;</span>`
            : `<span class="dep-icon dep-warn">&#9888;</span>`;

        let detail = '';
        if (!dep.ok && dep.note) {
            const installBlock = dep.install
                ? `<div class="dep-install"><pre>${this._esc(dep.install)}</pre></div>`
                : '';
            detail = `<div class="dep-note">${this._esc(dep.note)}${installBlock}</div>`;
        }

        return `
            <div class="dep-row">
                <div class="dep-header">
                    ${icon}
                    <span class="dep-name">${this._esc(dep.name)}</span>
                    <span class="dep-desc">${this._esc(dep.desc)}</span>
                </div>
                ${detail}
            </div>`;
    }

    _esc(s) {
        return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
}
