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
        // A dialog owns the keyboard while it is open, Escape included: the
        // Dialog handles its own keys (ADR-0033). The same goes for the crop
        // tool, which draws on the photo and takes the keys with it.
        if (document.querySelector('.dialog-scrim, .keyboard-owner')) return;

        if (e.key !== 'Escape' && this._isInputFocused(e)) return;

        // Organize: the number keys aim at a target, Enter uses the current
        // one, U takes the last move back (ADR-0032). The screen answers for
        // itself so the shortcut list stays with the place that owns it.
        const app = this._app;
        if (app.mode === 'organize' && app.organize && !this._viewerOpen()) {
            if (app.organize.handleKey(e)) return;
        }

        this._handlerFor(e)?.call(this, e);
    }

    // _handlerFor picks the one shortcut a key press means, or none.
    _handlerFor(e) {
        const modKey = this.isMac ? e.metaKey : e.ctrlKey;
        const plain = !e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey;
        const noCommand = !e.metaKey && !e.ctrlKey && !e.altKey;
        const key = e.key;
        const lower = key.toLowerCase();

        if (key === 'Escape') return this._escape;
        if (key === 'Backspace') return (ev) => this._markKey(ev, true);
        if (key === 'Delete') return (ev) => this._markKey(ev, false);
        if (key === 'i' || key === 'I') return this._toggleInfo;
        if (modKey && !e.shiftKey && !e.altKey && (key === 'a' || key === 'A')) return this._selectAll;
        if (modKey && !e.shiftKey && !e.altKey && (key === 'd' || key === 'D')) return this._markSelection;
        if (plain && GlobalKeyboard.NAVIGATION_KEYS.has(key)) return this._navigate;
        if (key === '\\' && noCommand) return this._toggleSidebar;
        if (plain && GlobalKeyboard.PLACE_KEYS[key]) return (ev) => { ev.preventDefault(); this._app.setMode(GlobalKeyboard.PLACE_KEYS[key]); };
        if (lower === 'h' && noCommand) return this._toggleUI;
        return null;
    }

    _viewerOpen() {
        return !!document.querySelector('.viewer');
    }

    // Escape clears the selection, or else goes up one folder; in Settings it
    // goes back to the place before.
    _escape(e) {
        const app = this._app;
        if (app.mode === 'browse' && app.browsePane) {
            if (this._viewerOpen()) return;
            e.preventDefault();
            if (this._clearSelection(app.browsePane)) return;
            const parentPath = parentFolder(app.browsePane.path);
            app.browsePane.load(parentPath);
            app.currentBrowsePath = parentPath;
        } else if (app.mode === 'library' && app._libraryTab) {
            if (this._viewerOpen()) return;
            const pane = app._libraryTab.getActivePaneForKeyboard();
            if (!pane) return;
            e.preventDefault();
            if (this._clearSelection(pane)) return;
            if (pane === app._libraryTab._pane && pane.path && pane.path.split('/').filter(Boolean).length > 0) {
                pane.load(parentFolder(pane.path));
            }
        } else if (app.mode === 'organize' && app.organize) {
            e.preventDefault();
            const pane = app.organize.pane;
            if (this._clearSelection(pane)) return;
            pane.load(parentFolder(pane.path));
        } else if (app.mode === 'settings') {
            // Escape is Done: back to where Settings was opened from.
            e.preventDefault();
            app.leaveSettings();
        }
    }

    // _clearSelection clears the selected folders, or else the selected
    // files, and reports whether there was anything to clear.
    _clearSelection(pane) {
        if (pane.selectedDirs?.size > 0) {
            pane.selectedDirs.clear();
            pane._updateDirSelectionClasses();
        } else if (pane.selection.selected.size > 0) {
            pane.selection.clear();
            pane.updateSelectionClasses();
        } else {
            return false;
        }
        if (pane.onSelectionChange) pane.onSelectionChange([]);
        return true;
    }

    // Backspace and Delete mark the selection for deletion. Backspace always
    // keeps the browser from acting on the key; Delete only when it marks
    // something in Folders.
    _markKey(e, alwaysPrevent) {
        const app = this._app;
        if (app.mode === 'browse' && app.browsePane) {
            if (alwaysPrevent) e.preventDefault();
            if (this._viewerOpen()) return;
            if (!this._markInPane(app.browsePane, false)) return;
            if (!alwaysPrevent) e.preventDefault();
        } else if (app.mode === 'organize' && app.organize) {
            e.preventDefault();
            app.organize.sendTo('mark');
        } else if (app.mode === 'library' && app._libraryTab) {
            e.preventDefault();
            if (this._viewerOpen()) return;
            const pane = app._libraryTab.getActivePaneForKeyboard();
            if (pane) this._markInPane(pane, true);
        }
    }

    // _markInPane marks a pane's selected files, or its focused one, for
    // deletion — with their library metadata when asked — and reports whether
    // there was anything to mark.
    _markInPane(pane, withLibraryMeta) {
        const targets = pane.getActionableFiles();
        if (targets.length === 0) return false;
        let photoMeta = null;
        if (withLibraryMeta) {
            const meta = {};
            for (const path of targets) {
                const m = pane.getLibraryMeta?.(path);
                if (m) meta[path] = m;
            }
            if (Object.keys(meta).length > 0) photoMeta = meta;
        }
        this._app.wastebin.mark(targets, pane.entries, pane.path, photoMeta);
        pane.selection.clear();
        pane.updateSelectionClasses();
        pane.updateMarkedForDeletion();
        return true;
    }

    // I shows or hides the info panel of the place.
    _toggleInfo(e) {
        const app = this._app;
        if (app.mode === 'browse' && app.infoPanel) {
            if (this._viewerOpen()) return;
            e.preventDefault();
            app.infoPanel.toggle();
            if (app.infoPanel.expanded && app.browsePane) app.browsePane._notifyFocusChange();
        } else if (app.mode === 'library' && app._libraryTab) {
            if (this._viewerOpen()) return;
            // Nothing to describe in the overview until the filter shows photos.
            const ip = app._libraryTab._infoPanel;
            if (!ip || !app._libraryTab.getActivePaneForKeyboard()) return;
            e.preventDefault();
            ip.toggle();
            if (ip.expanded) app._libraryTab.getActivePaneForKeyboard()?._notifyFocusChange();
        }
    }

    // Cmd/Ctrl+A selects everything in the place.
    _selectAll(e) {
        const app = this._app;
        if (app.mode === 'browse' && app.browsePane && !this._viewerOpen()) {
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

    // Cmd/Ctrl+D marks the selection for deletion.
    _markSelection(e) {
        const app = this._app;
        if (app.mode === 'browse' && app.browsePane && !this._viewerOpen()) {
            e.preventDefault();
            this._markInPane(app.browsePane, false);
        } else if (app.mode === 'organize' && app.organize) {
            e.preventDefault();
            app.organize.sendTo('mark');
        }
    }

    // Arrow keys move the focus, Enter opens, Space selects.
    _navigate(e) {
        if (this._viewerOpen()) return;
        // Enter and Space on a focused button or link are that control's own:
        // taking them for the grid left keyboard users unable to press it.
        if ((e.key === 'Enter' || e.key === ' ') && e.target.closest?.('button, a[href]')) return;
        const pane = this._app.getActiveBrowsePane();
        if (!pane) return;
        e.preventDefault();
        switch (e.key) {
            case 'ArrowLeft': pane.moveFocus(-1); break;
            case 'ArrowRight': pane.moveFocus(1); break;
            case 'ArrowUp': pane.moveFocus(-pane.getColumnCount()); break;
            case 'ArrowDown': pane.moveFocus(pane.getColumnCount()); break;
            case 'Enter': pane.activateFocused(); break;
            case ' ': pane.toggleFocusedSelection(); break;
        }
    }

    // Backslash collapses or expands the sidebar.
    _toggleSidebar(e) {
        if (this._viewerOpen()) return;
        e.preventDefault();
        this._app.toggleSidebar();
    }

    // H hides or shows the interface around the photos.
    _toggleUI(e) {
        if (this._viewerOpen()) return;
        e.preventDefault();
        this._app.toggleUIVisibility();
    }
}

GlobalKeyboard.NAVIGATION_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Enter', ' ']);

// The number keys switch places; comma opens Settings.
GlobalKeyboard.PLACE_KEYS = {
    '1': 'browse', '2': 'wastebin', '3': 'organize', '4': 'library',
    '5': 'published', '6': 'destinations', ',': 'settings',
};

// parentFolder is the path one folder up; "" above the top.
function parentFolder(path) {
    const parts = path.split('/').filter(Boolean);
    parts.pop();
    return parts.join('/');
}
