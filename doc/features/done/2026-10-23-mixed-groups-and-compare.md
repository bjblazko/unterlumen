# Mixed groups, comparing people, hidden faces, shared decisions

*Last modified: 2026-10-23*

## Summary

Two requests of the owner on 2026-10-23. A group waiting for a name is
often one person with a face or two of someone else: naming it meant
opening the group, selecting the right faces, typing the suggested name
again, and finding the smaller group in the list afterwards. And the
People topic of Statistics should compare people chosen freely, not only
those with the most photos.

## Details

- **A group's page** shows the suggestion at the top ("Is this Paula?
  Paula / No") with the name already in the field; both act on the
  selected faces, or on all. After naming, hiding or a No, the page stays
  with the faces left — and their own suggestion — and goes back to the
  list once the group is empty. Faces left out get no No of their own.
- **Compare**, in the People topic's head beside Periods: chips of the
  people chosen (up to eight, × leaves one out) and "Add someone…"; empty
  it reads "The most photos" and the charts are as before. Who appears
  most, People over time, Photos by age, Who with whom and Life lines show
  the people compared, in that order and colours. Kept in this browser
  (`stats.people.compare`); a change redraws without asking the server.
- **Hidden faces** are listed from Waiting for a name ("N faces hidden",
  `#people/name/<library>/hidden`) and shown again; a frame drawn by hand
  over a hidden face names it instead of being refused.
- **Hidden faces of all libraries** are listed from People's first page
  (`#people/hidden`), each shown again in its own library.
- **Hiding, No's and covers are shared** through `face-decisions.json` in
  the shared folder and followed by every index (ADR-0051, decision 15);
  the first sharing puts each installation's earlier decisions in.
- **The suggestion** reads "Is this **Anna**? Yes · No": the name in the
  sentence, two answers of one size beside it.
- Also: the bars of Who appears most are scaled by the largest shown, as
  compared people need not come largest first; No beside a suggestion has
  room of its own.

## Acceptance Criteria

- [x] A group's page starts from its suggestion; the name is filled in
- [x] Naming the selected faces keeps the page with the faces left; an empty group leads back to the list
- [x] No on the group's page refuses the suggestion for the selected faces, or all
- [x] Compare chooses up to eight people; the five charts show those, in that order
- [x] Nobody compared is the default as before; the choice outlives a reload
- [x] Hidden faces are listed and shown again; a frame over one names it
- [x] People's first page lists the hidden faces of all libraries
- [x] Hiding, No's and covers follow to the other installation; earlier decisions are kept; an unreachable shared folder shows nothing again
- [x] e2e: group page stays and starts from the suggestion (mocked and real recognition); Compare
