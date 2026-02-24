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
    }

    open(imagePath, imageList) {
        this.images = imageList || [imagePath];
        this.currentIndex = this.images.indexOf(imagePath);
        if (this.currentIndex < 0) this.currentIndex = 0;
        this.currentPath = this.images[this.currentIndex];

        this.infoPanel = new InfoPanel(document.createElement('div'));

        document.addEventListener('keydown', this.keyHandler);
        this.render();
    }

    close() {
        document.removeEventListener('keydown', this.keyHandler);
        this.infoPanel = null;
        if (this.onClose) this.onClose();
    }

    navigate(delta) {
        const newIndex = this.currentIndex + delta;
        if (newIndex < 0 || newIndex >= this.images.length) return;
        this.currentIndex = newIndex;
        this.currentPath = this.images[this.currentIndex];
        this.render();
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
            case 'Backspace':
                e.preventDefault();
                this.close();
                break;
            case 'Delete':
                e.preventDefault();
                this.markCurrentForDeletion();
                break;
            case 'i':
            case 'I':
                e.preventDefault();
                this.toggleInfo();
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

        if (this.infoPanel) {
            this.infoPanel.container = this.container.querySelector('.viewer-info-container');
            this.infoPanel.render();
        }
    }
}
