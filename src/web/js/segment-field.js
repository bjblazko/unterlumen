// segment-field.js — a place's only setting, in its head rather than
// behind ⋯: its name, then its values as one segment, the current one
// pressed (Map: Grey · Colour, Timeline: Rows 2 · 3 · 4).

// segmentField returns the element; options are [value, text] pairs and
// onChange gets the value chosen.
function segmentField({ label, options, value, onChange }) {
    const el = document.createElement('div');
    el.className = 'segment-field';
    el.innerHTML = `<span class="segment-field-label">${escapeHtml(label)}</span>
        <div class="seg" role="group" aria-label="${escapeHtml(label)}">${options.map(([v, text]) =>
            `<button type="button" data-value="${escapeHtml(String(v))}" aria-pressed="${String(v) === String(value)}">${escapeHtml(text)}</button>`).join('')}</div>`;
    el.addEventListener('click', (e) => {
        const btn = e.target.closest('button[data-value]');
        if (!btn || btn.getAttribute('aria-pressed') === 'true') return;
        for (const b of el.querySelectorAll('button[data-value]')) b.setAttribute('aria-pressed', String(b === btn));
        onChange(btn.dataset.value);
    });
    return el;
}
