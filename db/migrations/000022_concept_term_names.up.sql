-- Concept display names switch to the legal terms of art where one exists.
--
-- Migration 000019 gave concepts plain-language names on the theory that all
-- site text follows the editorial voice. But concept names are
-- authoring-facing only — renters never see them (slugs are the public
-- anchors, and those do not change here) — and the author doing the tagging
-- thinks in the terms of art. "Peaceful use of your home" made the reviewer
-- stop and wonder whether it was quiet enjoyment; a tag that needs decoding
-- fails at its one job. Plain names stay where no term of art exists.
UPDATE concepts SET name = 'Quiet enjoyment'                    WHERE slug = 'quiet-enjoyment';
UPDATE concepts SET name = 'Warranty of habitability'           WHERE slug = 'habitability-standard';
UPDATE concepts SET name = 'Notice to quit'                     WHERE slug = 'notice-to-quit';
UPDATE concepts SET name = 'Pay or quit notice'                 WHERE slug = 'late-rent-notice';
UPDATE concepts SET name = 'Pay and stay (redemption)'          WHERE slug = 'pay-and-stay';
UPDATE concepts SET name = 'Answer (responding in court)'       WHERE slug = 'answer-the-case';
UPDATE concepts SET name = 'No self-help eviction (court required)' WHERE slug = 'court-eviction-only';
UPDATE concepts SET name = 'Illegal lockout'                    WHERE slug = 'illegal-lockout';
UPDATE concepts SET name = 'Fair housing (discrimination)'      WHERE slug = 'fair-housing';
UPDATE concepts SET name = 'Retaliation'                        WHERE slug = 'retaliation-protection';
UPDATE concepts SET name = 'Rent control / rent stabilization'  WHERE slug = 'rent-control';
UPDATE concepts SET name = 'Rent increase notice'               WHERE slug = 'increase-notice-period';
UPDATE concepts SET name = 'Rent withholding'                   WHERE slug = 'rent-withholding';
UPDATE concepts SET name = 'Notice of entry'                    WHERE slug = 'entry-notice-period';
UPDATE concepts SET name = 'Right of entry (allowed reasons)'   WHERE slug = 'entry-allowed-reasons';
UPDATE concepts SET name = 'Eviction record sealing'            WHERE slug = 'eviction-record';
UPDATE concepts SET name = 'Security deposit cap'               WHERE slug = 'deposit-cap';
UPDATE concepts SET name = 'Deposit receipt'                    WHERE slug = 'deposit-receipt';
UPDATE concepts SET name = 'Deposit escrow and interest'        WHERE slug = 'deposit-escrow-interest';
UPDATE concepts SET name = 'Deposit return deadline'            WHERE slug = 'deposit-return-deadline';
UPDATE concepts SET name = 'Itemized deduction statement'       WHERE slug = 'deduction-itemization';
UPDATE concepts SET name = 'Deposit penalty damages (2x/3x)'    WHERE slug = 'deposit-damages';
UPDATE concepts SET name = 'Code inspection'                    WHERE slug = 'code-inspection';
