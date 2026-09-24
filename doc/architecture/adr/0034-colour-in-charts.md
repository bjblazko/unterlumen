# ADR-0034: Colour in charts

*Last modified: 2026-09-24*

## Status

Accepted. Extends [ADR-0030](0030-rams-design-tokens.md) (rams-design tokens).

## Context

ADR-0030 spends colour sparingly: roughly 90 % neutral, 9 % structure, 1 %
signal, and each signal colour has exactly one meaning — orange is the one
primary action, green confirmed, yellow time passing, red needs attention.

The statistics contradict that. Fourteen charts drew with fixed hex values
("warm-gray palette"), the film simulations carried invented colours, dark
mode was an afterthought, and `chartColors().border` read `var(--border)` —
a token ADR-0030 renamed a year ago, so the axes were drawing with an empty
colour string.

The question is not whether charts may use colour. It is what the colour is
allowed to mean.

## Decision

**In a chart, colour encodes data.** That is information, not decoration, so
it is allowed — but it must not borrow the meanings the interface has already
spent.

1. **Charts have their own ramp.** Eight families in a fixed order, as
   `--chart-1 … --chart-8`, designed twice (light and dark), never flipped.
   Slot N is the same entity in both themes. The ramp is validated against
   the six checks of the dataviz method — lightness band, chroma floor, CVD
   separation under protanopia and deuteranopia, a normal-vision floor, and
   contrast against the chart surface — with
   `dataviz/scripts/validate_palette.js`. Both modes pass all six.
2. **The signal colours stay with the interface.** Orange, green, yellow and
   red appear in a chart only with the meaning they have everywhere else
   (missing data in `--warning`), never as the next category. The ramp keeps
   its distance from the accent's saturation, so a mark never reads as
   something to press.
3. **The form follows the data.** One series takes slot 1 and needs no
   legend — the title names it. Several series take slots 1..N in order,
   never cycled. Magnitude (a calendar heatmap, the shooting clock) takes the
   sequential ramp `--chart-seq-1 … 5`, one hue in steps, rather than fading
   one colour with opacity.
4. **Never colour alone.** Every category carries its name, as a direct label
   or in a legend.
5. **Film simulations are the exception.** Their colour shows the look they
   are named after — Velvia punchy, Acros grey, Eterna cool. That is the
   information, so they get their own `--film-*` tokens in both themes rather
   than a slot in the ramp. As a palette they fail the categorical checks by
   construction (several film looks *are* grey), which is legal here because
   the film chart is a labelled bar chart: the name is always beside the bar,
   and colour never has to carry the difference on its own.

Anything outside the statistics that encodes a category with colour follows
the same rule — the batch-rename token chips, for example, take chart slots
instead of four more fixed hexes.

## Consequences

- `stats-modal.js` reads every colour from tokens; the dead `--border` access
  is gone and the axes draw again.
- Dark mode is designed, not derived: the dark steps are their own values,
  validated against the dark surface.
- A ninth category is not a new colour. It folds into "Other", or the chart
  becomes small multiples.
- Changing the accent no longer changes what the charts look like.
- The film tokens are a deliberate deviation from the dataviz checks and are
  recorded as such here; they may only be used where the label sits next to
  the mark.
