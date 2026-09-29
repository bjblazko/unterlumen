// Dependency check modal — shows status of external tools with install instructions

// How to install each helper program, per platform; Linux is the fallback.
const DEP_INSTALL = {
    ffmpeg: {
        darwin: 'brew install ffmpeg',
        linux: 'sudo apt install ffmpeg   # Debian/Ubuntu\nsudo dnf install ffmpeg   # Fedora/RHEL',
        windows: 'Download from https://ffmpeg.org/download.html and add to PATH',
    },
    heifConvert: {
        linux: 'sudo apt install libheif-examples   # Debian/Ubuntu\nsudo dnf install libheif   # Fedora/RHEL',
    },
    cwebp: {
        darwin: 'brew install webp',
        linux: 'sudo apt install webp   # Debian/Ubuntu\nsudo dnf install libwebp-tools   # Fedora/RHEL',
        windows: 'Download from https://developers.google.com/speed/webp/download and add to PATH',
    },
    exiftool: {
        darwin: 'brew install exiftool',
        linux: 'sudo apt install libimage-exiftool-perl   # Debian/Ubuntu\nsudo dnf install perl-Image-ExifTool   # Fedora/RHEL',
        windows: 'Download from https://exiftool.org and add to PATH',
    },
};

// A helper program that is either there, or missing with a note and how to
// install it.
function _toolDep(name, desc, ok, missingNote, install) {
    return { name, desc, ok, note: ok ? null : missingNote, install: ok ? null : install };
}

// ffmpeg — WebP export and embedded JPEG stream extraction from HEIF.
function _ffmpegDep(ffmpeg, platform, install) {
    const desc = 'Required for HEIF/HEIC image display and WebP export';
    if (!ffmpeg.available) {
        return {
            name: 'ffmpeg', desc, ok: false,
            note: 'Not installed — HEIF/HEIC images cannot be displayed and WebP export is unavailable.',
            install,
        };
    }
    if (platform !== 'darwin' && !ffmpeg.heifSupport) {
        // On Linux, ffmpeg may lack the HEIF container decoder even if hevc codec is present.
        // heif-convert (below) is the primary decoder; ffmpeg is still needed for WebP export.
        return {
            name: 'ffmpeg',
            desc: 'Required for WebP export and embedded HEIF preview extraction',
            ok: true,
            note: 'Installed. On this platform, HEIF/HEIC display uses heif-convert (see below).',
        };
    }
    if (!ffmpeg.heifSupport) {
        return {
            name: 'ffmpeg', desc, ok: false,
            note: 'Installed, but built without HEVC/HEIF decoder. HEIF/HEIC images cannot be displayed.',
            install,
        };
    }
    return { name: 'ffmpeg', desc, ok: true };
}

// toolsSummary names what is there and what is missing, and what each one
// is for; "some tools missing" tells nobody which feature will not work. The
// check reports a mixture of shapes — {available: true} per tool plus plain
// fields like `platform` — so each tool is read by name.
function toolsSummary(status) {
    const tools = [
        ['exiftool', status.exiftool?.available, 'reading and writing metadata'],
        ['ffmpeg', status.ffmpeg?.available, 'HEIF and video frames'],
        ['sips', status.sips?.available, 'HEIF conversion on macOS'],
        ['heif-convert', status.heifConvert?.available, 'HEIF conversion on Linux'],
    ];
    const found = tools.filter(([, ok]) => ok).map(([name]) => name);
    const missing = tools.filter(([, ok]) => !ok);
    return [
        found.length ? `${found.join(', ')} found` : 'No helper programs found',
        missing.length ? `missing: ${missing.map(([name, , what]) => `${name} (${what})`).join(', ')}` : null,
    ].filter(Boolean).join(' · ');
}

class DepsModal {
    open(status) {
        const deps = this._deps(status, status ? status.platform : 'unknown');
        this._dialog = new Dialog({
            title: 'Helper programs',
            subtitle: 'What each one is for, and how to install it',
            size: 'md',
            body: `<div class="deps-list">${deps.map(d => this._renderDep(d)).join('')}</div>`,
            actions: [{ label: 'Close', value: null }],
        });
        this._dialog.open();
    }

    close() {
        this._dialog?.close(null);
    }

    _deps(status, platform) {
        const ffmpeg = status && status.ffmpeg || {};
        const exiftool = status && status.exiftool || {};
        const sips = status && status.sips || {};
        const heifConvert = status && status.heifConvert || {};
        const webpAvailable = status && status.webpAvailable;
        const install = (tool) => DEP_INSTALL[tool][platform] || DEP_INSTALL[tool].linux || '';

        const deps = [_ffmpegDep(ffmpeg, platform, install('ffmpeg'))];
        // heif-convert (Linux only) — primary HEIF/HEIC decoder when ffmpeg lacks libheif
        if (platform !== 'darwin') {
            deps.push(_toolDep('heif-convert', 'Required for HEIF/HEIC image display on Linux (from libheif-examples)',
                heifConvert.available, 'Not installed — HEIF/HEIC images cannot be displayed.', install('heifConvert')));
        }
        // cwebp — only shown when ffmpeg lacks the libwebp encoder
        if (ffmpeg.available && !ffmpeg.webpSupport) {
            deps.push(_toolDep('cwebp', 'WebP encoder — required for WebP export (ffmpeg on this system lacks libwebp)',
                webpAvailable, 'Not installed — WebP export is unavailable.', install('cwebp')));
        }
        deps.push(_toolDep('exiftool', 'Required for GPS metadata editing and EXIF stripping on export',
            exiftool.available, 'Not installed — GPS location editing and EXIF stripping on export are unavailable.', install('exiftool')));
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
