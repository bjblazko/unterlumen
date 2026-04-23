// SlideshowPlayer — full-screen timed image slideshow.
// Independent of Viewer; mounted by App.openSlideshow().

class SlideshowPlayer {
    constructor(container) {
        this.container = container;
        this.onClose = null;
        this._images = [];
        this._options = {};
        this._cursor = 0;
        this._running = false;
        this._timer = null;
        this._stage = null;
        this._hud = null;
        this._hudTimer = null;
        this._counterEl = null;
        this._playPauseBtn = null;
        this._iconPause = null;
        this._iconPlay = null;
        this._playPauseLbl = null;
        this._audio = null;
        this._audioFiles = [];
        this._audioIndex = 0;
        this._audioObjectURLs = [];
        this._preloadCache = [];
        this._handleKey = this._handleKey.bind(this);
        this._onMouseMove = this._showHUD.bind(this);
    }

    open(images, options) {
        this._images = images;
        this._options = options;
        this._cursor = 0;
        this._running = true;
        this._buildDOM();
        this._setupAudio(options);
        document.addEventListener('keydown', this._handleKey);
        this.container.addEventListener('mousemove', this._onMouseMove);
        this._start();
    }

    close() {
        this._running = false;
        clearTimeout(this._timer);
        clearTimeout(this._hudTimer);
        if (this._audio) {
            this._audio.pause();
            this._audio = null;
        }
        this._audioObjectURLs.forEach(url => URL.revokeObjectURL(url));
        this._audioObjectURLs = [];
        this._preloadCache = [];
        document.removeEventListener('keydown', this._handleKey);
        this.container.removeEventListener('mousemove', this._onMouseMove);
        if (this.onClose) this.onClose();
    }

    _buildDOM() {
        this.container.innerHTML = `
            <div class="ss-overlay">
                <div class="ss-stage"></div>
                <div class="ss-hud">
                    <button class="ss-btn ss-btn-prev" title="Previous (←)">
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="8,2 4,6 8,10"/></svg>
                        Prev
                    </button>
                    <button class="ss-btn ss-btn-playpause" title="Pause (Space)">
                        <svg class="ss-icon-pause" width="12" height="12" viewBox="0 0 12 12" fill="currentColor"><rect x="2" y="2" width="3" height="8" rx="0.5"/><rect x="7" y="2" width="3" height="8" rx="0.5"/></svg>
                        <svg class="ss-icon-play" width="12" height="12" viewBox="0 0 12 12" fill="currentColor" style="display:none"><polygon points="2,2 10,6 2,10"/></svg>
                        <span class="ss-playpause-label">Pause</span>
                    </button>
                    <button class="ss-btn ss-btn-next" title="Next (→)">
                        Next
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="4,2 8,6 4,10"/></svg>
                    </button>
                    <span class="ss-counter"></span>
                    <button class="ss-btn ss-btn-close" title="Close (Esc)">
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><line x1="2" y1="2" x2="10" y2="10"/><line x1="10" y1="2" x2="2" y2="10"/></svg>
                        Close
                    </button>
                </div>
            </div>
        `;

        this._stage = this.container.querySelector('.ss-stage');
        this._hud = this.container.querySelector('.ss-hud');
        this._counterEl = this.container.querySelector('.ss-counter');
        this._playPauseBtn = this.container.querySelector('.ss-btn-playpause');
        this._iconPause = this._playPauseBtn.querySelector('.ss-icon-pause');
        this._iconPlay = this._playPauseBtn.querySelector('.ss-icon-play');
        this._playPauseLbl = this._playPauseBtn.querySelector('.ss-playpause-label');

        this.container.querySelector('.ss-btn-prev').addEventListener('click', () => this._prev());
        this.container.querySelector('.ss-btn-next').addEventListener('click', () => this._skip());
        this._playPauseBtn.addEventListener('click', () => this._playPause());
        this.container.querySelector('.ss-btn-close').addEventListener('click', () => this.close());

        this._showHUD();
    }

    _setupAudio(options) {
        if (options.audioMode === 'none' || !options.audioFiles || options.audioFiles.length === 0) return;

        const files = [...options.audioFiles];
        for (let i = files.length - 1; i > 0; i--) {
            const j = Math.floor(Math.random() * (i + 1));
            [files[i], files[j]] = [files[j], files[i]];
        }
        this._audioFiles = files;
        this._audioIndex = 0;

        this._audio = new Audio();
        this._audio.volume = 0.7;

        const playNext = () => {
            // Revoke the now-finished URL before creating the next one
            if (this._audioObjectURLs.length > 0) {
                URL.revokeObjectURL(this._audioObjectURLs.shift());
            }
            this._audioIndex = (this._audioIndex + 1) % this._audioFiles.length;
            const url = URL.createObjectURL(this._audioFiles[this._audioIndex]);
            this._audioObjectURLs.push(url);
            this._audio.src = url;
            this._audio.play().catch(() => {});
        };

        this._audio.addEventListener('ended', playNext);

        const firstURL = URL.createObjectURL(this._audioFiles[0]);
        this._audioObjectURLs.push(firstURL);
        this._audio.src = firstURL;
        this._audio.play().catch(() => {});
    }

    _start() {
        this._showFrame(this._currentPaths());
        this._scheduleNext();
    }

    _scheduleNext() {
        if (!this._running) return;
        const delayMs = (this._options.delay || 5) * 1000;
        this._timer = setTimeout(() => this._advance(), delayMs);
    }

    _advance() {
        if (!this._running) return;
        const step = this._step();
        this._cursor = (this._cursor + step) % this._images.length;
        this._showFrame(this._currentPaths());
        this._scheduleNext();
    }

    _prev() {
        clearTimeout(this._timer);
        const step = this._step();
        this._cursor = (this._cursor - step + this._images.length) % this._images.length;
        this._showFrame(this._currentPaths());
        if (this._running) this._scheduleNext();
    }

    _skip() {
        clearTimeout(this._timer);
        this._advance();
    }

    _step() {
        const d = this._options.display;
        if (d === '2up') return 2;
        if (d === '4up') return 4;
        return 1;
    }

    _currentPaths() {
        const step = this._step();
        const paths = [];
        for (let i = 0; i < step; i++) {
            paths.push(this._images[(this._cursor + i) % this._images.length]);
        }
        return paths;
    }

    _showFrame(paths) {
        const { transition, display } = this._options;
        const delay = this._options.delay || 5;
        const isKenBurns = display === 'kenburns';
        const effectiveTransition = isKenBurns ? 'fade' : (transition || 'fade');
        // Safety fallback duration matches the longest CSS animation (fade=600ms, slide/zoom=500ms)
        const animMs = effectiveTransition === 'fade' ? 600 : effectiveTransition === 'instant' ? 0 : 500;

        const frame = document.createElement('div');
        frame.className = 'ss-frame';
        frame.dataset.display = display === 'single' ? 'single' : display;

        const buildImg = (path) => {
            const img = document.createElement('img');
            img.className = 'ss-img';
            img.src = API.imageURL(path);
            img.decoding = 'async';
            return img;
        };

        if (display === 'single' || isKenBurns) {
            const img = buildImg(paths[0]);
            frame.appendChild(img);
            if (isKenBurns) {
                const kb = this._randomKenBurns();
                img.animate(
                    [{ transform: kb.from }, { transform: kb.to }],
                    { duration: delay * 1000, easing: 'ease-in-out', fill: 'forwards' }
                );
            }
        } else {
            paths.forEach(path => {
                const cell = document.createElement('div');
                cell.className = 'ss-cell';
                cell.appendChild(buildImg(path));
                frame.appendChild(cell);
            });
        }

        const outgoing = this._stage.querySelector('.ss-frame');
        if (outgoing && effectiveTransition !== 'instant') {
            outgoing.classList.add(`ss-exit-${effectiveTransition}`);
            outgoing.addEventListener('animationend', () => outgoing.remove(), { once: true });
            setTimeout(() => { if (outgoing.parentNode) outgoing.remove(); }, animMs + 50);
        } else if (outgoing) {
            outgoing.remove();
        }

        this._stage.appendChild(frame);
        if (effectiveTransition !== 'instant') {
            frame.classList.add(`ss-enter-${effectiveTransition}`);
        }

        this._updateCounter();
        this._preloadNext();
    }

    _randomKenBurns() {
        const rand = (min, max) => Math.random() * (max - min) + min;
        const sign = () => Math.random() < 0.5 ? 1 : -1;
        const scaleA = rand(1.0, 1.06);
        const scaleB = rand(1.09, 1.18);
        const [startScale, endScale] = Math.random() < 0.5 ? [scaleA, scaleB] : [scaleB, scaleA];
        const startX = rand(0, 4) * sign();
        const startY = rand(0, 3) * sign();
        const endX = rand(0, 4) * sign();
        const endY = rand(0, 3) * sign();
        const fmt = n => n.toFixed(2);
        return {
            from: `scale(${fmt(startScale)}) translate(${fmt(startX)}%, ${fmt(startY)}%)`,
            to:   `scale(${fmt(endScale)}) translate(${fmt(endX)}%, ${fmt(endY)}%)`,
        };
    }

    _preloadNext() {
        const step = this._step();
        const nextCursor = (this._cursor + step) % this._images.length;
        // Keep Image objects alive so the browser can complete the load and cache the response
        this._preloadCache = [];
        for (let i = 0; i < step; i++) {
            const img = new Image();
            img.src = API.imageURL(this._images[(nextCursor + i) % this._images.length]);
            this._preloadCache.push(img);
        }
    }

    _updateCounter() {
        const step = this._step();
        this._counterEl.textContent =
            `${Math.floor(this._cursor / step) + 1} / ${Math.ceil(this._images.length / step)}`;
    }

    _playPause() {
        this._running = !this._running;
        if (this._running) {
            this._iconPause.style.display = '';
            this._iconPlay.style.display = 'none';
            this._playPauseLbl.textContent = 'Pause';
            if (this._audio) this._audio.play().catch(() => {});
            this._scheduleNext();
        } else {
            this._iconPause.style.display = 'none';
            this._iconPlay.style.display = '';
            this._playPauseLbl.textContent = 'Play';
            clearTimeout(this._timer);
            if (this._audio) this._audio.pause();
        }
    }

    _showHUD() {
        if (!this._hud) return;
        this._hud.classList.remove('ss-hud-hidden');
        clearTimeout(this._hudTimer);
        this._hudTimer = setTimeout(() => {
            if (this._hud) this._hud.classList.add('ss-hud-hidden');
        }, 3000);
    }

    _handleKey(e) {
        switch (e.key) {
            case ' ':
                e.preventDefault();
                this._playPause();
                this._showHUD();
                break;
            case 'ArrowLeft':
                e.preventDefault();
                this._prev();
                this._showHUD();
                break;
            case 'ArrowRight':
                e.preventDefault();
                this._skip();
                this._showHUD();
                break;
            case 'Escape':
                e.preventDefault();
                this.close();
                break;
        }
    }
}
