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

    openRemove(files, onSuccess = null) {
        this.files = files;
        this._onSuccess = onSuccess;
        this._buildRemoveDOM();
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);
    }

    _buildRemoveDOM() {
        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        const n = this.files.length;
        this.overlay.innerHTML = `
            <div class="modal">
                <div class="modal-header">
                    <span class="modal-title">Remove Geolocation</span>
                    <button class="info-collapse-btn modal-close-btn">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="location-confirm-msg">
                        <p>GPS data will be removed from <strong>${n} image${n !== 1 ? 's' : ''}</strong>. This cannot be undone.</p>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn" id="loc-remove-cancel">Cancel</button>
                    <button class="btn btn-accent" id="loc-remove-confirm">Remove</button>
                </div>
            </div>`;

        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('#loc-remove-cancel').addEventListener('click', () => this.close());
        this.overlay.querySelector('#loc-remove-confirm').addEventListener('click', () => this._executeRemove());
    }

    async _executeRemove() {
        const footer = this.overlay.querySelector('.modal-footer');
        footer.innerHTML = '<span class="info-label">Removing GPS data...</span>';

        try {
            const result = await API.removeLocation(this.files);
            const successes = result.results.filter(r => r.success).length;
            const failures = result.results.filter(r => !r.success);
            let msg = `GPS data removed from ${successes} of ${this.files.length} image${this.files.length !== 1 ? 's' : ''}.`;
            const successFiles = result.results.filter(r => r.success).map(r => r.file);
            if (failures.length > 0) {
                msg += ' Some files failed.';
                footer.innerHTML = `<span class="info-label" style="color:#c0392b">${msg}</span>`;
                if (successFiles.length > 0 && this._onSuccess) this._onSuccess(successFiles);
            } else {
                footer.innerHTML = `<span class="info-label">${msg}</span>`;
                if (this._onSuccess) this._onSuccess(successFiles);
                setTimeout(() => this.close(), 1200);
            }
        } catch (err) {
            footer.innerHTML = `<span class="info-label" style="color:#c0392b">Failed: ${err.message}</span>`;
        }
    }

    open(files, onSuccess = null) {
        this.files = files;
        this._onSuccess = onSuccess;
        this.lat = null;
        this.lon = null;
        this._buildDOM();
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);
        this._initMap();
        this._loadInitialPosition(files);
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

        let initialCenter = [0, 20];
        let initialZoom = 2;
        const stored = localStorage.getItem('user-location');
        if (stored) {
            try {
                const { lat, lon } = JSON.parse(stored);
                initialCenter = [lon, lat];
                initialZoom = 9;
            } catch (e) { /* ignore */ }
        }

        this.map = new maplibregl.Map({
            container: mapEl,
            style: 'https://tiles.openfreemap.org/styles/liberty',
            center: initialCenter,
            zoom: initialZoom,
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
        const doUpdate = () => {
            if (this.marker) {
                this.marker.setLngLat([this.lon, this.lat]);
            } else {
                this.marker = new maplibregl.Marker({ color: '#d35400' })
                    .setLngLat([this.lon, this.lat])
                    .addTo(this.map);
            }
            this.map.flyTo({ center: [this.lon, this.lat], speed: 1.5 });
        };
        if (this.map.loaded()) doUpdate(); else this.map.once('load', doUpdate);
    }

    async _loadInitialPosition(files) {
        if (!this.map) return;

        // 1. Single file: try to get existing GPS from EXIF
        if (files.length === 1) {
            try {
                const info = await API.info(files[0]);
                if (info && info.exif && info.exif.latitude != null && info.exif.longitude != null) {
                    if (!this.overlay) return; // modal closed while loading
                    this.lat = info.exif.latitude;
                    this.lon = info.exif.longitude;
                    this.overlay.querySelector('#loc-lat').value = this.lat;
                    this.overlay.querySelector('#loc-lon').value = this.lon;
                    this.overlay.querySelector('#loc-confirm').disabled = false;
                    const flyTo = () => this.map.flyTo({ center: [this.lon, this.lat], zoom: 14, speed: 1.5 });
                    if (this.map.loaded()) flyTo(); else this.map.once('load', flyTo);
                    this._updateMarker();
                    return; // skip geolocation when GPS exists
                }
            } catch (e) { /* ignore */ }
        }

        // 2. Geolocation — only if no stored location
        if (!localStorage.getItem('user-location') && navigator.geolocation) {
            navigator.geolocation.getCurrentPosition((pos) => {
                const lat = pos.coords.latitude;
                const lon = pos.coords.longitude;
                localStorage.setItem('user-location', JSON.stringify({ lat, lon }));
                if (!this.map || !this.overlay) return;
                const flyTo = () => this.map.flyTo({ center: [lon, lat], zoom: 9, speed: 1.5 });
                if (this.map.loaded()) flyTo(); else this.map.once('load', flyTo);
            }, () => { /* denied or unavailable — world view already set */ });
        }
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
            const successFiles = result.results.filter(r => r.success).map(r => r.file);
            let msg = `Location set on ${successes} of ${this.files.length} image${this.files.length !== 1 ? 's' : ''}.`;
            if (failures.length > 0) {
                msg += ' Some files failed.';
                footer.innerHTML = `<span class="info-label" style="color:#c0392b">${msg}</span>`;
                if (successFiles.length > 0 && this._onSuccess) this._onSuccess(successFiles);
            } else {
                footer.innerHTML = `<span class="info-label">${msg}</span>`;
                if (this._onSuccess) this._onSuccess(successFiles);
                setTimeout(() => this.close(), 1200);
            }
        } catch (err) {
            footer.innerHTML = `<span class="info-label" style="color:#c0392b">Failed: ${err.message}</span>`;
        }
    }
}
