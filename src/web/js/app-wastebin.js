// Wastebin — tracks files marked for deletion and owns the review UI

class Wastebin {
    constructor() {
        this._items = new Map();
        this.selected = new Set();
        this._lastClickedIndex = -1;
    }

    get size() { return this._items.size; }

    isMarked(path) { return this._items.has(path); }

    mark(selectedPaths, entries, currentDir, photoMeta = null) {
        for (const path of selectedPaths) {
            if (this._items.has(path)) continue;
            const entry = entries.find(e => {
                const fp = currentDir ? `${currentDir}/${e.name}` : e.name;
                return fp === path;
            });
            if (entry) {
                const meta = photoMeta ? photoMeta[path] : null;
                this._items.set(path, {
                    name: entry.name,
                    label: entry.label || entry.name,
                    type: entry.type,
                    date: entry.date,
                    size: entry.size,
                    dir: currentDir,
                    ...(meta || {}),
                });
            }
        }
        this._updateBadge();
    }

    restore(paths) {
        for (const path of paths) this._items.delete(path);
        this._updateBadge();
    }

    selectAll() {
        this._items.forEach((_, p) => this.selected.add(p));
    }

    async permanentlyDelete(paths, onRefresh, afterDelete) {
        const filePaths = Array.from(paths);

        const deleteOne = async (file) => {
            const item = this._items.get(file);
            if (item?.libID && item?.photoID) {
                return LibraryAPI.deletePhoto(item.libID, item.photoID);
            }
            const result = await API.delete([file]);
            return result.results[0];
        };

        if (filePaths.length > 5) {
            const dialog = new ProgressDialog();
            dialog.open(filePaths, {
                verb: 'Deleting',
                action: async (file) => {
                    try {
                        const r = await deleteOne(file);
                        if (r && (r.success || (r.error && r.error.includes('no such file')))) {
                            this._items.delete(file);
                        }
                        return r || { success: true };
                    } catch (err) {
                        return { success: false, error: err.message };
                    }
                },
                onComplete: () => {
                    this._updateBadge();
                    if (onRefresh) onRefresh();
                    if (afterDelete) afterDelete();
                },
            });
            return;
        }

        const failures = [];
        for (const file of filePaths) {
            try {
                const r = await deleteOne(file);
                if (r && (r.success || (r.error && r.error.includes('no such file')))) {
                    this._items.delete(file);
                } else if (r) {
                    failures.push(`${file}: ${r.error}`);
                }
            } catch (err) {
                failures.push(`${file}: ${err.message}`);
            }
        }
        this._updateBadge();
        if (onRefresh) onRefresh();
        if (afterDelete) afterDelete();
        if (failures.length > 0) {
            alert(`Delete: ${failures.length} error(s):\n${failures.join('\n')}`);
        }
    }

    _updateBadge() {
        const countEl = document.getElementById('wastebin-count');
        if (!countEl) return;
        const count = this._items.size;
        countEl.textContent = count > 0 ? count : '';
        countEl.style.display = count > 0 ? 'inline' : 'none';
    }

    render(containerEl, onRefresh) {
        this._lastClickedIndex = -1;
        const items = Array.from(this._items.entries());

        if (items.length === 0) {
            containerEl.innerHTML = '<div class="browse-container"><div class="wastebin-empty">No photos marked yet for deletion. Use the "Select" or "Organize view to do so."</div></div>';
            return;
        }

        const header = `<div class="wastebin-header">${items.length} file${items.length !== 1 ? 's' : ''} marked for deletion</div>`;
        const selectedCount = this.selected.size;

        const actions = `<div class="wastebin-actions">
            <button class="btn btn-action" id="wb-restore" ${selectedCount === 0 ? 'disabled' : ''}>Restore${selectedCount > 0 ? ` (${selectedCount})` : ''}</button>
            <button class="btn btn-action btn-danger" id="wb-delete" ${selectedCount === 0 ? 'disabled' : ''}>Delete permanently${selectedCount > 0 ? ` (${selectedCount})` : ''}</button>
        </div>`;

        const gridItems = items.map(([path, entry], idx) => {
            const selectedClass = this.selected.has(path) ? ' selected' : '';
            const thumbSrc = (entry.libID && entry.photoID)
                ? LibraryAPI.thumbURL(entry.libID, entry.photoID)
                : API.thumbnailURL(path, 200);
            const label = entry.label || entry.name;
            return `<div class="grid-item image-item${selectedClass}" data-index="${idx}" data-path="${path}" data-type="image">
                <img src="${thumbSrc}" alt="${label}" loading="lazy" onload="this.classList.add('img-loaded')">
                <div class="item-name">${label}</div>
            </div>`;
        });

        containerEl.innerHTML = `<div class="browse-container"><div class="browse-header">${header}${actions}</div><div class="browse-content"><div class="grid">${gridItems.join('')}</div></div></div>`;

        document.getElementById('wb-restore').addEventListener('click', () => {
            this.restore(this.selected);
            this.selected.clear();
            this.render(containerEl, onRefresh);
        });

        document.getElementById('wb-delete').addEventListener('click', async () => {
            const count = this.selected.size;
            if (!confirm(`Permanently delete ${count} file${count !== 1 ? 's' : ''}? This cannot be undone.`)) return;
            const afterDelete = () => {
                this.selected.clear();
                this.render(containerEl, onRefresh);
            };
            await this.permanentlyDelete(this.selected, onRefresh, afterDelete);
        });

        containerEl.querySelectorAll('[data-type="image"]').forEach(el => {
            el.addEventListener('click', (e) => {
                const path = el.dataset.path;
                const idx = parseInt(el.dataset.index);

                if (e.ctrlKey || e.metaKey) {
                    if (this.selected.has(path)) {
                        this.selected.delete(path);
                    } else {
                        this.selected.add(path);
                    }
                    this._lastClickedIndex = idx;
                } else if (e.shiftKey && this._lastClickedIndex >= 0) {
                    const start = Math.min(this._lastClickedIndex, idx);
                    const end = Math.max(this._lastClickedIndex, idx);
                    for (let i = start; i <= end; i++) {
                        this.selected.add(items[i][0]);
                    }
                } else {
                    this.selected.clear();
                    this.selected.add(path);
                    this._lastClickedIndex = idx;
                }

                this.render(containerEl, onRefresh);
            });
        });
    }
}
