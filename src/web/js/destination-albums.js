// destination-albums.js — rebuilding a website destination's album list
// from what its photos record, and the report of what that found.

function wireRebuildAlbums(root, ch) {
    const btn = root.querySelector('#dest-rebuild-albums');
    const out = root.querySelector('#dest-rebuild-albums-report');
    btn.addEventListener('click', async () => {
        const restore = Activity.button(btn, t('Rebuilding…'));
        Activity.in(out, t('Reading the photos…'));
        try {
            out.innerHTML = albumReportHTML(await ChannelAPI.rebuildAlbumList(ch.slug));
        } catch (err) {
            out.innerHTML = `<div class="gal-detail-error">${t('Could not rebuild the album list: {error}', { error: escapeHtml(err.message) })}</div>`;
        } finally {
            restore();
        }
    });
}

// What the rebuild found, in plain sentences: what it added, what it
// completed, and what it left alone and why.
function albumReportHTML(r) {
    const found = r.added.length + r.present + r.unreadable.length + r.deleted;
    if (found === 0 && r.photos.length === 0 && !r.sidecarsCompleted && !r.unreachable) {
        return '<p>' + t('No albums found in the photos of this destination.') + '</p>';
    }
    return [
        albumsFoundHTML(r, found),
        r.sidecarsCompleted ? `<p>${t('Completed {sidecars} with the listed albums’ membership and address, so those albums can be restored from the photos too.', { sidecars: countText(r.sidecarsCompleted, 'photo sidecar') })}</p>` : '',
        r.unreachable ? `<p>${t('{photos} of the listed albums could not be reached from here (not in a scanned library, not mounted, or an old entry without a photo). Their sidecars were not touched.', { photos: countText(r.unreachable, 'photo') })}</p>` : '',
        r.unreadable.length ? olderPublicationsHTML(r.unreadable) : '',
        r.photos.length ? unreadPhotosHTML(r.photos) : '',
    ].join('');
}

function albumsFoundHTML(r, found) {
    const left = r.deleted ? ' ' + t('{n} deleted earlier, left out.', { n: formatCount(r.deleted) }) : '';
    const head = `<p>${t('Found {albums} in the photos. Added {added}; {present} already listed.', { albums: countText(found, 'album'), added: formatCount(r.added.length), present: formatCount(r.present) })}${left}</p>`;
    if (!r.added.length) return head;
    return head + `<ul>${r.added.map(a => `<li>${escapeHtml(a.title)} <span class="dest-report-data">/${escapeHtml(a.slug)}</span>, ${countText(a.photos, 'photo')}</li>`).join('')}</ul>`;
}

// Publications the list does not know and that carry no address: in
// practice albums that were deleted or replaced before deleting also cleaned
// up the photos. Left as they are, grouped by title.
function olderPublicationsHTML(unreadable) {
    const n = unreadable.length;
    const groups = new Map(); // label (HTML) -> count; an ID is data, a title is text
    for (const u of unreadable) {
        const label = u.title ? escapeHtml(u.title) : `<span class="dest-report-data">${escapeHtml(u.postID)}</span>`;
        groups.set(label, (groups.get(label) || 0) + 1);
    }
    const sentence = n === 1
        ? t('1 older publication is not in the list and has no recorded address, so it cannot be restored without inventing one. Usually these are albums that were deleted or replaced. They are left as they are.')
        : t('{n} older publications are not in the list and have no recorded address, so they cannot be restored without inventing one. Usually these are albums that were deleted or replaced. They are left as they are.', { n: formatCount(n) });
    const items = [...groups].map(([label, count]) => `<li>${label}${count > 1 ? ` <span class="dest-report-data">×${count}</span>` : ''}</li>`).join('');
    return `<p>${sentence}</p><details class="dest-report-details"><summary>${n === 1 ? t('Show it') : t('Show them')}</summary><ul>${items}</ul></details>`;
}

function unreadPhotosHTML(photos) {
    const shown = photos.slice(0, 10);
    const more = photos.length > shown.length ? `<li>${t('and {n} more', { n: formatCount(photos.length - shown.length) })}</li>` : '';
    return `<p class="dest-report-warning">${t('{photos} could not be read:', { photos: countText(photos.length, 'photo') })}</p>`
        + `<ul>${shown.map(p => `<li><span class="dest-report-data">${escapeHtml(p.filename)}</span>: ${escapeHtml(p.reason)}</li>`).join('')}${more}</ul>`;
}
