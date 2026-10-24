// face-mark-tool.js — marking a face that was not found, in the viewer
// (ADR-0051): a frame drawn on the photo as the crop tool draws one, then
// the name of who it is in a field below the frame. The faces already known
// show as quiet frames, so none is marked twice.

class FaceMarkTool {
    // faces are the photo's known faces (x, y, w, h in fractions, name);
    // names the names to offer; onSave({ x, y, w, h, name }) saves the face
    // and throws an Error whose message says why it could not.
    constructor(img, { faces, names, onSave }) {
        this._onSave = onSave;
        this._crop = new CropTool(img, { grid: false, onDraw: () => this._place() });
        this._crop._overlay.classList.add('keyboard-owner', 'face-mark-overlay');
        this._layer = this._buildLayer(faces, names);
        img.parentNode.appendChild(this._layer);
        this._input = this._layer.querySelector('.face-mark-input');
        this._error = this._layer.querySelector('.face-mark-error');
        this._input.addEventListener('input', () => { this._error.hidden = true; });
        this._onMouseUp = () => { if (this._crop.getRect()) this._input.focus(); };
        document.addEventListener('mouseup', this._onMouseUp);
        this._place();
    }

    hasFrame() {
        return !!this._crop.getRect();
    }

    // save hands the frame and the name over; false when either is missing.
    async save() {
        const r = this._crop.getRect();
        const name = this._input.value.trim();
        if (!r || !name) {
            this._showError(r ? t('Type who it is.') : t('Drag a frame around the face first.'));
            return false;
        }
        this._input.disabled = true;
        try {
            await this._onSave({ x: r.x, y: r.y, w: r.width, h: r.height, name });
            return true;
        } catch (err) {
            this._input.disabled = false;
            this._input.focus();
            this._showError(err.message);
            return false;
        }
    }

    destroy() {
        document.removeEventListener('mouseup', this._onMouseUp);
        this._crop.destroy();
        this._layer.remove();
    }

    _buildLayer(faces, names) {
        const layer = document.createElement('div');
        layer.className = 'face-mark-layer';
        const pct = FacePeek.pct;
        const known = faces.map(f => `<span class="face-mark-known" style="left:${pct(f.x)};top:${pct(f.y)};width:${pct(f.w)};height:${pct(f.h)}">` +
            (f.name ? `<span class="face-mark-known-name">${escapeHtml(f.name)}</span>` : '') + '</span>').join('');
        layer.innerHTML = `${known}
            <div class="face-mark-name" hidden>
                <input class="face-mark-input" type="text" list="face-mark-names" autocomplete="off" aria-label="${t('Who is this?')}" placeholder="">
                <datalist id="face-mark-names">${names.map(n => `<option value="${escapeHtml(n)}">`).join('')}</datalist>
                <p class="face-mark-error" role="alert" hidden></p>
            </div>`;
        return layer;
    }

    _showError(text) {
        this._error.textContent = text;
        this._error.hidden = false;
    }

    // The layer lies over the photo like the crop overlay; the name field
    // sits below the frame, or above it near the photo's foot.
    _place() {
        if (!this._layer) return;
        const area = this._crop._imgRect();
        Object.assign(this._layer.style, {
            left: `${area.left}px`, top: `${area.top}px`, width: `${area.width}px`, height: `${area.height}px`,
        });
        const field = this._layer.querySelector('.face-mark-name');
        const r = this._crop.getRect();
        field.hidden = !r;
        if (!r) return;
        const below = (r.y + r.height) * area.height + 8;
        const room = area.height - below > 80;
        field.style.left = `${r.x * area.width}px`;
        field.style.top = room ? `${below}px` : '';
        field.style.bottom = room ? '' : `${(1 - r.y) * area.height + 8}px`;
    }
}
