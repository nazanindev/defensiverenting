# ADR-019 — One statement card, one standing, groupings instead of pages

| | |
|---|---|
| Status | Accepted, shipped 2026-09-12 |
| Date | 2026-09-12 |

## Context

ADR-018 made the statement the unit of review, and the portal grew a surface per feature: the page view stamped, the editor attested quotes, the queue decided notes, a source page rechecked, a concept page stamped again. A statement could be acted on from five pages and each offered a different subset of the actions. Finishing one statement meant visiting three of them.

The storage had three vocabularies for one question, "is this statement ready to publish?": the gate's issues, ADR-014's proposals, ADR-018's stamps and notes. The UI mirrored the storage.

## Decision

### D1. One standing per statement, computed in one place

`CitedStatement.Standing(pageLevel)` returns Ready, Needs you, or Blocked, with reason codes: empty, uncited, quote-missing, quote-unconfirmed, source-blocked, statute-locator, note, proposal, unread. Blocked means an unconfirmed quote sits on a source the checker cannot read; only a person who opens it can clear it. It is computed in Go from the statement alone, so every surface reads the same answer. The gate in `issues.go` enforces the same conditions per page in SQL; a test holds the two together: a page passes the gate exactly when every statement is Ready.

### D2. One card, everywhere

The `stmtcard` template renders a statement with its standing, its quotes and their confirmation state, its pending notes with Fine / Fix in editor / Fixed, an attest action when blocked, and a Reviewed stamp action when the stamp would be accepted. The page view, the grouped list, and any future surface render this card and nothing else for a statement. Its forms carry a return target as two whitelisted fields, never a path.

### D3. Groupings, not pages

`/statements?by=source|concept|note` is one list with a group-by control. By source confirms a quote once for every page citing it, and carries the two source-only actions on the group header: recheck and attest-all. By concept reads one claim across jurisdictions. By note is what the drafting agent was unsure of. The page view is the same card grouped by page. The ADR-018 review index, source page, and concept page are deleted.

### D4. The dashboard row is a worklist row

Each draft shows ready over total and the counts behind the gap: unread, notes, quotes, blocked, proposed, broken. It is aggregated from the same standing rule, so the row and the cards agree. "Publish N ready" stays.

### D5. Four verbs

Confirm a quote. Decide a note. Review a statement. Publish a page. The UI uses these and retires "verify & publish", "mark reviewed", "attest", "resolved in editor" as labels; attestation survives as the mechanism behind "Attest quotes I found" on a blocked card.

## Consequences

- Portal pages for review work: dashboard, statements list, page view, editor, queue. The queue keeps proposals that carry replacement text, which need the old-beside-new layout, and still lists notes as an index; notes are decided on the card.
- A statement is actionable wherever it appears.
- The checker scope tightened to draft and live pages, and tolerates a statement leaving every page mid-run.
- The statement queries now carry `SourceUnreadable` per citation and `ProposalPending` per statement; the public page query pays for two more columns it does not render.
