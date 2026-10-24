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
                title: t('Deleting files'),
                done: t('Deleted'),
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
        // Failures belong on the screen that caused them, not in an alert box
        // the user has to dismiss before seeing what is left.
        this._lastFailures = failures;
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
            containerEl.innerHTML = `
                <div class="browse-container">
                    ${placeHeaderHTML({ title: t('Marked for deletion') })}
                    <div class="wastebin-empty-box">
                        <strong>${t('Nothing marked')}</strong>
                        <span>${t('Select photos in Folders or a library and press Backspace to mark them. They stay on disk until you delete them here.')}</span>
                        <div><button class="btn" id="wb-go-folders">${t('Go to Folders')}</button></div>
                    </div>
                </div>`;
            containerEl.querySelector('#wb-go-folders').addEventListener('click', () => App.setMode('browse'));
            return;
        }

        const selectedCount = this.selected.size;
        const deleteCount = selectedCount || items.length;
        // Say what marking means, because "marked" is not "deleted" — nothing
        // leaves the disk until it is deleted here.
        const header = `
            ${placeHeaderHTML({
                title: t('Marked for deletion'),
                meta: markedCount(items),
                actions: `<div class="wastebin-actions" id="wb-actions">
                    <button class="btn btn-sm" id="wb-restore">${selectedCount > 0 ? t('Restore {n}', { n: formatCount(selectedCount) }) : t('Restore all')}</button>
                    <button class="btn btn-sm btn-danger" id="wb-delete">${t('Delete {n} permanently…', { n: deleteCount })}</button>
                </div>`,
                lede: '<p class="place-lede">' + t('These photos are hidden from Folders and libraries. Nothing is removed from disk until you delete them here.') + '</p>',
            })}
            <div class="browse-bar"><span class="browse-bar-spacer"></span><div class="wastebin-select"></div></div>
            ${(this._lastFailures && this._lastFailures.length) ? `
            <div class="wastebin-failures">
                <strong>${this._lastFailures.length === 1 ? t('1 file could not be deleted') : t('{n} files could not be deleted', { n: formatCount(this._lastFailures.length) })}</strong>
                <ul>${this._lastFailures.map(f => `<li>${f}</li>`).join('')}</ul>
            </div>` : ''}`;
        const actions = '';

        const gridItems = items.map(([path, entry], idx) => {
            const selectedClass = this.selected.has(path) ? ' selected' : '';
            const label = entry.label || entry.name;
            // A folder has no thumbnail, and a broken image icon would look
            // like a defect rather than a folder waiting to be deleted.
            if (entry.type === 'dir') {
                return `<div class="grid-item image-item wastebin-dir${selectedClass}" data-index="${idx}" data-path="${escapeHtml(path)}" data-type="image">
                    <div class="wastebin-dir-tile">
                        <svg width="40" height="34" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.25" aria-hidden="true"><path d="M1.5 4.5a1 1 0 0 1 1-1h3.5l1.5 1.5h6a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1z"/></svg>
                    </div>
                    <div class="item-name">${escapeHtml(label)}</div>
                </div>`;
            }
            const thumbSrc = (entry.libID && entry.photoID)
                ? LibraryAPI.thumbURL(entry.libID, entry.photoID)
                : API.thumbnailURL(path, 200);
            return `<div class="grid-item image-item${selectedClass}" data-index="${idx}" data-path="${escapeHtml(path)}" data-type="image">
                <img src="${thumbSrc}" alt="${escapeHtml(label)}" loading="lazy" onload="this.classList.add('img-loaded')">
                <div class="item-name">${escapeHtml(label)}</div>
            </div>`;
        });

        containerEl.innerHTML = `<div class="browse-container"><div class="browse-header">${header}${actions}</div><div class="browse-content"><div class="grid">${gridItems.join('')}</div></div></div>`;

        const select = new SelectAll({
            onAll: () => { this.selectAll(); this.render(containerEl, onRefresh); },
            onNone: () => { this.selected.clear(); this.render(containerEl, onRefresh); },
        });
        select.update(selectedCount, items.length);
        containerEl.querySelector('.wastebin-select').appendChild(select.el);

        containerEl.querySelector('#wb-restore').addEventListener('click', () => {
            const paths = this.selected.size > 0 ? new Set(this.selected) : new Set(items.map(([path]) => path));
            this.restore(paths);
            this.selected.clear();
            this.render(containerEl, onRefresh);
        });

        // Deleting from disk confirms in place, naming the count, rather than
        // through a browser confirm() box.
        containerEl.querySelector('#wb-delete').addEventListener('click', () => {
            const paths = this.selected.size > 0 ? new Set(this.selected) : new Set(items.map(([path]) => path));
            const count = paths.size;
            const dirCount = [...paths].filter(p => this._items.get(p)?.type === 'dir').length;
            const what = describeDeletion(count, dirCount);
            const actionsEl = containerEl.querySelector('#wb-actions');
            actionsEl.innerHTML = `
                <span class="wastebin-question">${t('Delete {what} from disk?', { what })}${dirCount ? ' ' + t('A folder goes with everything inside it.') : ''} ${t('This can’t be undone: Unterlumen keeps no copy, and only a backup restores what is deleted.')}</span>
                <button class="btn btn-sm" id="wb-delete-cancel">${t('Cancel')}</button>
                <button class="btn btn-sm btn-danger" id="wb-delete-confirm">${t('Delete {what}', { what })}</button>`;
            actionsEl.querySelector('#wb-delete-cancel').addEventListener('click', () => this.render(containerEl, onRefresh));
            actionsEl.querySelector('#wb-delete-confirm').addEventListener('click', async () => {
                const afterDelete = () => {
                    this.selected.clear();
                    this.render(containerEl, onRefresh);
                };
                await this.permanentlyDelete(paths, onRefresh, afterDelete);
            });
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


// markedCount says what is marked, as counts: photos, and folders when
// there are any.
function markedCount(items) {
    const dirs = items.filter(([, e]) => e.type === 'dir').length;
    const files = items.length - dirs;
    return [files ? countHTML(files, 'photo') : '', dirs ? countHTML(dirs, 'folder') : ''].filter(Boolean).join(' and ');
}

// "3 photos and 1 folder" — the question has to say what it is about to do,
// because a folder takes its contents with it.
function describeDeletion(total, dirs) {
    const files = total - dirs;
    const parts = [];
    if (files) parts.push(`${files} photo${files !== 1 ? 's' : ''}`);
    if (dirs) parts.push(`${dirs} folder${dirs !== 1 ? 's' : ''}`);
    return parts.join(' and ') || `${total} items`;
}
