// SlideshowModal — options dialog for the slideshow player.
// Follows the same class pattern as ExportModal and BatchRenameModal.

class SlideshowModal {
    constructor() {
        this.overlay = null;
        this.onStart = null;
        this._images = [];
        this._audioFiles = [];  // persists within session — not reset on re-open
        this._audioMode = 'none';
        this._onKeyDown = this._onKeyDown.bind(this);
    }

    open(images) {
        if (this.overlay) this.close();
        this._images = images;
        this._buildDOM();
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);
    }

    close() {
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
        }
        document.removeEventListener('keydown', this._onKeyDown);
    }

    _onKeyDown(e) {
        if (e.key === 'Escape') this.close();
    }

    _buildDOM() {
        const count = this._images.length;
        const label = count === 1 ? '1 image' : `${count} images`;

        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.innerHTML = `
            <div class="modal slideshow-modal">
                <div class="modal-header">
                    <span class="modal-title">Slideshow — ${label}</span>
                    <button class="info-collapse-btn modal-close-btn" title="Close">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Delay</span>
                        <div class="ss-delay-row">
                            <input type="range" class="ss-delay-range" min="1" max="60" value="5">
                            <span class="ss-delay-value">5 s</span>
                        </div>
                    </div>
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Transition</span>
                        <div class="dropdown-toggle">
                            <button class="btn btn-sm active" data-transition="fade">Fade</button>
                            <button class="btn btn-sm" data-transition="slide">Slide</button>
                            <button class="btn btn-sm" data-transition="zoom">Zoom</button>
                            <button class="btn btn-sm" data-transition="instant">Instant</button>
                        </div>
                    </div>
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Display</span>
                        <div class="dropdown-toggle">
                            <button class="btn btn-sm active" data-display="single">Single</button>
                            <button class="btn btn-sm" data-display="kenburns">Ken Burns</button>
                            <button class="btn btn-sm" data-display="2up">2-up</button>
                            <button class="btn btn-sm" data-display="4up">4-up</button>
                        </div>
                    </div>
                    <div class="ss-opt-section ss-opt-audio-section">
                        <span class="ss-opt-label">Audio</span>
                        <div class="ss-audio-options">
                            <label class="export-radio-row">
                                <input type="radio" name="ss-audio" value="none" checked> None
                            </label>
                            <label class="export-radio-row">
                                <input type="radio" name="ss-audio" value="file"> File…
                            </label>
                            <label class="export-radio-row">
                                <input type="radio" name="ss-audio" value="folder"> Folder…
                            </label>
                            <div class="ss-audio-file-label"></div>
                        </div>
                    </div>
                    <input type="file" class="ss-file-input" accept="audio/*" style="display:none">
                    <input type="file" class="ss-folder-input" accept="audio/*" style="display:none" webkitdirectory>
                </div>
                <div class="modal-footer">
                    <button class="btn ss-cancel-btn">Cancel</button>
                    <button class="btn btn-accent ss-start-btn">Start</button>
                </div>
            </div>
        `;

        // Close button
        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('.ss-cancel-btn').addEventListener('click', () => this.close());

        // Click outside to close
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        // Delay range
        const range = this.overlay.querySelector('.ss-delay-range');
        const delayLabel = this.overlay.querySelector('.ss-delay-value');
        range.addEventListener('input', () => {
            delayLabel.textContent = `${range.value} s`;
        });

        // Segmented toggles (transition + display)
        this.overlay.querySelectorAll('.dropdown-toggle').forEach(group => {
            group.querySelectorAll('.btn').forEach(btn => {
                btn.addEventListener('click', () => {
                    group.querySelectorAll('.btn').forEach(b => b.classList.remove('active'));
                    btn.classList.add('active');
                });
            });
        });

        // Audio radios + file pickers
        const fileInput = this.overlay.querySelector('.ss-file-input');
        const folderInput = this.overlay.querySelector('.ss-folder-input');
        const audioLabel = this.overlay.querySelector('.ss-audio-file-label');

        const updateAudioLabel = (files) => {
            this._audioFiles = Array.from(files);
            if (files.length === 1) {
                audioLabel.textContent = files[0].name;
            } else if (files.length > 1) {
                audioLabel.textContent = `${files.length} files selected`;
            } else {
                audioLabel.textContent = '';
            }
        };

        // Restore state from current session or localStorage
        const restoredMode = this._audioMode !== 'none' && this._audioFiles.length > 0
            ? this._audioMode
            : localStorage.getItem('slideshow-audio-mode') || 'none';
        const restoredRadio = this.overlay.querySelector(`input[name="ss-audio"][value="${restoredMode}"]`);
        if (restoredRadio) restoredRadio.checked = true;

        if (this._audioFiles.length > 0) {
            // Files still in memory from this session — show their names
            updateAudioLabel(this._audioFiles);
        } else if (restoredMode !== 'none') {
            // New session: show saved label as a reminder to re-pick
            const savedLabel = localStorage.getItem('slideshow-audio-label') || '';
            if (savedLabel) audioLabel.textContent = `Last: ${savedLabel} — re-select to use`;
        }

        this.overlay.querySelectorAll('input[name="ss-audio"]').forEach(radio => {
            radio.addEventListener('change', () => {
                this._audioMode = radio.value;
                if (radio.value === 'file') {
                    fileInput.click();
                } else if (radio.value === 'folder') {
                    folderInput.click();
                } else {
                    this._audioFiles = [];
                    audioLabel.textContent = '';
                }
            });
        });

        fileInput.addEventListener('change', () => {
            if (fileInput.files.length > 0) {
                updateAudioLabel(fileInput.files);
            } else {
                // User cancelled — revert to previous mode or None
                const revert = this._audioFiles.length > 0 ? this._audioMode : 'none';
                this._audioMode = revert;
                this.overlay.querySelector(`input[name="ss-audio"][value="${revert}"]`).checked = true;
            }
        });

        folderInput.addEventListener('change', () => {
            const audioFiles = Array.from(folderInput.files).filter(f => f.type.startsWith('audio/'));
            if (audioFiles.length > 0) {
                updateAudioLabel(audioFiles);
            } else {
                const revert = this._audioFiles.length > 0 ? this._audioMode : 'none';
                this._audioMode = revert;
                this.overlay.querySelector(`input[name="ss-audio"][value="${revert}"]`).checked = true;
            }
        });

        // Start button
        this.overlay.querySelector('.ss-start-btn').addEventListener('click', () => this._onStart());
    }

    _getOptions() {
        const delay = parseInt(this.overlay.querySelector('.ss-delay-range').value, 10);
        const transition = this.overlay.querySelector('[data-transition].active')?.dataset.transition || 'fade';
        const display = this.overlay.querySelector('[data-display].active')?.dataset.display || 'single';
        const audioMode = this.overlay.querySelector('input[name="ss-audio"]:checked')?.value || 'none';
        return { delay, transition, display, audioMode, audioFiles: this._audioFiles };
    }

    _onStart() {
        const options = this._getOptions();
        // Persist audio selection for next session
        if (options.audioMode !== 'none' && this._audioFiles.length > 0) {
            localStorage.setItem('slideshow-audio-mode', options.audioMode);
            const label = this._audioFiles.length === 1
                ? this._audioFiles[0].name
                : `${this._audioFiles.length} files`;
            localStorage.setItem('slideshow-audio-label', label);
        }
        this.close();
        if (this.onStart) this.onStart(this._images, options);
    }
}
