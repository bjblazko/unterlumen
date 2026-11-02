// metadata-preview.js — what the metadata editor will write to each file
// (ADR-0060): its date before and after, the other fields that change, or
// why it is left as it is. The server works it out, so the preview and the
// writing never disagree.

function metadataPreviewHTML(states, { changedFields, keywords }) {
    if (!states.length) return '';
    const rows = states.map(s => metadataRowHTML(s, changedFields, keywords)).join('');
    return `<table class="meta-preview">
        <thead><tr><th>${t('File')}</th><th>${t('Date taken')}</th><th>${t('Changes')}</th></tr></thead>
        <tbody>${rows}</tbody></table>`;
}

function metadataRowHTML(s, changedFields, keywords) {
    const thumb = `<img class="meta-preview-thumb" src="${escapeHtml(API.thumbnailURL(s.file))}" alt="" loading="lazy">`;
    const name = `<td class="meta-preview-name" title="${escapeHtml(s.file)}">${thumb}<span>${escapeHtml(s.file.split('/').pop())}</span></td>`;
    if (s.error) return `<tr>${name}<td colspan="2" class="meta-preview-error">${escapeHtml(s.error)}</td></tr>`;
    if (s.plan?.skipped) {
        return `<tr>${name}<td colspan="2" class="meta-preview-skip">${t('Left as it is: {reason}.', { reason: t(s.plan.skipped) })}</td></tr>`;
    }
    const before = s.current?.fields?.dateTaken?.value;
    const after = s.plan?.set?.dateTaken;
    const date = after
        ? `<span class="meta-preview-before">${escapeHtml(formatDateTime(before) || t('None'))} →</span> <strong>${escapeHtml(formatDateTime(after))}</strong>`
        : escapeHtml(formatDateTime(before) || '');
    const names = changedNames(s.plan, changedFields, keywords);
    return `<tr>${name}<td class="meta-preview-date">${date}</td><td>${escapeHtml(names.join(', '))}</td></tr>`;
}

// changedNames are the fields a file gets: from its plan when the server
// worked one out — a list or an own field may leave a file as it is —
// otherwise what was changed in the form.
function changedNames(plan, changedFields, keywords) {
    let fields = changedFields;
    if (plan) {
        fields = Object.keys(plan.set || {});
        if (plan.own && Object.keys(plan.own).length) fields.push('own');
        keywords = Boolean(plan.keywordsAdd?.length || plan.keywordsRemove?.length);
    }
    const names = fields.filter(f => f !== 'dateTaken' && !(f === 'countryCode' && plan?.set?.country))
        .map(f => f === 'country' && plan?.set?.country ? countryText(plan.set) : t(META_FIELD_NAMES[f] || f));
    if (keywords) names.push(t('Keywords'));
    return names;
}

// countryText is the country a file gets, as "Country: Deutschland (DE)".
function countryText(set) {
    const code = set.countryCode ? ` (${set.countryCode})` : '';
    return `${t('Country')}: ${set.country}${code}`;
}
