-- Three payment-timeline concepts and one broadening, from bouncing the
-- vocabulary against the corpus (2026-08-22).
--
-- Partial payments: trap in one state, waiver in another — Illinois lets a
-- landlord take a partial payment during the notice period and still evict,
-- Chicago says knowingly accepting rent waives the termination, Seattle
-- routes payments to the noticed amount first.
--
-- Rent debt collection: what happens to the DEBT, distinct from
-- eviction-record (the court file): collectors, credit and screening
-- reports, the federal collection limits, "you do not pay the judgment on
-- the spot".
--
-- Grace period: how long after the due date before rent counts as late at
-- all — a different clock from the pay-or-quit window (late-rent-notice).
-- Today the corpus states it inside late-fee statements, so this starts
-- dark until those claims are split into their own statements.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('partial-payments', 'Partial payments',
     (SELECT id FROM topics WHERE slug = 'cant-pay-rent'),
     'What happens when you pay part of the rent, or your landlord accepts a partial payment.'),
    ('rent-debt-collection', 'Rent debt collection',
     (SELECT id FROM topics WHERE slug = 'cant-pay-rent'),
     'What happens to unpaid rent after you move or lose in court: collectors, credit reports, and screening reports.'),
    ('grace-period', 'Grace period',
     (SELECT id FROM topics WHERE slug = 'cant-pay-rent'),
     'How many days after the due date before rent counts as late.');

-- hotline-211 broadens into help lines: cities run their own advice and
-- referral lines beside 211 (the Philly Tenant Hotline, NYC's 311 Tenant
-- Helpline, Chicago's MTO line), and three phone-line concepts for "who do
-- I call" is taxonomy, not help. Boundaries stay clean: help lines advise
-- and refer, complaint lines enforce, legal aid represents. Slug moves with
-- the name, per the legal-aid precedent: the page is days old and a
-- name/slug mismatch is the confusion the renames exist to end.
UPDATE concepts
   SET slug = 'help-lines', name = 'Help lines',
       definition = 'Free phone lines that help renters, like 211 or a city tenant hotline.'
 WHERE slug = 'hotline-211';
