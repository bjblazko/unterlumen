// video.js — videos among the photos (ADR-0058): how a tile says it is a
// video, the short clip it plays under the pointer, and whether videos are
// shown at all. Videos are listed and played, never edited.

const VIDEO_EXTS = ['mp4', 'm4v', 'mov', 'webm', 'mkv', 'avi', 'mts', 'm2ts', '3gp'];

function isVideoName(name) {
    return VIDEO_EXTS.includes(String(name).split('.').pop().toLowerCase());
}

// formatDuration writes seconds as a clock: "0:42", "12:05", "1:02:03".
function formatDuration(seconds) {
    const s = Math.max(0, Math.round(Number(seconds) || 0));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const ss = String(s % 60).padStart(2, '0');
    return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

// videoBadgeHTML is what a tile always carries over its picture: a video's
// length (or the word until it is known), or "Live" for a Live Photo.
function videoBadgeHTML(entry, meta) {
    if (entry.video) {
        const text = meta?.duration ? formatDuration(meta.duration) : t('Video');
        return `<span class="video-badge num" title="${t('Video')}">${text}</span>`;
    }
    if (entry.live) return `<span class="video-badge" title="${t('Live Photo')}">${t('Live')}</span>`;
    return '';
}

// updateVideoBadges puts each video's length on its tile once it has come.
function updateVideoBadges(container, metaOf) {
    container.querySelectorAll('[data-video]').forEach(el => {
        const duration = metaOf(el.dataset.name)?.duration;
        const badge = el.querySelector('.video-badge');
        if (duration && badge) badge.textContent = formatDuration(duration);
    });
}

// VideoHover plays a video tile's clip while the pointer rests on it, and
// only where there is a pointer to rest: a phone shows the still frame.
const VideoHover = {
    attach(container, clipURLOf) {
        if (!window.matchMedia('(hover: hover)').matches) return;
        container.addEventListener('pointerover', e => {
            const item = e.target.closest('.image-item[data-video]');
            if (!item || item.querySelector('.tile-clip')) return;
            const url = clipURLOf(item.dataset.name);
            if (!url) return;
            const video = document.createElement('video');
            video.className = 'tile-clip';
            video.muted = true;
            video.loop = true;
            video.playsInline = true;
            video.preload = 'auto';
            video.setAttribute('aria-hidden', 'true');
            video.addEventListener('playing', () => video.classList.add('tile-clip-playing'), { once: true });
            video.addEventListener('error', () => video.remove(), { once: true });
            video.src = url;
            const img = item.querySelector('img');
            if (!img) return;
            // Over the picture exactly, whatever the layout puts beside it.
            Object.assign(video.style, {
                left: `${img.offsetLeft}px`, top: `${img.offsetTop}px`,
                width: `${img.offsetWidth}px`, height: `${img.offsetHeight}px`,
            });
            img.after(video);
            video.play().catch(() => {});
        });
        container.addEventListener('pointerout', e => {
            const item = e.target.closest('.image-item[data-video]');
            if (!item || item.contains(e.relatedTarget)) return;
            // A detail of the tile drawn anew under the pointer also ends
            // its hover for a moment; only a pointer that left stops it.
            requestAnimationFrame(() => {
                if (!item.matches(':hover')) VideoHover._stop(item);
            });
        });
    },

    _stop(item) {
        const video = item.querySelector('.tile-clip');
        if (!video) return;
        video.pause();
        video.removeAttribute('src');
        video.load(); // lets go of the request
        video.remove();
    },
};

// VideoPrefs remembers, per browser, whether videos are shown among the
// photos. They are, unless the person hid them.
const VideoPrefs = {
    _key: 'unterlumen.videosShown',
    shown() {
        try { return localStorage.getItem(this._key) !== 'no'; } catch { return true; }
    },
    setShown(on) {
        try { localStorage.setItem(this._key, on ? 'yes' : 'no'); } catch { /* not remembered */ }
    },
};
