# ADR-032 — A guide looks like a handbook, and a source is written as a citation

| | |
|---|---|
| Status | Accepted 2026-10-09 |
| Date | 2026-10-09 |
| Amends | ADR-006 D7 (kind of source is now said in words, not by a colored label with a glyph), ADR-031 (the first screen keeps its order, with a new look) |

## Context

Nazanin's editor, 2026-10-09, on the public site: it should "look less AI and not like our competitor", and "only an AI would use the word 'chip'". Nazanin on the source labels: they "look like type labels", not like links or citations.

What made the site read as machine-made, on the live Pennsylvania deposit guide:

- A gradient page background with two blurred color blobs behind the content.
- Every statement in a white rounded card with a drop shadow, fading in on load.
- Sources as colored pills, each with an emoji or glyph (§, 🏛, ⚖) and a prefix word (Org., Ed.), rounded like buttons. A pill reads as a category tag. It does not read as "this sentence comes from this document".
- Uppercase small labels (TIP, NATIONWIDE), a pill-shaped place button in the header, and the Fraunces plus Space Grotesk font pairing, all common in generated sites.

The competitor site is a dense sans-serif database of typed facts (ADR-030). Looking unlike it means looking like something written for a person to read.

## Decision

### D1. One serif, flat color, plain shapes

- One typeface for everything a reader reads: Source Serif 4, with Georgia as the fallback. The dark header and footer bars use the system sans. No display face.
- One flat off-white page. No gradient, no blobs, no drop shadows, no load animation.
- Color is flat blocks and simple shapes: a filled circle for a statement number, a short colored bar before a stage heading, a tinted box for a warning or a tip, a filled button. Corners are small (3 to 8px), never pill-shaped, except where a control is a real button.
- Two accent colors. The brand oxblood stays for the header, links, the first stage and the main button. A deep teal is the second stage color. Stages alternate between the two, on the heading bar and the number circles, so a reader sees where one stage ends.
- Warnings are amber with a drawn triangle. Tips are pale green with a drawn bulb. Icons are inline stroke drawings, never emoji, and always sit beside the words they decorate.

### D2. A source is a citation line, not a label

Under each statement, a line that begins "Source:" or "Sources:" and lists each source, numbered, one per line when there are several:

> Sources:
> 1. [68 P.S. § 250.512(a), Pennsylvania General Assembly]. Law.
> 2. [Consumer guide, p. 11, Pennsylvania Office of Attorney General]. A government guide.
> 3. [Our editorial rules].

- The link text is the locator followed by the publisher when there is a locator, else the publisher alone. The whole citation is the link, so it reads and taps as one.
- The kind of source follows in plain words, after a full stop: Law. An agency rule. A government guide. A court decision. A nonprofit group. The editorial source links to the editorial page as "Our editorial rules" and carries no kind, since the link text already says what it is.
- Nothing is carried by color, glyph or prefix. ADR-006 D7's rule, that color is never the only cue, is kept by having no cue but words.
- A tip or a page note that cites the editorial source ends with "From our editorial rules." in the same small text.
- The words chip, badge and pill leave the reader-facing copy. The editorial page describes the source line instead. Code names change in their own pass.

### D3. The rest of the guide

- Above the title, a small line in sans: the place and the topic ("Pennsylvania · Security deposits"). Below the title, the not-legal-advice block stays the filled oxblood block (ADR-031 D3), now without the info icon.
- The page-level warning (ADR-016 A3) is the amber box under the place line.
- Published-by, sources-checked, "Tell us" and "Did this page help?" share one bordered box at the foot of the guide, with a drawn check mark. Yes is the filled button.
- "More tenant rights in {place}" is a list of bordered rows with a chevron, so each reads as tappable on a phone. The same row style serves the topic lists on hubs and the homepage.
- The header keeps the logo and the reader's place (ADR-031 D5). The place sits in a soft translucent block, not a pill.

## Consequences

- Every page shares the base: fonts, colors, header, footer, flat surfaces. Pages not redrawn here (homepage, hubs, locations, concept, forms) take the base and keep their own layout. Their remaining rounded inputs and buttons are the next pass.
- The source line is longer than a pill. A statement with three sources takes three lines of small text. That is the trade for reading as a citation.
- Spanish strings exist for every new reader-facing word (kind of source, "From", "Source"/"Sources"), since the UI string table carries both languages even while Spanish content is parked (ADR-015).

## Later

- Citations per sentence rather than per statement: ADR-033, proposed, its page changes deferred.
- Rounded search inputs and pill buttons on the homepage, hubs and forms.
- Rename `CitationChip`, `chipClass` and the `.chip` test file in code.

## Rejected

- **Footnote numbers in the text with one Sources list at the foot of the page.** Reads most like a reference work, but citations attach to statements, not sentences, so every number would repeat and each source costs a jump down the page and back. Revisit with ADR-033.
- **Keeping a color per kind of source.** It was the thing that made a source look like a category tag.
- **A tinted band behind the title.** Tried on the mockup, 2026-10-09. Nazanin: the red block should be the disclaimer, not the band.
