# Things: SigLIP 2, better recognition, any language

*Last modified: 2026-10-24*

## Summary

Replace OpenAI's CLIP with Google's SigLIP 2 for things and searching by
words. Recognition quality comes first, then search in any language.
Unterlumen must run this on a NAS alone, such as a Raspberry Pi 5. A Mac or
other fast hardware is an advantage, never a condition. Decided with the
owner on 2026-10-24, without a measurement beforehand.

Why:
- **Quality.** "helicopter" found photos that show nothing at all: lens
  flare, blur, near black, close-ups. CLIP only gives a similarity relative
  to other words, and on a photo without a subject some word always comes
  out best. SigLIP gives each word its own probability, so "3 % helicopter"
  is simply no.
- **Languages.** SigLIP 2 is multilingual (Gemma tokenizer). "Hund am Strand",
  "perro en la playa" and "浜辺の犬" search without translation, whole
  sentences included.
- **License.** SigLIP 2 is Apache-2.0, which settles the reservation about
  CLIP's model card (ADR-0054, decision 12).

## Details

### Model

- **Variant:** SigLIP 2 base, patch 16, 224 px, for every installation.
  - One model everywhere, so the vectors in the sidecars match between
    installations.
  - No smaller model for a NAS: B/32 would be about four times faster, but
    its vectors do not fit B/16's.
  - The 384 and 512 px variants are 3–5× slower and out of reach for a NAS.
- **Source:** `onnx-community/siglip2-base-patch16-224-ONNX` on Hugging Face,
  pinned to one commit (`ba1f3b0`), each file's SHA-256 checked. The model is
  `google/siglip2-base-patch16-224` (Apache-2.0).
- **Files:**

  | File | Precision | Size |
  |---|---|---|
  | `vision_model.onnx` | fp32 | 372 MB |
  | `text_model_int8.onnx` | int8 | 283 MB |
  | `tokenizer.model` (SentencePiece) | — | 4 MB |

  About 660 MB in all, against 447 MB today.
- **Quantization:** The image encoder stays fp32 until it is measured; CLIP's
  8-bit image encoder lost too much.
- **Tokenizer:** SentencePiece (unigram/BPE as Gemma uses it), written in Go,
  from `tokenizer.model`, with reference IDs in a test.
- **Scores:** `sigmoid(logit_scale · cos + logit_bias)` per word. The scale and
  bias come with the model.
- **Detector:** YOLOX stays for boxes, for the pets to come.

### Recognition quality

- **Threshold:** Each thing is taken from a probability threshold, not from
  similarity relative to the best word. Floors per word stay possible, as
  data.
- **Quality gate:** The appearance pass already measures brightness, contrast
  and structure. A photo that is nearly black, burnt out or without
  structure gets no things, or only above a much higher threshold.
- **Learning from removals:** Every × in the info panel is `ul:ThingsNot` in
  the sidecar. Per thing and library, these removals raise its threshold, as
  face suggestions learn from No's.
- **Calibration:** on the owner's thumbnails, as in the first spike (the
  `calibrate_test.go` pattern).

### Running on a NAS

- One things pass at a time, already done in 0.18.2.
- The pass runs at low priority with limited threads (ONNX Runtime
  intra-op threads; a nice value on Linux), so browsing, Samba and other
  services come first.
- Input is only the thumbnail, never the original.
- **Whoever looks at a photo first runs the model** and writes things and
  vector into the sidecar. The other installation takes them over without
  the model (`ul:ThingsVector`, 0.18.2).
- **Estimates, not measured:**
  - Pi 5: 2–4 s a photo; a first pass over 48 000 photos alone takes one to
    two days.
  - Memory while loaded: 1–1.5 GB, workable with 8 GB.
- **Switching:** a new `things.Version`.
  - Every photo is looked at again once. On the Mac that takes hours; the
    NAS takes the vectors over.
  - The record's `ul:ThingsVersion` names the model, so CLIP vectors are
    never compared with SigLIP's.

### Languages

- **Things are kept by their ID** — the English names of today (`dog`,
  `half-timbered house`) — in the index, `ul:Things`/`ul:ThingsNot`, URLs and
  the shared filter.
- **Display names per UI language** come from the lexicon next to the word
  list (`internal/things/lexicon/<lang>.json`), with synonyms for the
  search bar's suggestions. A test checks that every ID has a name in every language.
- **Keyword language per library**, chosen when it is made.
  - The default is the UI language.
  - A sentence says plainly that found things are written as keywords into
    the sidecars, which Lightroom, Apple Photos and others read too.
  - The choice is kept in `.unterlumen-library.json`, so all installations
    of a shared library write the same language.
- **Rewriting later** is a pass that changes sidecars only, without models.
  Unterlumen knows its own keywords by the IDs in `ul:Things`; the owner's
  own keywords are never touched.
- **Reading keywords:** A scan maps every keyword back to its ID through the
  dictionaries of all languages, so a keyword removed in another program
  still removes its thing.
- **Searching by words** needs no translation: SigLIP 2's text encoder
  understands the languages.
- **UI translation** is a feature of its own
  (2026-10-24-interface-languages, ADR-0056). German comes first.

## Acceptance criteria

- [x] SigLIP 2 base B/16 224 px installed on request, files pinned and checked, credits and Your data updated (tokenizer.json, 34 MB, instead of tokenizer.model: plain BPE, no protobuf)
- [x] Gemma tokenizer in Go with reference IDs from Hugging Face's tokenizer (five languages, emoji, double spaces)
- [x] Things from per-word probabilities; photos without a subject get no things (helicopter case: only the helicopters left)
- [x] Quality gate — as things of their own (blurred, underexposed, overexposed), from SigLIP rather than the appearance measures; chart "Photo quality over time"
- [ ] Removals raise a thing's threshold per library
- [x] New `things.Version`: every photo looked at again once; vectors travel; the NAS takes them over without the model
- [ ] Pass at low priority with limited threads; the NAS stays usable during a first pass
- [x] Word search in German, Spanish, French, Japanese and Chinese finds the photos the English phrase finds (Japanese and Chinese weaker)
- [x] Thing IDs stable; display names per language (German); synonyms in the search bar; the nearest thing for a word that names none
- [x] Keyword language chosen per library when it is made, with a plain explanation; kept in the library's marker
- [x] Rewriting a library's keywords into another language without models (ADR-0056)
- [x] ADR-0055 (model change and quality); ADR-0054 decision 12 closed
- [ ] Go tests, e2e (things spec), legal check of the new download
