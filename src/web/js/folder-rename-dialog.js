// folder-rename-dialog.js — renames one folder (Folders, Organize, a library).
//
// diskPath: the folder relative to the browse boundary, the kind of path
// /api/rename takes. onDone(newName) runs after the rename; the server has
// moved the folder's photos in their library by then.

function openFolderRenameDialog(diskPath, onDone) {
    const oldName = diskPath.split('/').pop();
    const dlg = new Dialog({
        title: t('Rename folder'),
        subtitle: oldName,
        size: 'sm',
        className: 'library-dialog',
        body: `
            <label class="library-dialog-label" for="folder-rename">${t('Name')}</label>
            <input class="library-dialog-input" id="folder-rename" type="text" autocomplete="off" spellcheck="false" value="${escapeHtml(oldName)}">`,
        actions: [
            { label: t('Cancel'), value: null },
            { label: t('Rename'), kind: 'primary', onClick: (d) => renameFolder(d, diskPath, oldName, onDone) },
        ],
    });
    const input = dlg.open().querySelector('#folder-rename');
    input.select();
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') { e.preventDefault(); renameFolder(dlg, diskPath, oldName, onDone); }
    });
}

async function renameFolder(dlg, diskPath, oldName, onDone) {
    const name = dlg.el.querySelector('#folder-rename').value.trim();
    if (!name || name === oldName) { dlg.close(null); return; }
    if (name.includes('/') || name === '.' || name === '..') {
        dlg.setNote(t('A folder name cannot contain "/". Choose another name.'), 'error');
        return;
    }
    dlg.setNote(t('Renaming…'));
    try {
        await API.rename(diskPath, name);
    } catch (err) {
        dlg.setNote(t('The folder was not renamed: {err}', { err: err.message.trim() }), 'error');
        return;
    }
    dlg.close(name);
    onDone?.(name);
}
