// About dialog — app info, author, and legal notice

class AboutModal {
    constructor() {
        this.overlay = null;
        this._onKeyDown = (e) => { if (e.key === 'Escape') this.close(); };
    }

    open(version) {
        this._build(version);
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

    _build(version) {
        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        this.overlay.innerHTML = `
            <div class="modal about-modal">
                <div class="modal-header">
                    <span class="modal-title">About Unterlumen</span>
                    <button class="info-collapse-btn modal-close-btn">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="about-logo-row">
                        <svg class="about-logo" viewBox="0 0 36 28" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                            <rect x="0" y="0" width="25" height="4" fill="currentColor"/>
                            <rect x="26" y="0" width="10" height="4" fill="currentColor" fill-opacity="0.3"/>
                            <polygon points="0,8 36,8 18,28" fill="#d35400"/>
                        </svg>
                        <div>
                            <div class="about-app-name">Unterlumen</div>
                            <div class="about-app-tagline">Photo browser, culler, and digital asset manager</div>
                            ${version ? `<div class="about-app-version">${version}</div>` : ''}
                        </div>
                    </div>
                    <div class="about-section">
                        <a class="about-link" href="https://github.com/bjblazko/unterlumen" target="_blank" rel="noopener noreferrer">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                                <path d="M12 2C6.477 2 2 6.477 2 12c0 4.418 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.009-.868-.013-1.703-2.782.604-3.369-1.341-3.369-1.341-.454-1.154-1.11-1.462-1.11-1.462-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.832.092-.647.35-1.087.636-1.337-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.564 9.564 0 0 1 12 6.844a9.59 9.59 0 0 1 2.504.337c1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.579.688.481C19.138 20.163 22 16.418 22 12c0-5.523-4.477-10-10-10z"/>
                            </svg>
                            github.com/bjblazko/unterlumen
                        </a>
                    </div>
                    <div class="about-section">
                        <div class="about-label">Author</div>
                        <div class="about-value">Timo Böwing</div>
                        <a class="about-link about-link-sm" href="https://huepattl.de" target="_blank" rel="noopener noreferrer">huepattl.de</a>
                        <a class="about-link about-link-sm" href="mailto:timo.boewing@posteo.de">timo.boewing@posteo.de</a>
                    </div>
                    <div class="about-section about-disclaimer">
                        This software is provided as-is, without warranty of any kind. Always keep backups of your files before using tools that move, rename, or delete files.
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-accent" id="about-close-btn">Close</button>
                </div>
            </div>`;

        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('#about-close-btn').addEventListener('click', () => this.close());
    }
}
