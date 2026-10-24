# ADR-0055: Things found by SigLIP 2: quality first, any language

*Last modified: 2026-10-24*

## Status

Accepted. Supersedes the choice of CLIP in
[ADR-0054](0054-things-found-on-this-computer.md) (decisions 1 and 3) and
closes its decision 12.

## Context

CLIP gives a photo's similarity to each word, not a probability. On a photo
without a subject — lens flare, blur, near black, a pocket shot — every
word is about as unlikely, and the best one came out as a thing.
"helicopter" found ten such photos among the helicopters, and the owner put
recognition quality first, before any search by phrases. CLIP understands
English only, while the UI is to come in Spanish, French, German, Japanese
and Chinese. And OpenAI's model card calls any deployed use out of scope.

The owner asked for no measurement beforehand. The thresholds below were
set on the 200 thumbnails of the first spike and on the 40 photos CLIP had
called "helicopter".

## Decision

1. **SigLIP 2 base, patch 16, 224 px**, for every installation (Apache-2.0),
   from the ONNX export at `onnx-community/siglip2-base-patch16-224-ONNX`,
   pinned:

   | File | Precision | Size |
   |---|---|---|
   | image encoder | fp32 | 372 MB |
   | text encoder | int8 | 283 MB |
   | `tokenizer.json` | — | 34 MB |

   Vectors have 768 numbers. One model everywhere, so that vectors travel
   between installations. B/32 would be four times faster but cannot be
   compared with B/16, and 384 or 512 px is out of reach for a NAS.
   `things.Version` is 2: every photo is looked at once again.

2. **Gemma's tokenizer in Go**, from `tokenizer.json`:
   - byte-level BPE over the whole text in lower case;
   - spaces as "▁";
   - bytes as fallback;
   - the end mark, then padding to 64.

   Symbols are kept as IDs, so the 580 000 merges take little memory. A test
   checks it against Hugging Face's tokenizer in five languages.

3. **Things from probabilities.** `sigmoid(112.67 · cos − 16.77)` per word,
   with the scale and bias read from the model's weights. A thing is taken
   when its probability is at least 1 % and a tenth of the photo's best
   thing, at most five. The detector still adds what it is sure of. SigLIP's
   probabilities are low in absolute terms but rank well; on the sample,
   almost every thing taken was right, at the cost of more photos with
   none. Animals need no rule of their own any more.

4. **Photo quality is a category of things:** blurred, underexposed,
   overexposed (1.5 %).
   - A photo whose most likely thing is its quality gets only its
     qualities: it shows nothing clearly.
   - Qualities are filtered, searched and counted like any thing.
   - They have a chart of their own in the Things topic, and the other
     charts leave them out.

5. **Searching by words** takes the words as typed, in any language
   SigLIP 2's text encoder knows. There is no translation and no English
   "a photo of" in front.

6. **Older vectors** are not compared:
   - the search reads only vectors of photos looked at by the current
     version;
   - a scan takes a sidecar's vector only from a current record;
   - `Similarity` gives 0 for vectors of different lengths.

7. **The download replaces the one before.** The things page says so when
   older models are there (`outdated`), and that a NAS takes a day or two
   for a large library the first time.

## Consequences

- Fewer things per photo, nearly all of them right. A word that should be
  found more often gets its own `Min`.
- A NAS alone looks at a large library for a day or two the first time;
  with a faster installation sharing the library, it takes things and
  vectors over.
- The keyword language per library, display names per UI language and
  rewriting a library's keywords are next (feature document
  2026-10-24-things-siglip2-and-languages). Thing IDs stay the English
  names of ADR-0054.
