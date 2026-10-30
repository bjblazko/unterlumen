// viewer-video.js — a video in the viewer, and the clip of a Live Photo
// (ADR-0058). The browser plays them; what it cannot play is said in a
// sentence with the way to the file.

function viewerVideoHTML(src, poster, filename) {
    return `<video class="viewer-video" src="${escapeHtml(src)}"${poster ? ` poster="${escapeHtml(poster)}"` : ''}
        controls playsinline preload="metadata" aria-label="${escapeHtml(filename)}"></video>`;
}

const ViewerVideo = {
    // watch says so when the browser cannot play the video, and how to get
    // it anyway.
    watch(videoEl, containerEl, downloadURL) {
        videoEl.addEventListener('error', () => {
            videoEl.remove();
            const note = document.createElement('div');
            note.className = 'viewer-video-failed';
            note.innerHTML = `<p>${t('This video cannot be played in the browser: its format is not one the browser knows.')}</p>
                <a class="btn" href="${escapeHtml(downloadURL)}" download>${t('Download the original file')}</a>`;
            containerEl.appendChild(note);
        }, { once: true });
    },

    // toggle plays or pauses the video on screen; false when there is none.
    toggle(container) {
        const video = container.querySelector('.viewer-video');
        // A focused video takes Space itself.
        if (!video || document.activeElement === video) return false;
        if (video.paused) video.play().catch(() => {});
        else video.pause();
        return true;
    },

    // playLive plays a Live Photo's clip over the photo once, then shows
    // the photo again.
    playLive(containerEl, url) {
        if (containerEl.querySelector('.viewer-live-clip')) return;
        const clip = document.createElement('video');
        clip.className = 'viewer-live-clip';
        clip.playsInline = true;
        clip.src = url;
        const done = () => clip.remove();
        clip.addEventListener('ended', done, { once: true });
        clip.addEventListener('error', done, { once: true });
        containerEl.appendChild(clip);
        clip.play().catch(done);
    },
};
