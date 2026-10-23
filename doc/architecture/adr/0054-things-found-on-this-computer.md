# ADR-0054: Things found on this computer, written into the sidecars as keywords

*Last modified: 2026-10-24*

## Status

Accepted.

## Context

People (ADR-0051) finds faces. On 2026-10-23 the owner asked to find
animals and things as well — bird, car, house, tree, nature, the sea — to
filter and search by them, to count them in the statistics, and later to
tell their pets apart (dog Charly, dog Gerard, cat Sally). What is found
should go into the sidecars, as names do, and into the index, so searching
is fast.

Decided with the owner: two models, one for words and scenes and one with
boxes for the pets to come; keywords in `dc:subject`; English; a fixed,
curated word list; search by any words in this version; a download of a few
hundred megabytes is acceptable; a search bar above every place.

## Decision

1. **Two models on the same ONNX Runtime as faces.** OpenAI's CLIP ViT-B/16
   (MIT), Xenova's ONNX export pinned to one Hugging Face commit, describes a
   photo by 512 numbers of unit length, and a phrase too; how close the two are
   says how well the phrase fits the photo. YOLOX-s from OpenCV's model zoo
   (Apache-2.0), at the zoo commit faces already use, puts a box around each of
   the 80 COCO classes. The image encoder is taken at full precision (345 MB):
   its 8-bit version chose another best word for 48 of 200 photos tried; the
   text encoder's 8-bit version (64 MB) agreed with the full one (cosine 0.9986).
   Together 447 MB, downloaded on request and checked as faces' models are
   (`toolinstall.InstallThings`), into `<lib-dir>/things`.

2. **ONNX Runtime is shared.** It moved from `recognition/` to
   `<lib-dir>/onnxruntime/`; a library there before is moved on start. Each
   install downloads it only when it is missing. `internal/onnx` gained int64
   tensors, for CLIP's token IDs, and reads each output by its element type.

3. **A fixed word list, each word with its own floor.** `things.Vocabulary`
   holds about 170 things in categories (animals, vehicles, buildings, nature,
   water, sky and weather, plants, food and drink, objects, activities,
   occasions). A word's text vector is the average of its prompts. CLIP's
   similarities of one photo to all words lie close together (about
   0.15–0.33), so a word is taken when it is above its floor (0.25, higher for
   words CLIP sees too readily) and within 0.02 of the photo's best word, at
   most five. An animal needs the detector's box or a clear 0.28. A detector
   box of 0.7 or more, covering 1 % of the photo, names its thing by itself.
   Measured on 200 of the owner's thumbnails, about four in five tags are
   right. `VocabularyVersion` names the list: when it changes, photos are
   scored again from their stored vectors, without the models.

4. **A pass beside the scans.** After the faces pass, at start and after the
   install, `Manager.FindThingsMissing` looks at the photos no current models
   and word list looked at, two at a time, as "Finding things in …" in the
   status line. It holds no index lock.

5. **The sidecar holds the things, the index follows it.** Following
   ADR-0052, the pass writes the sidecar first and the index after. Found
   things are written as keywords, `dc:subject`, where Lightroom, digiKam and
   Apple Photos read them, and into Unterlumen's own description as
   `ul:Things` (the keywords Unterlumen wrote), `ul:ThingsNot` (things the
   owner took away; no pass brings them back) and `ul:ThingsVersion`. The
   owner's own keywords are never removed or taken for found things. A
   keyword removed in another program takes its thing with it at the next
   scan. When the sidecar already holds what current models found — the other
   installation looked first — the pass takes that over instead of writing.
   A sidecar without the record leaves the index's things alone (it could not
   be written, as on a read-only share), unless the record was written once:
   then the sidecar was replaced and the photo is looked at again.
   **This is new**: until now a pass never wrote a sidecar, and only names were
   written at all (ADR-0051, decision 6). The first pass writes a sidecar
   beside every photo with things that had none.

6. **Keywords another program keeps are changed in its own element.** A
   sidecar holds one `dc:subject`. When another program keeps one, Unterlumen
   adds and removes its keywords in that list, element by element; otherwise
   they stand in Unterlumen's own `dc` description, beside the title. Writing
   a title follows the same rule. Before, a title written into a sidecar where
   another program keeps everything in one description replaced that whole
   description, rating and keywords included.

7. **Index.** `photo_vectors` (the CLIP vector, 2 KB a photo),
   `photo_things_found` (version, word list, whether the sidecar holds the
   record), `photo_things` (indexed by thing and photo) and `thing_boxes` (the
   detector's boxes, before the photo's EXIF orientation, as faces are; kept
   for the pets). The shared filter (ADR-0050) gained `thing`: one set per
   thing, never correlated, through its index (`TestThingFilterIsNotCorrelated`),
   so the Map, the Timeline and the Statistics follow it.

8. **Search by words.** `text` in the Libraries search turns the words into
   CLIP's vector ("a photo of …") and compares it with every photo's vector,
   kept in memory per library and read again when they changed (about 2 KB a
   photo). Photos within 0.035 of the best match of all libraries, never below
   0.22, at most 500, come best match first. Without things installed the
   search says so (409).

9. **One search above every place.** A bar across the top of the window,
   chosen by the owner over a field in the sidebar or in Libraries' head:
   typing suggests the people named and the things found whose names hold the
   letters, with their photos, and last "Photos that show '…'". A choice opens
   Libraries with its filter set — Person, Thing, or Shows — where it can be
   refined. `/` focuses it from anywhere. On a phone it stands above the place.

10. **A Things topic in the statistics**: things met most, things over time,
    things together, drawn with the People topic's charts, which take their
    words as an argument. Clicking a thing shows its photos; the filter's
    Things and the Map show where.

11. **Pets later, prepared now.** Telling dog Charly from dog Gerard will
    describe the crop of each dog, cat or horse box (`thing_boxes`) with
    CLIP, group them as faces are grouped, and write names as MWG regions of
    type `Pet`. No new pass over the photos is needed beyond the crops.

12. **CLIP despite its model card, SigLIP as the way out.** OpenAI's model
    card calls "any deployed use case" of CLIP out of scope; its MIT license
    permits it. Decided with the owner on 2026-10-24 to keep CLIP: it runs
    locally on the owner's own photos, with a fixed word list that names no
    sensitive traits, and every thing it finds can be taken away. Should this
    become a problem — a change of terms, a complaint, results that do not
    hold — Google's SigLIP base (Apache-2.0, no such clause) takes its place:
    a SentencePiece tokenizer in Go instead of CLIP's BPE, new floors from a
    calibration, about 485 MB, and a new `things.Version`, so every photo is
    looked at again.

## Consequences

- The download grows by 447 MB; the pass takes about 0.2–0.3 s a photo on
  an Apple M-series processor with two workers; the models take memory while
  loaded, and the vectors about 2 KB a photo for searching by words.
- Sidecars are written for photos that had none. A library on a share the
  app cannot write keeps its things in the index only.
- The word list is English and fixed. Its floors are data in `vocabulary.go`;
  changing them is a new `VocabularyVersion` and a re-scoring from the stored
  vectors, not a new pass.
- CLIP is a general model: it confuses things (a funicular taken for a boat)
  and misses some. Wrong things are taken away in the info panel, for good.
