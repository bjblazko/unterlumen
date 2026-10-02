// Location picker modal — lets the user set GPS coordinates on images

class LocationModal {
    constructor() {
        this.overlay = null;
        this.map = null;
        this.marker = null;
        this.files = [];
        this.lat = null;
        this.lon = null;
    }

    // Taking a location away is destructive, so it asks first — in the dialog
    // it was started from, naming what it will strip.
    _confirmRemove() {
        const n = this.files.length;
        this._dialog.setBody(`
            <div class="location-confirm-msg">
                <p>Remove the location from <strong>${n} photo${n !== 1 ? 's' : ''}</strong>? The GPS coordinates are deleted from the files; nothing else changes.</p>
            </div>`);
        this._dialog.setActions([
            { label: 'Keep it', onClick: () => { const files = this.files; const cb = this._onSuccess; this.close(); this.open(files, cb); } },
            { label: `Remove from ${n} photo${n !== 1 ? 's' : ''}`, kind: 'danger', onClick: () => this._executeRemove() },
        ]);
    }

    async _executeRemove() {
        // Large batches: use progress dialog
        if (this.files.length > 5) {
            this.close();
            const files = this.files;
            const onSuccess = this._onSuccess;
            const dialog = new ProgressDialog();
            const successFiles = [];
            dialog.open(files, {
                verb: 'Removing GPS',
                action: async (file) => {
                    try {
                        const result = await API.removeLocation([file]);
                        const r = result.results[0];
                        if (r && r.success) successFiles.push(r.file);
                        return r || { success: true };
                    } catch (err) {
                        return { success: false, error: err.message };
                    }
                },
                onComplete: () => {
                    if (successFiles.length > 0 && onSuccess) onSuccess(successFiles);
                },
            });
            return;
        }

        this._dialog.setActions([]);
        this._dialog.setNote('Removing the location…');

        try {
            const result = await API.removeLocation(this.files);
            const successes = result.results.filter(r => r.success).length;
            const failures = result.results.filter(r => !r.success);
            let msg = `GPS data removed from ${successes} of ${this.files.length} image${this.files.length !== 1 ? 's' : ''}.`;
            const successFiles = result.results.filter(r => r.success).map(r => r.file);
            if (failures.length > 0) {
                msg += ` ${failures.length} did not: ${failures.map(f => f.error).join(', ')}`;
                this._dialog.setNote(msg, 'error');
                this._dialog.setActions([{ label: 'Close', onClick: () => this.close() }]);
                if (successFiles.length > 0 && this._onSuccess) this._onSuccess(successFiles);
            } else {
                this._dialog.setNote(msg);
                if (this._onSuccess) this._onSuccess(successFiles);
                setTimeout(() => this.close(), 1200);
            }
        } catch (err) {
            this._dialog.setNote(`It did not work: ${err.message}`, 'error');
            this._dialog.setActions([{ label: 'Close', onClick: () => this.close() }]);
        }
    }

    open(files, onSuccess = null) {
        this.files = files;
        this._onSuccess = onSuccess;
        this.lat = null;
        this.lon = null;
        this._buildDOM();
        this._initMap();
        this._loadInitialPosition(files);
    }

    close() {
        if (this.map) {
            this.map.remove();
            this.map = null;
        }
        this._dialog?.close(null);
        this.overlay = null;
        this.marker = null;
    }

    _buildDOM() {
        // Everything about a photo's location is in this one dialog: setting
        // it, and taking it away again.
        this._dialog = new Dialog({
            title: 'Location',
            subtitle: `${this.files.length} photo${this.files.length !== 1 ? 's' : ''}`,
            size: 'md',
            className: 'location-dialog',
            body: `
                <div class="location-map" id="location-map"></div>
                <div class="location-fields">
                    <label class="location-field">
                        <span class="field-label-inline">Latitude</span>
                        <input type="text" class="location-input" id="loc-lat" placeholder="e.g. 48.8566">
                    </label>
                    <label class="location-field">
                        <span class="field-label-inline">Longitude</span>
                        <input type="text" class="location-input" id="loc-lon" placeholder="e.g. 2.3522">
                    </label>
                </div>`,
            actions: [
                { label: 'Remove location…', kind: 'danger', id: 'loc-remove', onClick: () => this._confirmRemove() },
                { label: 'Cancel', id: 'loc-cancel', onClick: () => this.close() },
                { label: 'Set location', kind: 'primary', id: 'loc-confirm', disabled: true, onClick: () => this._showConfirmation() },
            ],
            onClose: () => { this.overlay = null; },
        });
        this.overlay = this._dialog.open();

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
        // OpenStreetMap's licence (ODbL) asks for its credit on every map.
        this.map.addControl(new maplibregl.AttributionControl({ compact: true }));

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
                this.marker = new maplibregl.Marker({ color: getComputedStyle(document.documentElement).getPropertyValue('--accent').trim() || '#E85D04' })
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
        this._dialog.setBody(`
            <div class="location-confirm-msg">
                <p>Location data will be set on <strong>${this.files.length} image${this.files.length !== 1 ? 's' : ''}</strong>. Existing GPS coordinates will be overwritten.</p>
                <div class="location-confirm-coords">
                    <span class="info-label">Latitude</span> <span class="info-value">${this.lat}</span><br>
                    <span class="info-label">Longitude</span> <span class="info-value">${this.lon}</span>
                </div>
            </div>`);

        this._dialog.setActions([
            { label: 'Back', onClick: () => { const files = this.files; this.close(); this.open(files, this._onSuccess); } },
            { label: 'Set the location', kind: 'primary', onClick: () => this._execute() },
        ]);
    }

    async _execute() {
        // Large batches: use progress dialog
        if (this.files.length > 5) {
            const lat = this.lat;
            const lon = this.lon;
            const files = this.files;
            const onSuccess = this._onSuccess;
            this.close();
            const dialog = new ProgressDialog();
            const successFiles = [];
            dialog.open(files, {
                verb: 'Setting location',
                action: async (file) => {
                    try {
                        const result = await API.setLocation([file], lat, lon);
                        const r = result.results[0];
                        if (r && r.success) successFiles.push(r.file);
                        return r || { success: true };
                    } catch (err) {
                        return { success: false, error: err.message };
                    }
                },
                onComplete: () => {
                    if (successFiles.length > 0 && onSuccess) onSuccess(successFiles);
                },
            });
            return;
        }

        this._dialog.setActions([]);
        this._dialog.setNote('Setting the location…');

        try {
            const result = await API.setLocation(this.files, this.lat, this.lon);
            const successes = result.results.filter(r => r.success).length;
            const failures = result.results.filter(r => !r.success);
            const successFiles = result.results.filter(r => r.success).map(r => r.file);
            let msg = `Location set on ${successes} of ${this.files.length} image${this.files.length !== 1 ? 's' : ''}.`;
            if (failures.length > 0) {
                msg += ` ${failures.length} did not: ${failures.map(f => f.error).join(', ')}`;
                this._dialog.setNote(msg, 'error');
                this._dialog.setActions([{ label: 'Close', onClick: () => this.close() }]);
                if (successFiles.length > 0 && this._onSuccess) this._onSuccess(successFiles);
            } else {
                this._dialog.setNote(msg);
                if (this._onSuccess) this._onSuccess(successFiles);
                setTimeout(() => this.close(), 1200);
            }
        } catch (err) {
            this._dialog.setNote(`It did not work: ${err.message}`, 'error');
            this._dialog.setActions([{ label: 'Close', onClick: () => this.close() }]);
        }
    }
}
