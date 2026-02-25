// Commander mode — dual pane file browser

const CMD_ICONS = {
    copy: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><rect x="4.5" y="4.5" width="8" height="8" rx="1"/><path d="M1.5 9.5v-7a1 1 0 0 1 1-1h7"/></svg>',
    move: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><rect x="1.5" y="3.5" width="7" height="8" rx="1"/><path d="M10.5 5l2.5 2-2.5 2"/><path d="M8.5 7h4.5"/></svg>',
    delete: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M2.5 4.5h9"/><path d="M5 4.5V3a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1.5"/><path d="M3.5 4.5l.5 7.5a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1l.5-7.5"/></svg>',
};

class Commander {
    constructor(container) {
        this.container = container;
        this.leftPane = null;
        this.rightPane = null;
        this.activePane = 'left';
        this.onImageClick = null;
    }

    init() {
        this.container.innerHTML = `
            <div class="commander">
                <div class="commander-pane left-pane" id="left-pane"></div>
                <div class="commander-actions">
                    <button class="btn btn-action" id="cmd-copy" title="Copy (F5)" disabled>${CMD_ICONS.copy} Copy →</button>
                    <button class="btn btn-action" id="cmd-move" title="Move (F6)" disabled>${CMD_ICONS.move} Move →</button>
                    <button class="btn btn-action" id="cmd-delete" title="Marked for Deletion (Del)" disabled>${CMD_ICONS.delete} Delete</button>
                </div>
                <div class="commander-pane right-pane" id="right-pane"></div>
            </div>
        `;

        const leftEl = document.getElementById('left-pane');
        const rightEl = document.getElementById('right-pane');

        this.leftPane = new BrowsePane(leftEl, {
            onImageClick: (path) => {
                if (this.onImageClick) this.onImageClick(path, this.leftPane);
            },
            onSelectionChange: () => this.updateActions(),
        });

        this.rightPane = new BrowsePane(rightEl, {
            onImageClick: (path) => {
                if (this.onImageClick) this.onImageClick(path, this.rightPane);
            },
            onSelectionChange: () => this.updateActions(),
        });

        // Track active pane
        leftEl.addEventListener('click', () => {
            this.activePane = 'left';
            leftEl.classList.add('active');
            rightEl.classList.remove('active');
            this.updateActions();
        });

        rightEl.addEventListener('click', () => {
            this.activePane = 'right';
            rightEl.classList.add('active');
            leftEl.classList.remove('active');
            this.updateActions();
        });

        // Copy/Move/Delete buttons
        document.getElementById('cmd-copy').addEventListener('click', () => this.doCopy());
        document.getElementById('cmd-move').addEventListener('click', () => this.doMove());
        document.getElementById('cmd-delete').addEventListener('click', () => this.doDelete());

        // Load both panes
        leftEl.classList.add('active');
        this.leftPane.load('');
        this.rightPane.load('');
    }

    getActivePane() {
        return this.activePane === 'left' ? this.leftPane : this.rightPane;
    }

    getOtherPane() {
        return this.activePane === 'left' ? this.rightPane : this.leftPane;
    }

    updateActions() {
        const active = this.getActivePane();
        const hasSelection = active.getSelectedFiles().length > 0;
        document.getElementById('cmd-copy').disabled = !hasSelection;
        document.getElementById('cmd-move').disabled = !hasSelection;
        document.getElementById('cmd-delete').disabled = !hasSelection;

        // Update button labels with direction
        const direction = this.activePane === 'left' ? '→' : '←';
        const count = active.getSelectedFiles().length;
        const label = count > 0 ? ` (${count})` : '';
        document.getElementById('cmd-copy').innerHTML = `${CMD_ICONS.copy} Copy ${direction}${label}`;
        document.getElementById('cmd-move').innerHTML = `${CMD_ICONS.move} Move ${direction}${label}`;
        document.getElementById('cmd-delete').innerHTML = `${CMD_ICONS.delete} Delete${label}`;
    }

    async doCopy() {
        const files = this.getActivePane().getSelectedFiles();
        const dest = this.getOtherPane().path;

        if (files.length === 0) return;
        if (!confirm(`Copy ${files.length} file(s) to "${dest || 'root'}"?`)) return;

        try {
            const result = await API.copy(files, dest);
            this.showResults('Copy', result.results);
            // Refresh both panes
            this.leftPane.load(this.leftPane.path);
            this.rightPane.load(this.rightPane.path);
        } catch (err) {
            alert('Copy failed: ' + err.message);
        }
    }

    async doMove() {
        const files = this.getActivePane().getSelectedFiles();
        const dest = this.getOtherPane().path;

        if (files.length === 0) return;
        if (!confirm(`Move ${files.length} file(s) to "${dest || 'root'}"?`)) return;

        try {
            const result = await API.move(files, dest);
            this.showResults('Move', result.results);
            // Refresh both panes
            this.leftPane.load(this.leftPane.path);
            this.rightPane.load(this.rightPane.path);
        } catch (err) {
            alert('Move failed: ' + err.message);
        }
    }

    doDelete() {
        const active = this.getActivePane();
        const files = active.getSelectedFiles();
        if (files.length === 0) return;

        App.markForDeletion(files, active.entries, active.path);
        active.selected.clear();
        active.render();
        this.updateActions();
    }

    showResults(op, results) {
        const failures = results.filter(r => !r.success);
        if (failures.length > 0) {
            const msgs = failures.map(f => `${f.file}: ${f.error}`).join('\n');
            alert(`${op}: ${failures.length} error(s):\n${msgs}`);
        }
    }
}
