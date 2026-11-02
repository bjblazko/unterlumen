// rating.js — stars, 1 to 5 (ADR-0060). A photo's rating is set with the
// keys 1–5 (0 takes it away) on the selection or the photo in the viewer, or
// with a click on a star in the info panel or the viewer. It is always
// written beside the file — the XMP sidecar of a photo, the NFO of a video —
// so it is quick, needs no exiftool and leaves the file and its library ID
// as they are. Stars are data, not status: drawn in the text colour.

const Rating = {
    MAX: 5,

    // of reads a rating as stars; 0 for none, rejected or anything else.
    of(value) {
        const n = parseInt(value, 10);
        return n >= 1 && n <= Rating.MAX ? n : 0;
    },

    label(n) {
        return n ? countText(n, 'star') : t('No rating');
    },

    // marksHTML shows a rating where it is only read: on a tile, in the list.
    marksHTML(n, cls = 'rating-marks') {
        if (!n) return '';
        return `<span class="${cls}" title="${escapeHtml(Rating.label(n))}" aria-label="${escapeHtml(Rating.label(n))}">${'★'.repeat(n)}</span>`;
    },

    // controlHTML is five stars to click. The star of the current rating
    // takes it away.
    controlHTML(n) {
        const stars = [];
        for (let i = 1; i <= Rating.MAX; i++) {
            const label = i === n ? t('Remove the rating') : t('Rate {stars}', { stars: countText(i, 'star') });
            stars.push(`<button type="button" class="rating-star${i <= n ? ' is-on' : ''}" data-stars="${i}" aria-label="${escapeHtml(label)}" title="${escapeHtml(label)} (${i})">★</button>`);
        }
        return `<span class="rating-control" role="group" aria-label="${escapeHtml(t('Rating'))}" data-rating="${n}">${stars.join('')}</span>`;
    },

    // wire calls onRate with the stars chosen in a control within root.
    wire(root, onRate) {
        root.querySelectorAll('.rating-control').forEach(el => el.addEventListener('click', (e) => {
            const btn = e.target.closest('.rating-star');
            if (!btn) return;
            const n = Number(btn.dataset.stars);
            onRate(n === Number(el.dataset.rating) ? 0 : n);
        }));
    },

    // rate writes the stars of files (relative to the boundary) and says in
    // a toast what happened. It returns the files that took it.
    async rate(files, n) {
        if (!files.length) return [];
        let results;
        try {
            results = (await API.rateFiles(files, n)).results || [];
        } catch (err) {
            App.showToast(t('The rating was not written: {error}', { error: err.message }));
            return [];
        }
        const done = results.filter(r => r.success).map(r => r.file);
        const failed = results.filter(r => !r.success);
        const what = countText(done.length, 'photo');
        let text = n ? t('{stars} on {photos}.', { stars: countText(n, 'star'), photos: what }) : t('Rating removed from {photos}.', { photos: what });
        if (failed.length) text += ' ' + t('{n} did not take it: {error}', { n: formatCount(failed.length), error: failed[0].error });
        App.showToast(text);
        return done;
    },
};
