// Single image viewer

class Viewer {
    constructor(container, options = {}) {
        this.container = container;
        this.images = [];
        this.currentIndex = 0;
        this.currentPath = '';
        this.onClose = null;
        this.onDelete = null;
        this.keyHandler = this.handleKey.bind(this);
        this.infoPanel = null;
        this.filmStripVisible = false;
        this.filmStripEl = null;
        this._filmstripLoaded = false;
        this._imageURLFn = options.imageURLFn || ((p) => API.imageURL(p));
        this._thumbURLFn = options.thumbURLFn || ((p) => API.thumbnailURL(p, 80));
        this._infoLoadFn = options.infoLoadFn || ((p, ip) => ip.loadInfo(p));
        this._cacheBust = null;
        this._cropTool = null;
        this._cropKeyHandler = null;
        this._zoomTool = null;
    }

    open(imagePath, imageList) {
        this.images = imageList || [imagePath];
        this.currentIndex = this.images.indexOf(imagePath);
        if (this.currentIndex < 0) this.currentIndex = 0;
        this.currentPath = this.images[this.currentIndex];

        this.infoPanel = new InfoPanel(document.createElement('div'));
        this.buildFilmStrip();

        document.addEventListener('keydown', this.keyHandler);
        this.render();
        this._prefetch(2);
    }

    close() {
        // Clean up crop mode without triggering a re-render that's about to be discarded
        if (this._cropKeyHandler) {
            document.removeEventListener('keydown', this._cropKeyHandler);
            this._cropKeyHandler = null;
        }
        if (this._cropTool) {
            this._cropTool.destroy();
            this._cropTool = null;
        }
        if (this._zoomTool) { this._zoomTool.destroy(); this._zoomTool = null; }
        document.removeEventListener('keydown', this.keyHandler);
        this.infoPanel = null;
        this.filmStripEl = null;
        this._filmstripLoaded = false;
        if (this.onClose) this.onClose();
    }

    buildFilmStrip() {
        const strip = document.createElement('div');
        strip.className = 'viewer-filmstrip';
        strip.style.display = 'none';
        this.images.forEach((path, i) => {
            const thumb = document.createElement('div');
            thumb.className = 'filmstrip-thumb';
            thumb.dataset.index = i;
            strip.appendChild(thumb);
        });
        strip.addEventListener('click', (e) => {
            const thumb = e.target.closest('.filmstrip-thumb');
            if (!thumb) return;
            const idx = parseInt(thumb.dataset.index, 10);
            if (isNaN(idx) || idx < 0 || idx >= this.images.length) return;
            this.currentIndex = idx;
            this.currentPath = this.images[this.currentIndex];
            this.render();
            this.updateFilmStrip();
            if (this.infoPanel && this.infoPanel.expanded) {
                this._infoLoadFn(this.currentPath, this.infoPanel);
            }
        });
        this.filmStripEl = strip;
    }

    updateFilmStrip(instant) {
        if (!this.filmStripEl) return;
        this.filmStripEl.style.display = this.filmStripVisible ? 'flex' : 'none';
        if (this.filmStripVisible) this._ensureFilmStripLoaded();
        const thumbs = this.filmStripEl.children;
        for (let i = 0; i < thumbs.length; i++) {
            thumbs[i].classList.toggle('filmstrip-active', i === this.currentIndex);
        }
        if (this.filmStripVisible) {
            const active = this.filmStripEl.querySelector('.filmstrip-active');
            if (active) {
                active.scrollIntoView({ inline: 'center', block: 'nearest', behavior: instant ? 'instant' : 'smooth' });
            }
        }
    }

    _ensureFilmStripLoaded() {
        if (this._filmstripLoaded || !this.filmStripEl) return;
        this._filmstripLoaded = true;
        this.images.forEach((path, i) => {
            const thumb = this.filmStripEl.children[i];
            if (!thumb || thumb.querySelector('img')) return;
            const img = document.createElement('img');
            img.src = this._thumbURLFn(path);
            img.loading = 'lazy';
            img.decoding = 'async';
            img.fetchPriority = 'low';
            thumb.appendChild(img);
        });
    }

    navigate(delta) {
        const newIndex = this.currentIndex + delta;
        if (newIndex < 0 || newIndex >= this.images.length) return;
        this.currentIndex = newIndex;
        this.currentPath = this.images[this.currentIndex];
        this.render();
        this.updateFilmStrip();
        if (this.infoPanel && this.infoPanel.expanded) {
            this._infoLoadFn(this.currentPath, this.infoPanel);
        }
        this._prefetch(2);
    }

    toggleInfo() {
        if (!this.infoPanel) return;
        this.infoPanel.toggle();
        if (this.infoPanel.expanded) {
            this._infoLoadFn(this.currentPath, this.infoPanel);
        }
    }

    markCurrentForDeletion() {
        if (this.onDelete) this.onDelete(this.currentPath);

        // Remove corresponding filmstrip thumb and re-index
        if (this.filmStripEl) {
            const thumbs = this.filmStripEl.children;
            if (thumbs[this.currentIndex]) {
                thumbs[this.currentIndex].remove();
            }
            for (let i = 0; i < this.filmStripEl.children.length; i++) {
                this.filmStripEl.children[i].dataset.index = i;
            }
        }

        // Remove from image list
        this.images.splice(this.currentIndex, 1);

        if (this.images.length === 0) {
            this.close();
            return;
        }

        // Adjust index if we were at the end
        if (this.currentIndex >= this.images.length) {
            this.currentIndex = this.images.length - 1;
        }
        this.currentPath = this.images[this.currentIndex];
        this.render();
        this.updateFilmStrip();
        if (this.infoPanel && this.infoPanel.expanded) {
            this._infoLoadFn(this.currentPath, this.infoPanel);
        }
    }

    handleKey(e) {
        if (this._cropTool) return; // crop mode has its own key handler
        switch (e.key) {
            case 'ArrowLeft':
                e.preventDefault();
                this.navigate(-1);
                break;
            case 'ArrowRight':
                e.preventDefault();
                this.navigate(1);
                break;
            case 'Escape':
                e.preventDefault();
                this.close();
                break;
            case 'Backspace':
            case 'Delete':
                e.preventDefault();
                this.markCurrentForDeletion();
                break;
            case 'i':
            case 'I':
                e.preventDefault();
                this.toggleInfo();
                break;
            case 'f':
            case 'F':
                e.preventDefault();
                this.filmStripVisible = !this.filmStripVisible;
                if (this._filmToggle) this._filmToggle.setState(this.filmStripVisible);
                this.updateFilmStrip();
                break;
            case 'h':
            case 'H':
                e.preventDefault();
                App.toggleUIVisibility();
                break;
        }
    }

    render() {
        if (this._zoomTool) { this._zoomTool.destroy(); this._zoomTool = null; }

        const filename = this.currentPath.split('/').pop();
        const counter = `${this.currentIndex + 1} / ${this.images.length}`;
        const hasPrev = this.currentIndex > 0;
        const hasNext = this.currentIndex < this.images.length - 1;
        const infoActive = this.infoPanel && this.infoPanel.expanded;

        this.container.innerHTML = `
            <div class="viewer">
                <div class="viewer-toolbar">
                    <button class="btn viewer-back" title="Back (Esc)"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 2 4 7 9 12"/></svg> Back</button>
                    <span class="viewer-filename">${filename}</span>
                    <span class="viewer-filmstrip-label">Film strip</span>
                    <div class="viewer-filmstrip-toggle-wrap" title="Film strip (F)"></div>
                    <span class="viewer-counter">${counter}</span>
                    <div class="viewer-zoom-group">
                        <button class="btn viewer-zoom-out" title="Zoom out"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" aria-hidden="true"><circle cx="5.5" cy="5.5" r="4"/><line x1="3.5" y1="5.5" x2="7.5" y2="5.5"/><line x1="8.6" y1="8.6" x2="12" y2="12"/></svg></button>
                        <select class="viewer-zoom-select" title="Zoom level">
                            <option value="fit">Fit</option>
                            <option value="5">5%</option>
                            <option value="10">10%</option>
                            <option value="15">15%</option>
                            <option value="25">25%</option>
                            <option value="50">50%</option>
                            <option value="75">75%</option>
                            <option value="100">100%</option>
                            <option value="150">150%</option>
                            <option value="200">200%</option>
                            <option value="300">300%</option>
                            <option value="400">400%</option>
                        </select>
                        <button class="btn viewer-zoom-in" title="Zoom in"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" aria-hidden="true"><circle cx="5.5" cy="5.5" r="4"/><line x1="3.5" y1="5.5" x2="7.5" y2="5.5"/><line x1="5.5" y1="3.5" x2="5.5" y2="7.5"/><line x1="8.6" y1="8.6" x2="12" y2="12"/></svg></button>
                        <button class="btn viewer-zoom-reset" title="Reset to fit" disabled>↺</button>
                    </div>
                    <div class="viewer-action-group">
                        <button class="btn viewer-crop-btn" title="Crop">Crop</button>
                        <button class="btn viewer-delete" title="Mark for deletion (Delete)">Delete</button>
                    </div>
                </div>
                <div class="viewer-content">
                    <div class="viewer-body">
                        <button class="btn viewer-prev ${hasPrev ? '' : 'disabled'}" title="Previous (←)" ${hasPrev ? '' : 'disabled'}><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 4 7 12 15 20"/></svg></button>
                        <div class="viewer-image-container">
                            <img src="${this._currentImageURL()}" alt="${filename}" loading="eager" fetchpriority="high">
                        </div>
                        <button class="btn viewer-next ${hasNext ? '' : 'disabled'}" title="Next (→)" ${hasNext ? '' : 'disabled'}><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 4 17 12 9 20"/></svg></button>
                    </div>
                    <div class="viewer-info-container"></div>
                </div>
            </div>
        `;

        this.container.querySelector('.viewer-back').addEventListener('click', () => this.close());

        const imgEl       = this.container.querySelector('.viewer-image-container img');
        const containerEl = this.container.querySelector('.viewer-image-container');
        this._zoomTool = new ZoomTool(imgEl, containerEl);
        this._zoomTool._onchange = () => this._updateZoomUI();

        this.container.querySelector('.viewer-zoom-out').addEventListener('click', () => this._zoomTool.zoomOut());
        this.container.querySelector('.viewer-zoom-in').addEventListener('click', () => this._zoomTool.zoomIn());
        this.container.querySelector('.viewer-zoom-reset').addEventListener('click', () => this._zoomTool.reset());
        this.container.querySelector('.viewer-zoom-select').addEventListener('change', (e) => {
            const v = e.target.value;
            this._zoomTool.setLevel(v === 'fit' ? 'fit' : parseInt(v, 10));
        });

        this.container.querySelector('.viewer-crop-btn').addEventListener('click', () => this._enterCropMode());
        this.container.querySelector('.viewer-delete').addEventListener('click', () => this.markCurrentForDeletion());
        const prevBtn = this.container.querySelector('.viewer-prev');
        const nextBtn = this.container.querySelector('.viewer-next');
        if (hasPrev) prevBtn.addEventListener('click', () => this.navigate(-1));
        if (hasNext) nextBtn.addEventListener('click', () => this.navigate(1));

        // Re-append persistent film strip element
        if (this.filmStripEl) {
            this.container.querySelector('.viewer').appendChild(this.filmStripEl);
            this.updateFilmStrip(true);
        }

        const filmToggleWrap = this.container.querySelector('.viewer-filmstrip-toggle-wrap');
        if (filmToggleWrap) {
            this._filmToggle = Toggle.create(filmToggleWrap, {
                initial: this.filmStripVisible,
                onChange: (on) => {
                    this.filmStripVisible = on;
                    this.updateFilmStrip();
                }
            });
        }

        if (this.infoPanel) {
            this.infoPanel.container = this.container.querySelector('.viewer-info-container');
            this.infoPanel.render();
            if (this.infoPanel.expanded && !this.infoPanel.data && !this.infoPanel.loading && !this.infoPanel.error) {
                this._infoLoadFn(this.currentPath, this.infoPanel);
            }
        }
    }

    _updateZoomUI() {
        if (!this._zoomTool) return;
        const level  = this._zoomTool.getCurrentLevel();
        const select = this.container.querySelector('.viewer-zoom-select');
        const outBtn = this.container.querySelector('.viewer-zoom-out');
        const inBtn  = this.container.querySelector('.viewer-zoom-in');
        const reset  = this.container.querySelector('.viewer-zoom-reset');
        if (select) select.value = String(level);
        if (outBtn) outBtn.disabled = this._zoomTool.isAtMin();
        if (inBtn)  inBtn.disabled  = this._zoomTool.isAtMax();
        if (reset)  reset.disabled  = (level === 'fit');
    }

    _currentImageURL() {
        const url = this._imageURLFn(this.currentPath);
        return this._cacheBust ? `${url}&t=${this._cacheBust}` : url;
    }

    _prefetch(ahead = 2) {
        this._prefetchCache = [];
        for (let i = 1; i <= ahead; i++) {
            const idx = this.currentIndex + i;
            if (idx >= this.images.length) break;
            const img = new Image();
            img.src = this._imageURLFn(this.images[idx]);
            this._prefetchCache.push(img);
        }
    }

    _enterCropMode() {
        const toolbar = this.container.querySelector('.viewer-toolbar');
        const filename = this.currentPath.split('/').pop();

        toolbar.innerHTML = `
            <button class="btn viewer-crop-cancel" title="Cancel (Esc)">Cancel</button>
            <span class="viewer-filename">${filename}</span>
            <select class="viewer-crop-ratio" title="Aspect ratio">
                <option value="">Free</option>
                <option disabled>──────────</option>
                <option value="1">1:1</option>
                <option value="1.3333">4:3</option>
                <option value="0.75">3:4</option>
                <option value="1.5">3:2</option>
                <option value="0.6667">2:3</option>
                <option value="1.7778">16:9</option>
                <option value="0.5625">9:16</option>
                <option disabled>──────────</option>
                <option value="1.85">1.85:1 Flat</option>
                <option value="2.35">2.35:1 Anamorphic</option>
                <option value="2.39">2.39:1 DCI Scope</option>
            </select>
            <button class="btn viewer-crop-apply active" title="Apply crop (Enter)">Apply Crop</button>
        `;

        toolbar.querySelector('.viewer-crop-cancel').addEventListener('click', () => this._exitCropMode());
        toolbar.querySelector('.viewer-crop-apply').addEventListener('click', () => this._applyCrop());
        toolbar.querySelector('.viewer-crop-ratio').addEventListener('change', (e) => {
            const val = e.target.value;
            if (this._cropTool) this._cropTool.setAspectRatio(val ? parseFloat(val) : null);
        });

        // Hide nav and delete buttons — they remain in the DOM but are not usable during crop
        const prevBtn = this.container.querySelector('.viewer-prev');
        const nextBtn = this.container.querySelector('.viewer-next');
        if (prevBtn) prevBtn.style.visibility = 'hidden';
        if (nextBtn) nextBtn.style.visibility = 'hidden';

        // Reset zoom to fit so the overlay covers the full visible image.
        if (this._zoomTool) this._zoomTool.reset();

        const img = this.container.querySelector('.viewer-image-container img');
        this._cropTool = new CropTool(img);

        // Add modal-overlay class so app-keyboard.js defers on this viewer
        this._cropTool._overlay.classList.add('modal-overlay');

        this._cropKeyHandler = (e) => {
            if (e.key === 'Escape') { e.preventDefault(); e.stopImmediatePropagation(); this._exitCropMode(); }
            if (e.key === 'Enter')  { e.preventDefault(); e.stopImmediatePropagation(); this._applyCrop(); }
        };
        document.addEventListener('keydown', this._cropKeyHandler);
    }

    _exitCropMode() {
        if (this._cropKeyHandler) {
            document.removeEventListener('keydown', this._cropKeyHandler);
            this._cropKeyHandler = null;
        }
        if (this._cropTool) {
            this._cropTool.destroy();
            this._cropTool = null;
        }
        this.render();
    }

    async _applyCrop() {
        if (!this._cropTool) return;
        const rect = this._cropTool.getRect();
        if (!rect) return;

        const applyBtn = this.container.querySelector('.viewer-crop-apply');
        if (applyBtn) {
            applyBtn.disabled = true;
            applyBtn.textContent = 'Saving…';
        }

        try {
            await API.crop(this.currentPath, rect.x, rect.y, rect.width, rect.height);
            this._cacheBust = Date.now();
            // Refresh the film strip thumbnail so it reflects the cropped image.
            if (this.filmStripEl) {
                const thumb = this.filmStripEl.children[this.currentIndex];
                if (thumb) {
                    const tImg = thumb.querySelector('img');
                    if (tImg) tImg.src = this._thumbURLFn(this.currentPath) + '&t=' + this._cacheBust;
                }
            }
            this._exitCropMode();
        } catch (err) {
            if (applyBtn) {
                applyBtn.disabled = false;
                applyBtn.textContent = 'Apply Crop';
            }
            const toolbar = this.container.querySelector('.viewer-toolbar');
            if (toolbar) {
                let errEl = toolbar.querySelector('.viewer-crop-error');
                if (!errEl) {
                    errEl = document.createElement('span');
                    errEl.className = 'viewer-crop-error';
                    toolbar.appendChild(errEl);
                }
                errEl.textContent = err.message || 'Crop failed';
            }
        }
    }
}
