// GlobalKeyboard — global keydown handler for App

class GlobalKeyboard {
    constructor(app) {
        this._app = app;
        this.isMac = /Mac|iPhone|iPad|iPod/.test(navigator.platform);
        this._inputActive = false;
    }

    attach() {
        document.addEventListener('keydown', (e) => this._handle(e));
        document.addEventListener('focusin', (e) => {
            const t = e.target?.tagName;
            this._inputActive = t === 'INPUT' || t === 'TEXTAREA' || t === 'SELECT' || !!e.target?.isContentEditable;
        });
        // Defer so _inputActive stays true for the entire synchronous event-loop
        // turn that contains the keydown. Safari fires blur on input[type="date"]
        // synchronously during key processing — before the keydown finishes bubbling
        // to our document listener — so a non-deferred clear would make _inputActive
        // false by the time _isInputFocused() runs.
        document.addEventListener('focusout', () => {
            setTimeout(() => {
                const active = document.activeElement;
                const t = active?.tagName;
                this._inputActive = t === 'INPUT' || t === 'TEXTAREA' || t === 'SELECT' || !!active?.isContentEditable;
            }, 0);
        });
    }

    // Returns true when a form element (input, textarea, select, contenteditable) is
    // active. Uses three complementary checks to survive shadow DOM retargeting and
    // Safari's non-standard blur-during-keydown behaviour for date inputs.
    _isInputFocused(e) {
        if (this._inputActive) return true;
        const formTags = new Set(['INPUT', 'TEXTAREA', 'SELECT']);
        for (const el of e.composedPath()) {
            if (formTags.has(el.tagName) || el.isContentEditable) return true;
        }
        const active = document.activeElement;
        return !!(active && (formTags.has(active.tagName) || active.isContentEditable));
    }

    _handle(e) {
        const app = this._app;

        // A dialog owns the keyboard while it is open, Escape included: the
        // Dialog handles its own keys (ADR-0033). The same goes for the crop
        // tool, which draws on the photo and takes the keys with it.
        if (document.querySelector('.dialog-scrim, .keyboard-owner')) return;

        if (e.key !== 'Escape' && this._isInputFocused(e)) return;

        // Organize: the number keys aim at a target, Enter uses the current
        // one, U takes the last move back (ADR-0032). The screen answers for
        // itself so the shortcut list stays with the place that owns it.
        if (app.mode === 'organize' && app.organize && !document.querySelector('.viewer')) {
            if (app.organize.handleKey(e)) return;
        }

        // Escape: clear selection or go up in browse mode
        if (e.key === 'Escape' && app.mode === 'browse' && app.browsePane) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            if (app.browsePane.selectedDirs.size > 0) {
                app.browsePane.selectedDirs.clear();
                app.browsePane._updateDirSelectionClasses();
                if (app.browsePane.onSelectionChange) app.browsePane.onSelectionChange([]);
                return;
            }
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

        // Escape: clear selection or go up in library mode
        if (e.key === 'Escape' && app.mode === 'library' && app._libraryTab) {
            if (document.querySelector('.viewer')) return;
            const pane = app._libraryTab.getActivePaneForKeyboard();
            if (!pane) return;
            e.preventDefault();
            if (pane.selectedDirs?.size > 0) {
                pane.selectedDirs.clear();
                pane._updateDirSelectionClasses();
                if (pane.onSelectionChange) pane.onSelectionChange([]);
                return;
            }
            if (pane.selection.selected.size > 0) {
                pane.selection.clear();
                pane.updateSelectionClasses();
                if (pane.onSelectionChange) pane.onSelectionChange([]);
                return;
            }
            if (pane === app._libraryTab._pane && pane.path) {
                const parts = pane.path.split('/').filter(Boolean);
                if (parts.length > 0) {
                    parts.pop();
                    pane.load(parts.join('/'));
                }
            }
        }

        // Escape: clear selection or go up in Organize
        if (e.key === 'Escape' && app.mode === 'organize' && app.organize) {
            e.preventDefault();
            const pane = app.organize.pane;
            if (pane.selectedDirs?.size > 0) {
                pane.selectedDirs.clear();
                pane._updateDirSelectionClasses();
                if (pane.onSelectionChange) pane.onSelectionChange([]);
                return;
            }
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

        // Backspace: mark for deletion in Organize
        if (e.key === 'Backspace' && app.mode === 'organize' && app.organize) {
            e.preventDefault();
            app.organize.sendTo('mark');
        }

        // Backspace: mark for deletion in library mode
        if (e.key === 'Backspace' && app.mode === 'library' && app._libraryTab) {
            e.preventDefault();
            if (document.querySelector('.viewer')) return;
            const pane = app._libraryTab.getActivePaneForKeyboard();
            if (!pane) return;
            const targets = pane.getActionableFiles();
            if (targets.length === 0) return;
            const photoMeta = {};
            let hasLibMeta = false;
            for (const path of targets) {
                const m = pane.getLibraryMeta?.(path);
                if (m) { photoMeta[path] = m; hasLibMeta = true; }
            }
            app.wastebin.mark(targets, pane.entries, pane.path, hasLibMeta ? photoMeta : null);
            pane.selection.clear();
            pane.updateSelectionClasses();
            pane.updateMarkedForDeletion();
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
        if ((e.key === 'i' || e.key === 'I') && app.mode === 'library' && app._libraryTab) {
            if (document.querySelector('.viewer')) return;
            // Nothing to describe in the overview until the filter shows photos.
            const ip = app._libraryTab._infoPanel;
            if (!ip || !app._libraryTab.getActivePaneForKeyboard()) return;
            e.preventDefault();
            ip.toggle();
            if (ip.expanded) {
                const activePane = app._libraryTab.getActivePaneForKeyboard();
                if (activePane) activePane._notifyFocusChange();
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

        // Delete: mark for deletion in Organize
        if (e.key === 'Delete' && app.mode === 'organize' && app.organize) {
            e.preventDefault();
            app.organize.sendTo('mark');
        }

        // Delete: mark for deletion in library mode
        if (e.key === 'Delete' && app.mode === 'library' && app._libraryTab) {
            e.preventDefault();
            if (document.querySelector('.viewer')) return;
            const pane = app._libraryTab.getActivePaneForKeyboard();
            if (!pane) return;
            const targets = pane.getActionableFiles();
            if (targets.length === 0) return;
            const photoMeta = {};
            let hasLibMeta = false;
            for (const path of targets) {
                const m = pane.getLibraryMeta?.(path);
                if (m) { photoMeta[path] = m; hasLibMeta = true; }
            }
            app.wastebin.mark(targets, pane.entries, pane.path, hasLibMeta ? photoMeta : null);
            pane.selection.clear();
            pane.updateSelectionClasses();
            pane.updateMarkedForDeletion();
        }

        const modKey = this.isMac ? e.metaKey : e.ctrlKey;

        // Cmd/Ctrl+A: select all
        if (modKey && (e.key === 'a' || e.key === 'A') && !e.shiftKey && !e.altKey) {
            if (app.mode === 'browse' && app.browsePane && !document.querySelector('.viewer')) {
                e.preventDefault();
                app.browsePane.selectAll();
            } else if (app.mode === 'organize' && app.organize) {
                e.preventDefault();
                app.organize.pane.selectAll();
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
            } else if (app.mode === 'organize' && app.organize) {
                e.preventDefault();
                app.organize.sendTo('mark');
            }
        }

        // Arrow keys / Enter / Space: browse pane navigation
        if (!e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
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

        // Backslash: collapse or expand the sidebar
        if (e.key === '\\' && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            app.toggleSidebar();
        }

        // 1–6: switch places
        if (!e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
            if (e.key === '1') { e.preventDefault(); app.setMode('browse'); }
            else if (e.key === '2') { e.preventDefault(); app.setMode('wastebin'); }
            else if (e.key === '3') { e.preventDefault(); app.setMode('organize'); }
            else if (e.key === '4') { e.preventDefault(); app.setMode('library'); }
            else if (e.key === '5') { e.preventDefault(); app.setMode('published'); }
            else if (e.key === '6') { e.preventDefault(); app.setMode('destinations'); }
            else if (e.key === ',') { e.preventDefault(); app.setMode('settings'); }
        }

        // H: toggle UI visibility
        if ((e.key === 'h' || e.key === 'H') && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            app.toggleUIVisibility();
        }
    }
}
