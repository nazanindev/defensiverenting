DROP TABLE IF EXISTS coverage_records;

ALTER TABLE playbook_statements DROP COLUMN IF EXISTS stage;

ALTER TABLE playbooks DROP CONSTRAINT IF EXISTS playbooks_page_kind_check;
ALTER TABLE playbooks ADD CONSTRAINT playbooks_page_kind_check
    CHECK (page_kind IN ('playbook', 'directory', 'faq', 'checklist'));

UPDATE concepts c SET topic_id = t.id FROM (VALUES
    ('late-rent-notice',           'cant-pay-rent'),
    ('illegal-lockout',            'renting-fundamentals'),
    ('court-eviction-only',        'renting-fundamentals'),
    ('utility-shutoff-protection', 'heat-not-working'),
    ('constructive-eviction',      'constructive-eviction'),
    ('fair-housing',               'renting-fundamentals')
) AS v(slug, topic) JOIN topics t ON t.slug = v.topic
WHERE c.slug = v.slug;

DELETE FROM concepts c
 WHERE c.slug IN ('nonrefundable-fees', 'deposit-increase', 'move-in-condition-report', 'holding-deposit',
                  'deposit-last-month-rent', 'deposit-after-sale', 'move-out-inspection', 'forwarding-address',
                  'move-out-charges', 'refused-rent', 'rent-receipt', 'subsidized-rent-change', 'duty-to-mitigate',
                  'early-termination-rights', 'default-judgment', 'eviction-appeal', 'time-to-move-after-judgment',
                  'abandoned-property', 'right-to-counsel', 'end-of-tenancy-notice', 'just-cause-eviction',
                  'foreclosure-tenant-protection', 'landlord-unpaid-utilities', 'energy-assistance', 'heat-requirement',
                  'ac-requirement', 'cooling-device-rights', 'mold', 'pests-bed-bugs', 'lead-paint', 'smoke-co-detectors',
                  'condemnation-relocation', 'casualty-damage', 'refuse-entry', 'landlord-harassment', 'added-fees',
                  'utility-billing', 'emergency-price-gouging', 'reasonable-accommodation', 'assistance-animal-rules',
                  'source-of-income', 'fair-housing-complaint', 'domestic-violence-protections', 'application-fees',
                  'tenant-screening', 'criminal-history-screening')
   AND NOT EXISTS (SELECT 1 FROM statements s WHERE s.concept_id = c.id);

ALTER TABLE concepts DROP COLUMN IF EXISTS question, DROP COLUMN IF EXISTS position, DROP COLUMN IF EXISTS national_only;

DELETE FROM topics t
 WHERE t.rules_for IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM playbooks pb WHERE pb.topic_id = t.id);
DELETE FROM topics t
 WHERE t.slug IN ('breaking-lease', 'locked-out', 'utility-shutoff', 'building-sold', 'move-out-bill',
                  'assistance-animal', 'rental-application')
   AND NOT EXISTS (SELECT 1 FROM playbooks pb WHERE pb.topic_id = t.id)
   AND NOT EXISTS (SELECT 1 FROM concepts c WHERE c.topic_id = t.id);

UPDATE topics SET name = 'Heat Not Working' WHERE slug = 'heat-not-working';
UPDATE topics SET is_core = false WHERE slug IN ('lease-renewal', 'discrimination', 'heat-not-working');
INSERT INTO topics (slug, name, is_core) VALUES
    ('noise-complaints', 'Noise Complaints', false),
    ('rent-stabilization', 'Rent Stabilization', false)
ON CONFLICT (slug) DO NOTHING;

ALTER TABLE topics DROP COLUMN IF EXISTS stages, DROP COLUMN IF EXISTS rules_for, DROP COLUMN IF EXISTS national_only;
