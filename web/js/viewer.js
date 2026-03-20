// Single image viewer

class Viewer {
    constructor(container) {
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
    }

    close() {
        document.removeEventListener('keydown', this.keyHandler);
        this.infoPanel = null;
        this.filmStripEl = null;
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
            const img = document.createElement('img');
            img.src = API.thumbnailURL(path, 80);
            img.loading = 'lazy';
            thumb.appendChild(img);
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
                this.infoPanel.loadInfo(this.currentPath);
            }
        });
        this.filmStripEl = strip;
    }

    updateFilmStrip(instant) {
        if (!this.filmStripEl) return;
        this.filmStripEl.style.display = this.filmStripVisible ? 'flex' : 'none';
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

    navigate(delta) {
        const newIndex = this.currentIndex + delta;
        if (newIndex < 0 || newIndex >= this.images.length) return;
        this.currentIndex = newIndex;
        this.currentPath = this.images[this.currentIndex];
        this.render();
        this.updateFilmStrip();
        if (this.infoPanel && this.infoPanel.expanded) {
            this.infoPanel.loadInfo(this.currentPath);
        }
    }

    toggleInfo() {
        if (!this.infoPanel) return;
        this.infoPanel.toggle();
        if (this.infoPanel.expanded) {
            this.infoPanel.loadInfo(this.currentPath);
        }
        const btn = this.container.querySelector('.viewer-info');
        if (btn) {
            btn.classList.toggle('active', this.infoPanel.expanded);
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
            this.infoPanel.loadInfo(this.currentPath);
        }
    }

    handleKey(e) {
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
                const cb = this.container.querySelector('.viewer-filmstrip-toggle input');
                if (cb) cb.checked = this.filmStripVisible;
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
        const filename = this.currentPath.split('/').pop();
        const counter = `${this.currentIndex + 1} / ${this.images.length}`;
        const hasPrev = this.currentIndex > 0;
        const hasNext = this.currentIndex < this.images.length - 1;
        const infoActive = this.infoPanel && this.infoPanel.expanded;

        this.container.innerHTML = `
            <div class="viewer">
                <div class="viewer-toolbar">
                    <button class="btn viewer-back" title="Back (Esc)">← Back</button>
                    <span class="viewer-filename">${filename}</span>
                    <label class="viewer-filmstrip-toggle" title="Film strip (F)">
                        <input type="checkbox" ${this.filmStripVisible ? 'checked' : ''}>
                        <span>Film strip</span>
                    </label>
                    <span class="viewer-counter">${counter}</span>
                    <button class="btn viewer-info ${infoActive ? 'active' : ''}" title="Info (I)">Info</button>
                    <button class="btn viewer-delete" title="Mark for deletion (Delete)">Delete</button>
                </div>
                <div class="viewer-content">
                    <div class="viewer-body">
                        <button class="btn viewer-prev ${hasPrev ? '' : 'disabled'}" title="Previous (←)" ${hasPrev ? '' : 'disabled'}>‹</button>
                        <div class="viewer-image-container">
                            <img src="${API.imageURL(this.currentPath)}" alt="${filename}">
                        </div>
                        <button class="btn viewer-next ${hasNext ? '' : 'disabled'}" title="Next (→)" ${hasNext ? '' : 'disabled'}>›</button>
                    </div>
                    <div class="viewer-info-container"></div>
                </div>
            </div>
        `;

        this.container.querySelector('.viewer-back').addEventListener('click', () => this.close());
        this.container.querySelector('.viewer-info').addEventListener('click', () => this.toggleInfo());
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

        const filmToggle = this.container.querySelector('.viewer-filmstrip-toggle input');
        if (filmToggle) {
            filmToggle.addEventListener('change', (e) => {
                this.filmStripVisible = e.target.checked;
                this.updateFilmStrip();
            });
        }

        if (this.infoPanel) {
            this.infoPanel.container = this.container.querySelector('.viewer-info-container');
            this.infoPanel.render();
        }
    }
}
