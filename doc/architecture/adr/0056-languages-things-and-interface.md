# ADR-0056: Languages — thing names, keywords and the interface

*Last modified: 2026-10-24*

## Status

Accepted. Builds on [ADR-0054](0054-things-found-on-this-computer.md) and
[ADR-0055](0055-siglip2-quality-and-languages.md).

## Context

Unterlumen was English throughout. Its owner works in German. Things are
found in English (ADR-0054). They become keywords in sidecars, and
Lightroom, Apple Photos and other programs read those keywords too.
SigLIP 2 searches by words in any language (ADR-0055). The thing names,
the keywords and the interface were not translated.

The owner asked for three things:
- German first, other languages later;
- a language switch in Settings that defaults to the browser's language;
- a clear choice of keyword language per library, because those keywords
  leave Unterlumen.

## Decision

1. **A thing keeps its ID.**
   - The ID is the English name from ADR-0054, such as `dog` or
     `half-timbered house`.
   - IDs are used in the index, `ul:Things` and `ul:ThingsNot`, URLs and the
     shared filter.
   - A language only changes what a person reads.

2. **The lexicon names the things per language**
   (`internal/things/lexicon/<code>.json`, embedded).
   - Each entry is `id: [name, synonym…]`, and each category has its own
     name.
   - Tests check that every thing and category has a name in every
     language, and that no name belongs to two things.
   - The web reads it from `GET /api/things/words?lang=` (`ThingWords`). The
     search bar suggests things from names and synonyms.
   - For a word that matches no name, `GET /api/things/nearest?text=` asks
     the text model which thing the word means. It answers only when the
     match is clear: similarity of at least 0.80 and at least 0.01 ahead of
     the second-best thing.

3. **Each library chooses its keyword language when it is made.**
   - The default is the interface's language.
   - The dialog says that found things become keywords in the `.xmp` files,
     where other programs read them, and that the language can be changed
     later.
   - The choice is the library's `keyword_language` property. A shared
     library also keeps it in `.unterlumen-library.json`, so every
     installation writes the same language and a joining installation
     adopts it.

4. **Changing the keyword language rewrites the keywords.**
   - The rewrite only touches sidecars and needs no model; it runs as a scan
     ("Writing keywords in …").
   - Unterlumen knows its own keywords by the IDs in `ul:Things`. It
     replaces the keywords any language wrote for those IDs and never
     touches the owner's own keywords.
   - When a scan reads a sidecar, it accepts the keyword of a thing in every
     language. A keyword removed in another program still removes its
     thing, whatever its language.

5. **The interface is written in English and translated with gettext-style
   keys.**
   - `t('English text', {vars})` returns the text in the chosen language;
     `{name}` placeholders are filled from `vars`.
   - The dictionary is `web/i18n/<lang>.js`, keyed by the English text and
     loaded as a script before every other one.
   - A text that means two things carries its sense in front of a bar,
     `t('verb|Name')`; English shows what follows the bar.
   - Shared helpers translate what they are given, so a call site passes
     English: place header, dialog, menu, toggle, segment field, panel
     button, `Activity`, `countText`/`countHTML`, the info panel's rows,
     `showToast`.
   - Plurals are whole sentences per number, never an English "s".
   - Static HTML uses `data-i18n` and `data-i18n-attr`.

6. **The language is chosen per browser.**
   - Settings → Language offers Browser, English and Deutsch, stored in
     `localStorage` as `ui.language`.
   - Browser follows `navigator.languages` and falls back to English.
   - Choosing a language reloads the page; nothing is redrawn in place.

7. **Only the interface is translated.**
   - The server words its jobs in English, a name in quotes
     (`Scanning "Archive"`). The status line translates the text with the
     name taken out (`Scanning "{name}"`) and puts the name back in.
   - The server's error messages stay English for now.
   - Photo data is never translated: EXIF values, file names, camera and
     film simulation names.

8. **The core places come first.**
   - 0.20.0 translates Libraries, Folders, Organize, Map, Timeline, People,
     Statistics, the info panel, the viewer, the search bar, the filter and
     Settings.
   - 0.20.1 translates the rest: Destinations, Galleries, Publish, Export,
     batch rename, the folder picker, About and its topics, the guide.
   - `TestEveryInterfaceTextHasAGermanWord` fails for any text passed to
     `t()` without a German entry. `TestGermanWordsKeepTheirPlaceholders`
     fails when a translation loses a placeholder.

## Consequences

- **Adding a language means writing two files:** `web/i18n/<code>.js` and
  `internal/things/lexicon/<code>.json` (read at start). The only code
  change is one entry in `I18n.languages` and the script tag in
  `index.html`.
- **Changing an English text changes its key.** Its German entry has to
  move with it, and the test says when one is missing.
- **Keywords in another language are a sidecar change**, written like every
  other change (ADR-0052). Other programs see them after their next read.
