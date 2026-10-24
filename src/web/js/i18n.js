// i18n.js — the language of the interface (ADR-0056): English, or German.
// Each browser chooses its own, in Settings; until it does, it follows the
// browser's language. A text is written in English and is its own key:
// t('Libraries') is "Bibliotheken" in German, and a text without a
// translation stays English rather than breaking. The words for a language
// load before every other script (i18n/<code>.js), so texts made while the
// scripts load are translated too. Changing the language loads the page
// again.

const I18N_KEY = 'ui.language';

const I18n = {
    // The languages the interface speaks, by code, in their own words.
    languages: { en: 'English', de: 'Deutsch' },

    // lang is the language the interface speaks now.
    lang: (() => {
        let chosen = null;
        try { chosen = localStorage.getItem(I18N_KEY); } catch { /* a private window */ }
        if (chosen && chosen !== 'auto') return chosen in { en: 1, de: 1 } ? chosen : 'en';
        return (navigator.language || 'en').toLowerCase().startsWith('de') ? 'de' : 'en';
    })(),

    // chosen is what this browser chose: a language code, or 'auto' to
    // follow the browser.
    chosen() {
        try { return localStorage.getItem(I18N_KEY) || 'auto'; } catch { return 'auto'; }
    },

    choose(code) {
        try { localStorage.setItem(I18N_KEY, code); } catch { /* not kept */ }
        location.reload();
    },

    words() {
        return (globalThis.I18N_WORDS || {})[this.lang] || {};
    },
};

// t translates an English text; {name} in it is filled from vars.
function t(text, vars) {
    // A text that means two things carries its sense in front, "verb|Name";
    // English shows what follows the bar.
    const out = I18n.words()[text] ?? text.slice(text.indexOf('|') + 1);
    return vars ? out.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m)) : out;
}

// translateStatic translates the page's own HTML: an element with
// data-i18n has its text translated, data-i18n-attr names attributes
// (aria-label, title, placeholder) to translate as well.
function translateStatic(root = document) {
    document.documentElement.lang = I18n.lang;
    for (const el of root.querySelectorAll('[data-i18n]')) el.textContent = t(el.textContent.trim());
    for (const el of root.querySelectorAll('[data-i18n-attr]')) {
        for (const attr of el.dataset.i18nAttr.split(/\s+/)) {
            if (el.hasAttribute(attr)) el.setAttribute(attr, t(el.getAttribute(attr)));
        }
    }
}
