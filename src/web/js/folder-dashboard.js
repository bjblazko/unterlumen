// FolderDashboard — the info panel's view of a folder: where it is, what it
// holds, a size map, how deep it nests, its file types and — in a library —
// what its photos were taken with and when. It renders into the InfoPanel
// that owns it and uses its sections and rows.

class FolderDashboard {
    constructor(panel) {
        this.panel = panel;
    }


    render(d) {
        if (d.photoCount !== undefined) return this._renderLibraryFolderData(d);

        const sections = [];

        // Folder section
        const folderRows = [];
        folderRows.push(this.panel.row('Name', d.name));
        folderRows.push(this.panel.row('Path', d.path || '/'));
        folderRows.push(this.panel.row('Modified', this.panel.formatDate(d.modified)));
        sections.push(this.panel.section('Folder', folderRows));

        // Contents section
        const contRows = [];
        contRows.push(this.panel.row('Total size', formatSize(d.totalSize)));
        contRows.push(this.panel.row('Files', d.fileCount.toLocaleString()));
        contRows.push(this.panel.row('Subfolders', d.dirCount.toLocaleString()));
        contRows.push(this.panel.row('Max depth', d.maxDepth + (d.maxDepth === 1 ? ' level' : ' levels')));
        sections.push(this.panel.section('Contents', contRows));

        // Size map (treemap)
        if (d.subfolders && d.subfolders.length > 0) {
            const treemap = this._renderTreemap(d.subfolders, d.totalSize);
            sections.push(this.panel.section('Size Map', [treemap]));
        }

        // Nesting depth histogram
        if (d.subfolders && d.subfolders.some(s => s.maxDepth > 0)) {
            sections.push(this.panel.section('Nesting Depth', [this._renderDepthHistogram(d.subfolders)]));
        }

        // File types (browse mode — no library stats)
        if (!this.panel.libraryStats && d.fileTypes && Object.keys(d.fileTypes).length > 0) {
            sections.push(this.panel.section('File Types', [this._renderFileTypeChart(d.fileTypes)]));
        }

        // Library EXIF stats
        if (this.panel.libraryStats) {
            sections.push(...this._renderLibraryStats(this.panel.libraryStats));
        }

        return sections.join('');
    }

    _renderLibraryFolderData(d) {
        const sections = [];

        // Folder section — derive name/path from currentPath since LibraryFolderStats has no FS metadata
        const folderRows = [];
        const pathStr = this.panel.currentPath || '';
        const name = pathStr ? pathStr.split('/').pop() : '(root)';
        folderRows.push(this.panel.row('Name', name));
        folderRows.push(this.panel.row('Path', pathStr || '/'));
        sections.push(this.panel.section('Folder', folderRows));

        // Contents section
        const contRows = [];
        contRows.push(this.panel.row('Total size', formatSize(d.totalSize)));
        contRows.push(this.panel.row('Photos', d.photoCount.toLocaleString()));
        if (d.dateFirst && d.dateLast) {
            const first = d.dateFirst.slice(0, 10);
            const last = d.dateLast.slice(0, 10);
            contRows.push(this.panel.row('Date range', first === last ? first : first + ' – ' + last));
        }
        sections.push(this.panel.section('Contents', contRows));

        // Size map treemap — adapt LibSubfolder to the shape _renderTreemap expects
        if (d.subfolders && d.subfolders.length > 0) {
            const adapted = d.subfolders.map(s => ({ name: s.name, size: s.totalSize, fileCount: s.photoCount }));
            sections.push(this.panel.section('Size Map', [this._renderTreemap(adapted, d.totalSize)]));
        }

        // File Types — only when libraryStats is not rendering formats already
        if (!this.panel.libraryStats && d.formats && d.formats.length > 0) {
            const fileTypes = Object.fromEntries(d.formats.map(f => [f.name, f.count]));
            sections.push(this.panel.section('File Types', [this._renderFileTypeChart(fileTypes)]));
        }

        // Library EXIF stats (cameras, focal lengths, shooting hours, etc.)
        if (this.panel.libraryStats) {
            sections.push(...this._renderLibraryStats(this.panel.libraryStats));
        }

        return sections.join('');
    }

    _renderTreemap(subfolders, totalSize) {
        const W = 260;
        const maxH = 200;
        const sorted = [...subfolders].sort((a, b) => b.size - a.size);
        const total = sorted.reduce((s, f) => s + f.size, 0) || 1;
        const H = Math.max(80, Math.min(maxH, Math.round(W * Math.min(total / (1024 * 1024 * 1024) + 0.5, 1))));

        const layout = this._squarify(sorted, 0, 0, W, H);
        // The size map is a chart, so it takes the chart ramp (ADR-0034)
        // rather than eight colours of its own.
        const read = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
        const colors = Array.from({ length: 8 }, (_, i) => read(`--chart-${i + 1}`));

        let rects = '';
        layout.forEach((cell, i) => {
            const color = colors[i % colors.length];
            const label = cell.item.name.length > 14 ? cell.item.name.slice(0, 13) + '…' : cell.item.name;
            const sizeLabel = formatSize(cell.item.size);
            const countLabel = cell.item.fileCount + ' file' + (cell.item.fileCount !== 1 ? 's' : '');
            const showText = cell.w > 40 && cell.h > 30;
            const showCount = cell.w > 60 && cell.h > 52;
            const subPath = (this.panel.currentPath ? this.panel.currentPath + '/' : '') + cell.item.name;

            const tooltip = `${cell.item.name}\n${sizeLabel} · ${countLabel}`;
            rects += `<g class="folder-treemap-cell" data-path="${escapeHtml(subPath)}" style="cursor:pointer">` +
                `<title>${escapeHtml(tooltip)}</title>` +
                `<rect x="${cell.x.toFixed(1)}" y="${cell.y.toFixed(1)}" width="${cell.w.toFixed(1)}" height="${cell.h.toFixed(1)}" fill="${color}" rx="2"/>` +
                (showText ? `<text x="${(cell.x + 6).toFixed(1)}" y="${(cell.y + 16).toFixed(1)}" class="folder-treemap-name">${escapeHtml(label)}</text>` : '') +
                (showText ? `<text x="${(cell.x + 6).toFixed(1)}" y="${(cell.y + 30).toFixed(1)}" class="folder-treemap-size">${escapeHtml(sizeLabel)}</text>` : '') +
                (showCount ? `<text x="${(cell.x + 6).toFixed(1)}" y="${(cell.y + 44).toFixed(1)}" class="folder-treemap-count">${escapeHtml(countLabel)}</text>` : '') +
                '</g>';
        });

        return `<div class="folder-treemap"><svg width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">${rects}</svg></div>`;
    }

    _squarify(items, x, y, w, h) {
        if (!items.length) return [];
        const total = items.reduce((s, f) => s + (f.size || 1), 0);
        const result = [];
        this._squarifyRow(items, x, y, w, h, total, result);
        return result;
    }

    _squarifyRow(items, x, y, w, h, total, result) {
        if (!items.length) return;
        if (items.length === 1) {
            result.push({ x, y, w, h, item: items[0] });
            return;
        }

        const area = w * h;
        let row = [];
        let rowSize = 0;
        let bestWorst = Infinity;
        let i = 0;

        while (i < items.length) {
            const item = items[i];
            const size = item.size || 1;
            row.push(item);
            rowSize += size;

            const side = Math.min(w, h);
            const rowArea = (rowSize / total) * area;
            const rowLen = rowArea / side;
            let worst = 0;
            for (const r of row) {
                const rArea = ((r.size || 1) / total) * area;
                const rSide = rArea / rowLen;
                const ratio = Math.max(rowLen / rSide, rSide / rowLen);
                if (ratio > worst) worst = ratio;
            }

            if (worst >= bestWorst) {
                row.pop();
                rowSize -= size;
                break;
            }
            bestWorst = worst;
            i++;
        }

        // Lay out the row
        const rowFrac = rowSize / total;
        let offset = 0;
        const isWide = w >= h;
        const rowDim = isWide ? w * rowFrac : h * rowFrac;

        for (const r of row) {
            const frac = (r.size || 1) / rowSize;
            if (isWide) {
                const rh = h * frac;
                result.push({ x, y: y + offset, w: rowDim, h: rh, item: r });
                offset += rh;
            } else {
                const rw = w * frac;
                result.push({ x: x + offset, y, w: rw, h: rowDim, item: r });
                offset += rw;
            }
        }

        // Recurse on remaining items
        const remainingItems = items.slice(row.length);
        if (!remainingItems.length) return;
        const remainTotal = total - rowSize;
        if (isWide) {
            this._squarifyRow(remainingItems, x + rowDim, y, w - rowDim, h, remainTotal, result);
        } else {
            this._squarifyRow(remainingItems, x, y + rowDim, w, h - rowDim, remainTotal, result);
        }
    }

    _renderDepthHistogram(subfolders) {
        const maxDepth = Math.max(...subfolders.map(s => s.maxDepth), 1);
        const bars = subfolders.map(s => {
            const pct = Math.round((s.maxDepth / maxDepth) * 100);
            const label = s.name.length > 8 ? s.name.slice(0, 7) + '…' : s.name;
            return `<div class="folder-depth-col" title="${escapeHtml(s.name)}: ${s.maxDepth} level${s.maxDepth !== 1 ? 's' : ''} deep">` +
                `<div class="folder-depth-bar" style="height:${pct}%"></div>` +
                `<div class="folder-depth-label">${escapeHtml(label)}</div>` +
                '</div>';
        }).join('');
        return `<div class="folder-depth-histogram">${bars}</div>`;
    }

    _renderFileTypeChart(fileTypes) {
        const entries = Object.entries(fileTypes).sort((a, b) => b[1] - a[1]).slice(0, 10);
        const max = entries[0]?.[1] || 1;
        const bars = entries.map(([ext, count]) => {
            const pct = Math.round((count / max) * 100);
            const extUpper = ext.toUpperCase();
            return `<div class="folder-type-row">` +
                `<span class="folder-type-ext">${escapeHtml(extUpper)}</span>` +
                `<div class="folder-type-bar-wrap"><div class="folder-type-bar" style="width:${pct}%"></div></div>` +
                `<span class="folder-type-count">${count}</span>` +
                '</div>';
        }).join('');
        return `<div class="folder-type-chart">${bars}</div>`;
    }

    _renderLibraryStats(stats) {
        const sections = [];

        // Shooting date range from shootingDays
        const days = Object.keys(stats.shootingDays || {}).sort();
        if (days.length > 0) {
            const first = days[0];
            const last = days[days.length - 1];
            const totalDays = days.length;
            const rows = [];
            rows.push(this.panel.row('First shot', first));
            rows.push(this.panel.row('Last shot', last));
            rows.push(this.panel.row('Active days', totalDays.toLocaleString()));
            rows.push(this.panel.row('Total photos', (stats.totalPhotos || 0).toLocaleString()));
            sections.push(this.panel.section('Photos', rows));
        }

        // Format breakdown
        if (stats.formats && stats.formats.length > 0) {
            const max = Math.max(...stats.formats.map(f => f.count), 1);
            const bars = stats.formats.slice(0, 8).map(f => {
                const pct = Math.round((f.count / max) * 100);
                return `<div class="folder-type-row">` +
                    `<span class="folder-type-ext">${escapeHtml(f.name.toUpperCase())}</span>` +
                    `<div class="folder-type-bar-wrap"><div class="folder-type-bar" style="width:${pct}%"></div></div>` +
                    `<span class="folder-type-count">${f.count}</span>` +
                    '</div>';
            }).join('');
            sections.push(this.panel.section('Formats', [`<div class="folder-type-chart">${bars}</div>`]));
        }

        // Camera × lens
        if (stats.cameraLens && stats.cameraLens.length > 0) {
            const max = Math.max(...stats.cameraLens.map(cl => cl.count), 1);
            const items = stats.cameraLens.slice(0, 5).map(cl => {
                const pct = Math.round((cl.count / max) * 100);
                // EXIF values come quoted from the index; a camera without a
                // lens (a phone, a compact) is named alone.
                const cam = this.panel.stripQuotes(cl.camera) || 'Unknown';
                const lens = this.panel.stripQuotes(cl.lens);
                const label = cam + (lens && lens !== '(no lens)' ? ' / ' + lens : '');
                return `<div class="folder-cam-item">` +
                    `<div class="folder-cam-bar-row">` +
                    `<span class="folder-cam-count">${cl.count}x</span>` +
                    `<div class="folder-cam-bar-wrap"><div class="folder-cam-bar" style="width:${pct}%"></div></div>` +
                    `</div>` +
                    `<div class="folder-cam-label">${escapeHtml(label)}</div>` +
                    `</div>`;
            }).join('');
            sections.push(this.panel.section('Camera', [`<div class="folder-cam-chart">${items}</div>`]));
        }

        // Shooting hours (24-bar chart)
        if (stats.shootingHours && stats.shootingHours.some(h => h > 0)) {
            sections.push(this.panel.section('Shooting hours', [this._renderHoursChart(stats.shootingHours)]));
        }

        return sections;
    }

    _renderHoursChart(hours) {
        const max = Math.max(...hours, 1);
        const bars = hours.map((count, h) => {
            const pct = Math.round((count / max) * 100);
            const label = h + 'h';
            return `<div class="folder-hours-col" title="${label}: ${count}">` +
                `<div class="folder-hours-bar" style="height:${pct}%"></div>` +
                `<div class="folder-hours-label">${h % 6 === 0 ? label : ''}</div>` +
                '</div>';
        }).join('');
        return `<div class="folder-hours-chart">${bars}</div>`;
    }

    attachEvents() {
        this.panel.container.querySelectorAll('.folder-treemap-cell').forEach(cell => {
            cell.addEventListener('click', () => {
                const path = cell.dataset.path;
                if (path && this.panel.onDirNavigate) this.panel.onDirNavigate(path);
            });
        });
    }
}
