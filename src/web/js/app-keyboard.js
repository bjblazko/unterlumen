// GlobalKeyboard — global keydown handler for App

class GlobalKeyboard {
    constructor(app) {
        this._app = app;
        this.isMac = /Mac|iPhone|iPad|iPod/.test(navigator.platform);
    }

    attach() {
        document.addEventListener('keydown', (e) => this._handle(e));
    }

    _handle(e) {
        const app = this._app;

        if (document.querySelector('.modal-overlay')) return;

        // Tab: switch panes in commander mode
        if (e.key === 'Tab' && app.mode === 'commander' && app.commander) {
            e.preventDefault();
            const leftEl = document.getElementById('left-pane');
            const rightEl = document.getElementById('right-pane');
            if (app.commander.activePane === 'left') {
                app.commander.activePane = 'right';
                leftEl.classList.remove('active');
                rightEl.classList.add('active');
            } else {
                app.commander.activePane = 'left';
                rightEl.classList.remove('active');
                leftEl.classList.add('active');
            }
            app.commander.updateActions();
        }

        // Escape: clear selection or go up in browse mode
        if (e.key === 'Escape' && app.mode === 'browse' && app.browsePane) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            if (app.browsePane.selection.selected.size > 0) {
                app.browsePane.selection.clear();
                app.browsePane.updateSelectionClasses();
                if (app.browsePane.onSelectionChange) app.browsePane.onSelectionChange([]);
                return;
            }
            const parts = app.browsePane.path.split('/').filter(Boolean);
            parts.pop();
            const parentPath = parts.join('/');
            app.browsePane.load(parentPath);
            app.currentBrowsePath = parentPath;
        }

        // Escape: clear selection in library mode
        if (e.key === 'Escape' && app.mode === 'library' && app._libraryTab) {
            const pane = app._libraryTab.getActivePaneForKeyboard();
            if (pane && pane.selection.selected.size > 0) {
                e.preventDefault();
                pane.selection.clear();
                pane.updateSelectionClasses();
                if (pane.onSelectionChange) pane.onSelectionChange([]);
            }
        }

        // Escape: clear selection or go up in commander mode
        if (e.key === 'Escape' && app.mode === 'commander' && app.commander) {
            e.preventDefault();
            const pane = app.commander.getActivePane();
            if (pane.selection.selected.size > 0) {
                pane.selection.clear();
                pane.updateSelectionClasses();
                if (pane.onSelectionChange) pane.onSelectionChange([]);
                return;
            }
            const parts = pane.path.split('/').filter(Boolean);
            parts.pop();
            pane.load(parts.join('/'));
        }

        // Backspace: mark for deletion in browse mode
        if (e.key === 'Backspace' && app.mode === 'browse' && app.browsePane) {
            e.preventDefault();
            if (document.querySelector('.viewer')) return;
            const targets = app.browsePane.getActionableFiles();
            if (targets.length === 0) return;
            app.wastebin.mark(targets, app.browsePane.entries, app.browsePane.path);
            app.browsePane.selection.clear();
            app.browsePane.updateSelectionClasses();
            app.browsePane.updateMarkedForDeletion();
        }

        // Backspace: mark for deletion in commander mode
        if (e.key === 'Backspace' && app.mode === 'commander' && app.commander) {
            e.preventDefault();
            const targets = app.commander.getActivePane().getActionableFiles();
            if (targets.length === 0) return;
            app.commander.doDelete();
        }

        // I: toggle info panel in browse mode
        if ((e.key === 'i' || e.key === 'I') && app.mode === 'browse' && app.infoPanel) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            app.infoPanel.toggle();
            if (app.infoPanel.expanded && app.browsePane) {
                app.browsePane._notifyFocusChange();
            }
        }

        // I: toggle info panel in library mode
        if ((e.key === 'i' || e.key === 'I') && app.mode === 'library' && app._libraryTab && app._libraryTab._infoPanel) {
            e.preventDefault();
            const ip = app._libraryTab._infoPanel;
            ip.toggle();
            if (ip.expanded && app._libraryTab._pane) {
                app._libraryTab._pane._notifyFocusChange();
            }
        }

        // Delete: mark for deletion in browse mode
        if (e.key === 'Delete' && app.mode === 'browse' && app.browsePane) {
            if (document.querySelector('.viewer')) return;
            const targets = app.browsePane.getActionableFiles();
            if (targets.length === 0) return;
            e.preventDefault();
            app.wastebin.mark(targets, app.browsePane.entries, app.browsePane.path);
            app.browsePane.selection.clear();
            app.browsePane.updateSelectionClasses();
            app.browsePane.updateMarkedForDeletion();
        }

        // Delete: mark for deletion in commander mode
        if (e.key === 'Delete' && app.mode === 'commander' && app.commander) {
            const targets = app.commander.getActivePane().getActionableFiles();
            if (targets.length === 0) return;
            e.preventDefault();
            app.commander.doDelete();
        }

        // F5/F6: copy/move in commander mode
        if (app.mode === 'commander' && app.commander) {
            if (e.key === 'F5') { e.preventDefault(); app.commander.doCopy(); }
            else if (e.key === 'F6') { e.preventDefault(); app.commander.doMove(); }
        }

        const modKey = this.isMac ? e.metaKey : e.ctrlKey;

        // Cmd/Ctrl+A: select all
        if (modKey && (e.key === 'a' || e.key === 'A') && !e.shiftKey && !e.altKey) {
            if (app.mode === 'browse' && app.browsePane && !document.querySelector('.viewer')) {
                e.preventDefault();
                app.browsePane.selectAll();
            } else if (app.mode === 'commander' && app.commander) {
                e.preventDefault();
                app.commander.getActivePane().selectAll();
            } else if (app.mode === 'library' && app._libraryTab) {
                e.preventDefault();
                app._libraryTab.getActivePaneForKeyboard()?.selectAll();
            } else if (app.mode === 'wastebin') {
                e.preventDefault();
                app.wastebin.selectAll();
                app.wastebin.render(app._wastebinEl, () => app._refreshPanes());
            }
        }

        // Cmd/Ctrl+D: mark for deletion
        if (modKey && (e.key === 'd' || e.key === 'D') && !e.shiftKey && !e.altKey) {
            if (app.mode === 'browse' && app.browsePane && !document.querySelector('.viewer')) {
                e.preventDefault();
                const targets = app.browsePane.getActionableFiles();
                if (targets.length > 0) {
                    app.wastebin.mark(targets, app.browsePane.entries, app.browsePane.path);
                    app.browsePane.selection.clear();
                    app.browsePane.updateSelectionClasses();
                    app.browsePane.updateMarkedForDeletion();
                }
            } else if (app.mode === 'commander' && app.commander) {
                e.preventDefault();
                app.commander.doDelete();
            }
        }

        // Arrow keys / Enter / Space: browse pane navigation
        if (!e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;
            if (!document.querySelector('.viewer')) {
                const pane = app.getActiveBrowsePane();
                if (pane) {
                    if (e.key === 'ArrowLeft') {
                        e.preventDefault(); pane.moveFocus(-1);
                    } else if (e.key === 'ArrowRight') {
                        e.preventDefault(); pane.moveFocus(1);
                    } else if (e.key === 'ArrowUp') {
                        e.preventDefault(); pane.moveFocus(-pane.getColumnCount());
                    } else if (e.key === 'ArrowDown') {
                        e.preventDefault(); pane.moveFocus(pane.getColumnCount());
                    } else if (e.key === 'Enter') {
                        e.preventDefault(); pane.activateFocused();
                    } else if (e.key === ' ') {
                        e.preventDefault(); pane.toggleFocusedSelection();
                    }
                }
            }
        }

        // 1/2/3: switch modes
        if (!e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;
            if (e.key === '1') { e.preventDefault(); app.setMode('browse'); }
            else if (e.key === '2') { e.preventDefault(); app.setMode('wastebin'); }
            else if (e.key === '3') { e.preventDefault(); app.setMode('commander'); }
        }

        // H: toggle UI visibility
        if ((e.key === 'h' || e.key === 'H') && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            app.toggleUIVisibility();
        }
    }
}
