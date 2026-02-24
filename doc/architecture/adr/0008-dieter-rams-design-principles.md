# ADR-0008: Dieter Rams' Ten Principles of Good Design

*Last modified: 2026-02-21*

## Status

Accepted

## Context

The application needs a visual and interaction design language. Without a deliberate design philosophy, UIs tend to accumulate visual noise, inconsistent controls, and decorative elements that don't serve the user.

Dieter Rams' ten principles of good design, developed during his tenure at Braun (1961–1995), provide a clear, timeless framework. The visual language of Braun products from that era — the SK4 record player, T3 pocket radio, TP1 portable, ET calculators — is characterized by restraint, clarity, and functional honesty.

## Decision

Adopt Dieter Rams' ten principles as the governing design philosophy for the application's UI:

1. **Innovative** — Use the medium's strengths (browser rendering, CSS grid) rather than imitating native apps.
2. **Useful** — Every element serves the task of browsing and culling photos. No decorative chrome.
3. **Aesthetic** — Warm, neutral palette inspired by Braun products: off-white surfaces, warm grays, functional orange accents.
4. **Understandable** — Controls are self-explanatory. Labels over icons. No hidden gestures.
5. **Unobtrusive** — The UI recedes; the photos are the focus. Minimal visual weight on controls.
6. **Honest** — No faux textures, no skeuomorphism, no animations that don't convey state.
7. **Long-lasting** — Neutral palette and clean typography that won't feel dated.
8. **Thorough** — Consistent spacing, alignment, and typography down to every element.
9. **Environmentally friendly** — Minimal resource usage: no framework, no build step, small payload.
10. **As little design as possible** — Remove until it breaks. Every remaining element must justify its existence.

### Visual language

- **Palette**: Off-white background (#f5f2ed), warm grays for text (#3a3a3a, #7a7a7a), functional orange (#d35400) for primary actions and active states
- **Typography**: System sans-serif at restrained sizes, medium weight for hierarchy
- **Controls**: Simple rectangular buttons with subtle borders, no rounded corners beyond 2-3px, no gradients
- **Spacing**: Generous, consistent whitespace derived from an 8px grid
- **Photos**: Presented without ornament — no drop shadows, no borders, clean grid

## Consequences

- **Distinctive appearance** — The app will look noticeably different from typical dark-themed developer tools.
- **Light theme** — Braun products were predominantly light-colored. The app moves from a dark theme to a light one.
- **Restraint required** — New features must pass the "does this element earn its place?" test before adding visual complexity.
- **Accessibility** — The warm neutral palette with dark text provides strong contrast ratios. Orange accents must meet WCAG AA against the background.
