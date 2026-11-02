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
    if (s.error) return `<tr>${name}<td colspan="2" class="meta-preview-skip">${escapeHtml(s.error)}</td></tr>`;
    if (s.plan?.skipped) {
        return `<tr>${name}<td colspan="2" class="meta-preview-skip">${t('Left as it is: {reason}.', { reason: t(s.plan.skipped) })}</td></tr>`;
    }
    const before = s.current?.fields?.dateTaken?.value;
    const after = s.plan?.set?.dateTaken;
    const date = after
        ? `<span class="meta-preview-before">${escapeHtml(formatDateTime(before) || t('None'))} →</span> <strong>${escapeHtml(formatDateTime(after))}</strong>`
        : escapeHtml(formatDateTime(before) || '');
    const names = changedFields.filter(f => f !== 'dateTaken').map(f => t(META_FIELD_NAMES[f]));
    if (keywords) names.push(t('Keywords'));
    return `<tr>${name}<td class="meta-preview-date">${date}</td><td>${escapeHtml(names.join(', '))}</td></tr>`;
}
