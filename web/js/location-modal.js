// Location picker modal — lets the user set GPS coordinates on images

class LocationModal {
    constructor() {
        this.overlay = null;
        this.map = null;
        this.marker = null;
        this.files = [];
        this.lat = null;
        this.lon = null;
        this._onKeyDown = (e) => {
            if (e.key === 'Escape') this.close();
        };
    }

    open(files) {
        this.files = files;
        this.lat = null;
        this.lon = null;
        this._buildDOM();
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);
        this._initMap();
    }

    close() {
        document.removeEventListener('keydown', this._onKeyDown);
        if (this.map) {
            this.map.remove();
            this.map = null;
        }
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
        }
        this.marker = null;
    }

    _buildDOM() {
        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        this.overlay.innerHTML = `
            <div class="modal">
                <div class="modal-header">
                    <span class="modal-title">Set Location</span>
                    <button class="info-collapse-btn modal-close-btn">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="location-map" id="location-map"></div>
                    <div class="location-fields">
                        <label class="location-field">
                            <span class="view-menu-label">Latitude</span>
                            <input type="text" class="location-input" id="loc-lat" placeholder="e.g. 48.8566">
                        </label>
                        <label class="location-field">
                            <span class="view-menu-label">Longitude</span>
                            <input type="text" class="location-input" id="loc-lon" placeholder="e.g. 2.3522">
                        </label>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn" id="loc-cancel">Cancel</button>
                    <button class="btn btn-accent" id="loc-confirm" disabled>Set Location</button>
                </div>
            </div>`;

        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('#loc-cancel').addEventListener('click', () => this.close());
        this.overlay.querySelector('#loc-confirm').addEventListener('click', () => this._showConfirmation());

        const latInput = this.overlay.querySelector('#loc-lat');
        const lonInput = this.overlay.querySelector('#loc-lon');
        const updateFromInputs = () => {
            const lat = parseFloat(latInput.value);
            const lon = parseFloat(lonInput.value);
            if (!isNaN(lat) && !isNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180) {
                this.lat = lat;
                this.lon = lon;
                this._updateMarker();
                this.overlay.querySelector('#loc-confirm').disabled = false;
            } else {
                this.overlay.querySelector('#loc-confirm').disabled = true;
            }
        };
        latInput.addEventListener('input', updateFromInputs);
        lonInput.addEventListener('input', updateFromInputs);
    }

    _initMap() {
        if (typeof maplibregl === 'undefined') return;
        const mapEl = this.overlay.querySelector('#location-map');
        if (!mapEl) return;

        this.map = new maplibregl.Map({
            container: mapEl,
            style: 'https://tiles.openfreemap.org/styles/liberty',
            center: [0, 20],
            zoom: 2,
            attributionControl: false,
        });

        this.map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');

        this.map.on('click', (e) => {
            this.lat = Math.round(e.lngLat.lat * 1000000) / 1000000;
            this.lon = Math.round(e.lngLat.lng * 1000000) / 1000000;
            this._updateMarker();
            this.overlay.querySelector('#loc-lat').value = this.lat;
            this.overlay.querySelector('#loc-lon').value = this.lon;
            this.overlay.querySelector('#loc-confirm').disabled = false;
        });
    }

    _updateMarker() {
        if (!this.map || this.lat === null || this.lon === null) return;
        if (this.marker) {
            this.marker.setLngLat([this.lon, this.lat]);
        } else {
            this.marker = new maplibregl.Marker({ color: '#d35400' })
                .setLngLat([this.lon, this.lat])
                .addTo(this.map);
        }
        this.map.flyTo({ center: [this.lon, this.lat], speed: 1.5 });
    }

    _showConfirmation() {
        const body = this.overlay.querySelector('.modal-body');
        const footer = this.overlay.querySelector('.modal-footer');

        body.innerHTML = `
            <div class="location-confirm-msg">
                <p>Location data will be set on <strong>${this.files.length} image${this.files.length !== 1 ? 's' : ''}</strong>. Existing GPS coordinates will be overwritten.</p>
                <div class="location-confirm-coords">
                    <span class="info-label">Latitude</span> <span class="info-value">${this.lat}</span><br>
                    <span class="info-label">Longitude</span> <span class="info-value">${this.lon}</span>
                </div>
            </div>`;

        footer.innerHTML = `
            <button class="btn" id="loc-back">Back</button>
            <button class="btn btn-accent" id="loc-do-it">Confirm</button>`;

        footer.querySelector('#loc-back').addEventListener('click', () => {
            this.close();
            this.open(this.files);
        });
        footer.querySelector('#loc-do-it').addEventListener('click', () => this._execute());
    }

    async _execute() {
        const footer = this.overlay.querySelector('.modal-footer');
        footer.innerHTML = '<span class="info-label">Setting location...</span>';

        try {
            const result = await API.setLocation(this.files, this.lat, this.lon);
            const successes = result.results.filter(r => r.success).length;
            const failures = result.results.filter(r => !r.success);
            let msg = `Location set on ${successes} of ${this.files.length} image${this.files.length !== 1 ? 's' : ''}.`;
            if (failures.length > 0) {
                msg += '\n\nErrors:\n' + failures.map(f => `${f.file}: ${f.error}`).join('\n');
            }
            alert(msg);
        } catch (err) {
            alert('Failed to set location: ' + err.message);
        }
        this.close();
    }
}
