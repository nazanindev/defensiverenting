-- Every concept gets a short definition. The reference layer derived blurbs
-- from national statements, which left every term without a published
-- national definition blurbless on /terms. A definition is registry metadata
-- like the name: a gloss of what the term means, deliberately free of any
-- jurisdictional claim — the law stays in statements, where it is cited and
-- checked. Written in the editorial voice; authored here, edited by
-- migration, like everything else in the registry.
ALTER TABLE concepts ADD COLUMN definition TEXT NOT NULL DEFAULT '';

UPDATE concepts SET definition = 'When a landlord punishes you for using a legal right. The law forbids it.' WHERE slug = 'retaliation-protection';
UPDATE concepts SET definition = 'Laws that ban housing discrimination based on race, family status, disability, and more.' WHERE slug = 'fair-housing';
UPDATE concepts SET definition = 'Only a court can order you out of your home. A landlord acting alone cannot.' WHERE slug = 'court-eviction-only';
UPDATE concepts SET definition = 'A landlord forcing you out by changing locks or cutting utilities. It is illegal.' WHERE slug = 'illegal-lockout';
UPDATE concepts SET definition = 'The written warning you must get before a late-rent eviction case. It gives you time to pay.' WHERE slug = 'late-rent-notice';
UPDATE concepts SET definition = 'Programs that help pay rent when money runs short.' WHERE slug = 'rent-assistance-programs';
UPDATE concepts SET definition = 'A letter telling you to move out by a set date. An eviction case can only start after it.' WHERE slug = 'notice-to-quit';
UPDATE concepts SET definition = 'Paying all the rent you owe to stop an eviction case. Many places allow it up to a deadline.' WHERE slug = 'pay-and-stay';
UPDATE concepts SET definition = 'Your written response to an eviction lawsuit. Filing it protects your right to be heard.' WHERE slug = 'answer-the-case';
UPDATE concepts SET definition = 'The public court record an eviction case leaves. Some places let you seal it.' WHERE slug = 'eviction-record';
UPDATE concepts SET definition = 'How much advance warning your landlord owes you before coming in.' WHERE slug = 'entry-notice-period';
UPDATE concepts SET definition = 'The reasons the law lets your landlord enter, like repairs or showings.' WHERE slug = 'entry-allowed-reasons';
UPDATE concepts SET definition = 'A landlord may enter right away in a true emergency, like a fire or a burst pipe.' WHERE slug = 'emergency-entry';
UPDATE concepts SET definition = 'What a landlord owes you for entering illegally.' WHERE slug = 'entry-penalties';
UPDATE concepts SET definition = 'When you or your landlord may change the locks.' WHERE slug = 'lock-change-rules';
UPDATE concepts SET definition = 'Laws that cap how much rent can go up.' WHERE slug = 'rent-control';
UPDATE concepts SET definition = 'During a fixed lease term, your rent cannot go up unless you agree.' WHERE slug = 'mid-lease-protection';
UPDATE concepts SET definition = 'The written warning your landlord owes you before rent goes up.' WHERE slug = 'increase-notice-period';
UPDATE concepts SET definition = 'Your landlord''s duty to keep your home safe and fit to live in.' WHERE slug = 'habitability-standard';
UPDATE concepts SET definition = 'Asking for repairs in writing, so there is proof of when you asked.' WHERE slug = 'repair-request-in-writing';
UPDATE concepts SET definition = 'Holding back rent until your landlord fixes a serious problem. Legal only in some places.' WHERE slug = 'rent-withholding';
UPDATE concepts SET definition = 'Paying for a repair yourself and subtracting the cost from rent. Legal only in some places.' WHERE slug = 'repair-and-deduct';
UPDATE concepts SET definition = 'Asking a city inspector to check unsafe conditions in your home.' WHERE slug = 'code-inspection';
UPDATE concepts SET definition = 'The most a landlord can charge as a security deposit.' WHERE slug = 'deposit-cap';
UPDATE concepts SET definition = 'Written proof that you paid your deposit.' WHERE slug = 'deposit-receipt';
UPDATE concepts SET definition = 'Where your deposit must be kept, and the interest it may earn.' WHERE slug = 'deposit-escrow-interest';
UPDATE concepts SET definition = 'How long a landlord has to give your deposit back after you move out.' WHERE slug = 'deposit-return-deadline';
UPDATE concepts SET definition = 'The written list of charges a landlord must send when keeping part of a deposit.' WHERE slug = 'deduction-itemization';
UPDATE concepts SET definition = 'The penalty a landlord pays for wrongly keeping a deposit. Often 2 or 3 times the amount.' WHERE slug = 'deposit-damages';
UPDATE concepts SET definition = 'Free lawyers and legal aid for renters who qualify.' WHERE slug = 'free-legal-help';
UPDATE concepts SET definition = 'A free helpline. Dial 211 to find rent help and services near you.' WHERE slug = 'hotline-211';
UPDATE concepts SET definition = 'Free or low cost advice from HUD approved housing counselors.' WHERE slug = 'housing-counseling';
UPDATE concepts SET definition = 'Your right to live in your home without your landlord disturbing you.' WHERE slug = 'quiet-enjoyment';
UPDATE concepts SET definition = 'A low cost court for money disputes. You do not need a lawyer.' WHERE slug = 'small-claims-court';
UPDATE concepts SET definition = 'Keeping photos, letters, and receipts as proof for court.' WHERE slug = 'records-and-evidence';
