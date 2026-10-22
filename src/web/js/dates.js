// dates.js — how a date or a time is written everywhere: ISO, to the
// second, without fractions or zone. The zone stays in All metadata.

// formatDateTime writes a timestamp as 2026-10-21 14:05:09. An ISO string
// keeps its own clock time, whatever zone it was taken in.
function formatDateTime(value) {
    if (!value) return '';
    const s = String(value);
    const m = s.match(/^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}(:\d{2})?)/);
    if (m && !/Z$/.test(s)) return `${m[1]} ${m[2]}`;
    const d = new Date(value);
    if (isNaN(d)) return s;
    return `${formatDay(d)} ${[d.getHours(), d.getMinutes(), d.getSeconds()].map(pad2).join(':')}`;
}

// formatDay writes the day of a date or timestamp as 2026-10-21.
function formatDay(value) {
    if (!value) return '';
    if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}/.test(value) && !/Z$/.test(value)) return value.slice(0, 10);
    const d = value instanceof Date ? value : new Date(value);
    if (isNaN(d)) return String(value);
    return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

function pad2(n) { return String(n).padStart(2, '0'); }
