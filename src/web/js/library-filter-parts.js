// library-filter-parts.js — pieces of the library filter that need nothing
// of the panel: the date-taken fields, the chips' namespaces and values, and
// what the chips add to a query.

// The two days of "Date taken". A text field, so the day reads 2026-10-21
// as everywhere else, not in the browser's own order.
function dateTakenFilter(min, max, onChange) {
    const el = document.createElement('div');
    el.className = 'lib-filter-groups';
    el.innerHTML = `<div class="lib-filter-group lib-filter-group--date"><div class="lib-filter-label">${t('Date taken')}</div></div>`;
    const group = el.firstElementChild;
    const from = dateRow(group, t('From'), min, onChange.from);
    const to = dateRow(group, t('To'), max, onChange.to);
    return { el, from, to };
}

function dateRow(group, labelText, value, onChange) {
    const row = document.createElement('div');
    row.className = 'lib-date-row';
    row.innerHTML = `<span class="lib-date-row-label">${escapeHtml(labelText)}</span>`;
    const input = document.createElement('input');
    Object.assign(input, { type: 'text', inputMode: 'numeric', placeholder: t('YYYY-MM-DD'), autocomplete: 'off', className: 'lib-date-input', value });
    input.addEventListener('change', () => {
        const v = input.value.trim();
        const ok = v === '' || (/^\d{4}-\d{2}-\d{2}$/.test(v) && !isNaN(new Date(v)));
        input.toggleAttribute('aria-invalid', !ok);
        input.title = ok ? '' : t('Write the day as 2026-10-21');
        if (ok) onChange(v);
    });
    // Escape blurs without changing value, restoring keyboard navigation.
    input.addEventListener('keydown', (e) => { if (e.key === 'Escape') input.blur(); });
    row.appendChild(input);
    group.appendChild(row);
    return input;
}

// The chips' namespaces: the fixed ones, then every EXIF field of the index
// that no slider or fixed namespace covers, then the fields people added.
function chipNamespaces(exifFields, metaKeys) {
    const fixedParams = new Set(CHIP_NS_FIXED.filter(n => n.type === 'exif').map(n => n.param));
    const nss = [...CHIP_NS_FIXED];
    const add = (ns) => { if (!nss.some(n => n.ns === ns.ns)) nss.push(ns); };
    for (const field of exifFields) {
        if (field === 'DateTaken' || field === 'DateTimeOriginal' || SLIDER_FIELDS.has(field) || fixedParams.has(field)) continue;
        add({ ns: field, label: field, hint: t('EXIF field'), type: 'exif', param: field });
    }
    for (const key of metaKeys) {
        if (!key.startsWith('built:')) add({ ns: key, label: key, hint: t('Custom tag'), type: 'meta', param: 'meta_' + key });
    }
    return nss;
}

// The values a namespace offers: from the filter's own lists, the index's
// EXIF values, or the fields people gave.
async function chipValues(nsDesc, ns, sources, ids) {
    if (nsDesc && sources[nsDesc.ns]) return sources[nsDesc.ns]();
    if (nsDesc?.type === 'exif') {
        const raw = await LibraryAPI.exifValues(nsDesc.param, ids).catch(() => []);
        return labeledExifValues(nsDesc.param, raw);
    }
    return LibraryAPI.metaValues(ns, ids).catch(() => []);
}

// Several people or things narrow together: photos with all of them.
function addChipParams(params, chips) {
    for (const chip of chips) {
        const p = chipToParam(chip);
        if (!p) continue;
        if (p.key === 'person' || p.key === 'thing') (params[p.key] ||= []).push(p.value);
        else params[p.key] = p.value;
    }
}
