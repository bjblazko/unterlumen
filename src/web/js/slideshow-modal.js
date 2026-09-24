// SlideshowModal — options dialog for the slideshow player.
// Follows the same class pattern as ExportModal and BatchRenameModal.

class SlideshowModal {
    constructor() {
        this.overlay = null;
        this.onStart = null;
        this._images = [];
        this._audioFiles = [];  // persists within session — not reset on re-open
        this._audioMode = 'none';
    }

    open(images) {
        if (this.overlay) this.close();
        this._images = images;
        this._buildDOM();
    }

    close() {
        this._dialog?.close(null);
        this.overlay = null;
    }

    _buildDOM() {
        const count = this._images.length;
        const label = count === 1 ? '1 photo' : `${count} photos`;

        this._dialog = new Dialog({
            title: 'Slideshow',
            subtitle: label,
            size: 'md',
            className: 'slideshow-dialog',
            body: `
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Delay</span>
                        <div class="ss-delay-row">
                            <input type="range" class="ss-delay-range" min="1" max="60" value="5">
                            <span class="ss-delay-value">5 s</span>
                        </div>
                    </div>
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Transition</span>
                        <div class="seg" role="group" aria-label="Transition">
                            <button aria-pressed="true" data-transition="fade">Fade</button>
                            <button aria-pressed="false" data-transition="slide">Slide</button>
                            <button aria-pressed="false" data-transition="zoom">Zoom</button>
                            <button aria-pressed="false" data-transition="instant">Instant</button>
                        </div>
                    </div>
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Display</span>
                        <div class="seg" role="group" aria-label="Display">
                            <button aria-pressed="true" data-display="single">Single</button>
                            <button aria-pressed="false" data-display="kenburns">Ken Burns</button>
                            <button aria-pressed="false" data-display="2up">2-up</button>
                            <button aria-pressed="false" data-display="4up">4-up</button>
                        </div>
                    </div>
                    <div class="ss-opt-section">
                        <span class="ss-opt-label">Loop</span>
                        <div class="ss-loop-wrap"></div>
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
                            <label class="export-radio-row">
                                <input type="radio" name="ss-audio" value="builtin"> Built-in
                            </label>
                            <div class="ss-builtin-track" style="display:none">
                                <label class="ss-track-check">
                                    <input type="checkbox" class="ss-track-cb" value="After_Hours_Transit.mp3" checked> After Hours Transit (Electro-Pop)
                                </label>
                                <label class="ss-track-check">
                                    <input type="checkbox" class="ss-track-cb" value="Mahogany_Gravity.mp3" checked> Mahogany Gravity (Classic)
                                </label>
                                <label class="ss-track-check">
                                    <input type="checkbox" class="ss-track-cb" value="Sunlight_Through_Leaves.mp3" checked> Sunlight through the leaves (Piano Pop)
                                </label>
                                <div class="ss-track-order">
                                    <label class="ss-track-order-label"><input type="radio" name="ss-track-order" value="inorder" checked> In order</label>
                                    <label class="ss-track-order-label"><input type="radio" name="ss-track-order" value="shuffle"> Shuffled</label>
                                </div>
                                <span class="ss-track-none-error" style="display:none">Select at least one track.</span>
                            </div>
                            <div class="ss-audio-file-label"></div>
                        </div>
                    </div>
                    <input type="file" class="ss-file-input" accept="audio/*" style="display:none">
                    <input type="file" class="ss-folder-input" accept="audio/*" style="display:none" webkitdirectory>
`,
            actions: [
                { label: 'Cancel', onClick: () => this.close() },
                { label: 'Start slideshow', kind: 'primary', onClick: () => this._onStart() },
            ],
            onClose: () => { this.overlay = null; },
        });
        this.overlay = this._dialog.open();

        // Delay range
        const range = this.overlay.querySelector('.ss-delay-range');
        const delayLabel = this.overlay.querySelector('.ss-delay-value');
        range.addEventListener('input', () => {
            delayLabel.textContent = `${range.value} s`;
        });

        // Segmented toggles (transition + display)
        this.overlay.querySelectorAll('.seg').forEach(group => {
            group.querySelectorAll('button').forEach(btn => {
                btn.addEventListener('click', () => {
                    group.querySelectorAll('button').forEach(b => b.setAttribute('aria-pressed', 'false'));
                    btn.setAttribute('aria-pressed', 'true');
                });
            });
        });

        // Audio radios + file pickers
        const fileInput = this.overlay.querySelector('.ss-file-input');
        const folderInput = this.overlay.querySelector('.ss-folder-input');
        const audioLabel = this.overlay.querySelector('.ss-audio-file-label');
        const builtinTrackContainer = this.overlay.querySelector('.ss-builtin-track');

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

        // Restore loop preference
        const savedLoop = localStorage.getItem('slideshow-loop') !== 'false';
        this._loopToggle = Toggle.create(this.overlay.querySelector('.ss-loop-wrap'), {
            initial: savedLoop,
            onChange: (on) => { localStorage.setItem('slideshow-loop', on); }
        });

        // Restore state from current session or localStorage
        const restoredMode = this._audioMode === 'builtin' || (this._audioMode !== 'none' && this._audioFiles.length > 0)
            ? this._audioMode
            : localStorage.getItem('slideshow-audio-mode') || 'none';
        const restoredRadio = this.overlay.querySelector(`input[name="ss-audio"][value="${restoredMode}"]`);
        if (restoredRadio) restoredRadio.checked = true;

        if (restoredMode === 'builtin') {
            builtinTrackContainer.style.display = '';
            const savedTracks = JSON.parse(localStorage.getItem('slideshow-builtin-tracks') || 'null');
            if (savedTracks) {
                builtinTrackContainer.querySelectorAll('.ss-track-cb').forEach(cb => {
                    cb.checked = savedTracks.includes(cb.value);
                });
            }
            if (localStorage.getItem('slideshow-builtin-shuffle') === 'true') {
                builtinTrackContainer.querySelector('input[value="shuffle"]').checked = true;
            }
        } else if (this._audioFiles.length > 0) {
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
                builtinTrackContainer.style.display = radio.value === 'builtin' ? '' : 'none';
                if (radio.value === 'file') {
                    fileInput.click();
                } else if (radio.value === 'folder') {
                    folderInput.click();
                } else if (radio.value === 'none') {
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

        // Hide track-selection error on any checkbox change
        builtinTrackContainer.querySelectorAll('.ss-track-cb').forEach(cb => {
            cb.addEventListener('change', () => {
                builtinTrackContainer.querySelector('.ss-track-none-error').style.display = 'none';
            });
        });

        // Start button
    }

    _getOptions() {
        const delay = parseInt(this.overlay.querySelector('.ss-delay-range').value, 10);
        const transition = this.overlay.querySelector('[data-transition][aria-pressed="true"]')?.dataset.transition || 'fade';
        const display = this.overlay.querySelector('[data-display][aria-pressed="true"]')?.dataset.display || 'single';
        const audioMode = this.overlay.querySelector('input[name="ss-audio"]:checked')?.value || 'none';
        const loop = this._loopToggle.state();
        const builtinTracks = audioMode === 'builtin'
            ? [...this.overlay.querySelectorAll('.ss-track-cb:checked')].map(cb => cb.value)
            : [];
        const builtinShuffle = audioMode === 'builtin' &&
            this.overlay.querySelector('input[name="ss-track-order"]:checked')?.value === 'shuffle';
        return { delay, transition, display, loop, audioMode, audioFiles: this._audioFiles, builtinTracks, builtinShuffle };
    }

    _onStart() {
        const options = this._getOptions();
        if (options.audioMode === 'builtin' && options.builtinTracks.length === 0) {
            this.overlay.querySelector('.ss-track-none-error').style.display = '';
            return;
        }
        // Persist audio selection for next session
        if (options.audioMode === 'builtin') {
            localStorage.setItem('slideshow-audio-mode', 'builtin');
            localStorage.setItem('slideshow-audio-label', 'Built-in');
            localStorage.setItem('slideshow-builtin-tracks', JSON.stringify(options.builtinTracks));
            localStorage.setItem('slideshow-builtin-shuffle', options.builtinShuffle ? 'true' : 'false');
        } else if (options.audioMode !== 'none' && this._audioFiles.length > 0) {
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
