# Interface in English and German

*Last modified: 2026-10-24*

## Summary

The interface can be shown in German as well as English, chosen in
Settings. By default it follows the browser's language. The owner asked for
this on 2026-10-24 as version 0.20 and chose the scope "core first, the rest
follows". Only the interface is translated; photo data is not. Decisions are
recorded in [ADR-0056](../../architecture/adr/0056-languages-things-and-interface.md).

## Details

- **Settings → Language** offers Browser, English and Deutsch.
  - The choice is kept per browser.
  - Choosing a language reloads the page.
- **Translation keys** are the English texts, gettext-style:
  - `t('English text', {vars})` looks the text up;
  - the German dictionary is `web/i18n/de.js`;
  - shared helpers (place header, dialogs, menus, Activity, counts, info
    panel rows) translate what they are given.
- **0.20.0 translates the core places:**
  - Libraries and a library;
  - Folders and Organize;
  - Map and Timeline;
  - People and Statistics;
  - the info panel and the viewer;
  - the search bar and the filter;
  - Settings, the sidebar and the tab bar.
- **0.20.x translates the rest:**
  - Destinations, Galleries and Publish;
  - Export and batch rename;
  - the slideshow's setup dialog;
  - About, the guide and the setup page;
  - texts sent by the server.
- **Never translated:**
  - EXIF values and file names;
  - camera, lens and film simulation names;
  - thing IDs in URLs and sidecars (their names are translated, ADR-0056).

## Acceptance criteria

- [x] Settings has a Language switch: Browser, English, Deutsch; Browser is the default
- [x] The core places read in German, with no English left on screen apart from data (checked on a throwaway instance)
- [x] Plurals and sentences with values are whole sentences per language, never an English "s"
- [x] `TestEveryInterfaceTextHasAGermanWord` and `TestGermanWordsKeepTheirPlaceholders`
- [x] English unchanged: the full e2e suite passes
- [x] e2e spec: switching to German and back
- [x] Destinations, Galleries, Publish, Export, batch rename, About, guide and setup in German (0.20.1)
- [x] The status line's job titles from the server in German (0.20.1); the server's error messages stay English
- [ ] Error messages from the server in German
