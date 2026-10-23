# Suggestions that learn from names and No's

*Last modified: 2026-10-23*

## Summary

Naming faces and correcting them makes the next suggestions better, and a
refused suggestion does not come back. Asked by the owner on 2026-10-23
("das sorgt nicht für eine künftige bessere Erkennung?"); of the six ideas
offered, the owner chose the first three and ruled out the others for
good: listing named faces that may be wrong, prints for faces marked by
hand, and looking again for missed faces.

## Details

- **Compared with their own faces.** `faces.Resemblance`: the mean
  similarity of a person's nearest three faces. Measured on a copy of the
  largest library (12 554 named faces, 143 people, each face left out in
  turn): right 98.1 %, wrong 1.2 %, against 92.3 % and 4.8 % for the
  average of all their faces. Threshold 0.45 as before.
- **Fast enough for every photo.** The people are pre-selected by their
  averages (the five nearest) and compared face by face only then; the
  prints are cached per library until its confirmed faces change. On that
  library: groups 21 ms, a photo's faces 3 ms, Maybe for the person with
  3 173 faces 254 ms.
- **A No is kept**: `face_not (face, person)`, written by No beside a
  suggestion, "This is not Anna" and Is instead; naming the face that
  person takes it back. Local to the installation, like hiding.
- **Where suggestions show**: each group waiting for a name ("Is this
  Anna? No"), each unnamed face in a photo's info panel, and *Maybe Anna*
  on a person's page (at most 60, the likeliest first, with This is Anna
  and Not Anna for a selection).
- **Not in it**: naming anything without the owner.

## Acceptance Criteria

- [x] A face or group like a named person is suggested as them, compared with their nearest faces
- [x] No beside a suggestion, This is not and Is instead keep the face from being suggested as that person again
- [x] Naming the face that person after all takes the No back
- [x] Info panel: Is this Anna? under an unnamed face, Anna takes it, No refuses it
- [x] Person page: Maybe Anna lists unnamed faces like them; a selection is taken or refused
- [x] Measured on a copy of a real library: accuracy and times above
- [x] Go tests for suggestions and No's; e2e with real recognition (Maybe, No) and a mocked info panel test
