// segment-field.js — a place's only setting, in its head rather than
// behind ⋯: its name, then its values as one segment, the current one
// pressed (Map: Grey · Colour, Timeline: Rows 2 · 3 · 4). On a phone the
// head has one row, so the same setting is a small select there, named by
// the setting.

// segmentField returns the element; options are [value, text] pairs and
// onChange gets the value chosen.
function segmentField({ label, options, value, onChange }) {
    const el = document.createElement('div');
    el.className = 'segment-field';
    el.innerHTML = `<span class="segment-field-label">${escapeHtml(t(label))}</span>
        <div class="seg" role="group" aria-label="${escapeHtml(t(label))}">${options.map(([v, text]) =>
            `<button type="button" data-value="${escapeHtml(String(v))}" aria-pressed="${String(v) === String(value)}">${escapeHtml(t(text))}</button>`).join('')}</div>
        <select class="btn btn-sm select-btn segment-field-select" aria-label="${escapeHtml(t(label))}">${options.map(([v, text]) =>
            `<option value="${escapeHtml(String(v))}"${String(v) === String(value) ? ' selected' : ''}>${escapeHtml(t(text))}</option>`).join('')}</select>`;
    const select = el.querySelector('select');
    const choose = (v) => {
        for (const b of el.querySelectorAll('button[data-value]')) b.setAttribute('aria-pressed', String(b.dataset.value === v));
        select.value = v;
        onChange(v);
    };
    el.addEventListener('click', (e) => {
        const btn = e.target.closest('button[data-value]');
        if (btn && btn.getAttribute('aria-pressed') !== 'true') choose(btn.dataset.value);
    });
    select.addEventListener('change', () => choose(select.value));
    return el;
}
