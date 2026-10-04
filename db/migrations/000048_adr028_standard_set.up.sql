-- The standard set, rules pages, concept questions, stages and coverage
-- records. ADR-028.
--
-- Rules pages are topics of their own (decided 2026-10-04, ADR-028 build
-- note): "security-deposit-rules" beside "security-deposits". A slot holds
-- one page per place, topic and language, and every save, publish and
-- revision path finds a page by that slot, so a rules page on its own topic
-- needs none of them changed and can never overwrite the situation page.

-- ---- Topics -----------------------------------------------------------------

ALTER TABLE topics
    -- A national-only topic has one page, on united-states. Seeding,
    -- coverage and the place picker skip it per state (D6).
    ADD COLUMN national_only BOOLEAN NOT NULL DEFAULT false,
    -- A rules topic points at the situation topic whose concepts it answers
    -- (D4). NULL for every situation topic.
    ADD COLUMN rules_for BIGINT REFERENCES topics(id),
    -- The fixed stage headings for a playbook on this topic, in order (D10).
    -- Empty for rules pages (question headings) and checklists.
    ADD COLUMN stages TEXT[] NOT NULL DEFAULT '{}';

-- New situation topics, then names and the standard flag for all of them.
INSERT INTO topics (slug, name, is_core) VALUES
    ('breaking-lease',     'Breaking a Lease',        true),
    ('locked-out',         'Locked Out',              true),
    ('utility-shutoff',    'Utility Shutoffs',        true),
    ('building-sold',      'Building Sold',           true),
    ('move-out-bill',      'Bill After Moving Out',   true),
    ('assistance-animal',  'Assistance Animals',      false),
    ('rental-application', 'Applying for a Rental',   false)
ON CONFLICT (slug) DO NOTHING;

-- is_core now means standard for every state (D2).
UPDATE topics SET is_core = true
 WHERE slug IN ('lease-renewal', 'discrimination', 'heat-not-working');
UPDATE topics SET name = 'Heat or AC Not Working' WHERE slug = 'heat-not-working';

UPDATE topics SET national_only = true, is_core = false
 WHERE slug IN ('move-in-checklist', 'move-out-checklist', 'rental-application',
                'assistance-animal', 'renting-fundamentals');

-- Stage lists (D10), from docs/ADR-028-stages-and-questions.md.
UPDATE topics t SET stages = v.stages FROM (VALUES
    ('cant-pay-rent',            ARRAY['What the law says', 'Talk to your landlord', 'Get help paying rent', 'If you still cannot pay']),
    ('eviction-defense',         ARRAY['The notice', 'Paying to stop the case', 'Going to court', 'After the court decides']),
    ('security-deposits',        ARRAY['What the law says', 'Ask for your deposit back', 'If your landlord does not pay', 'Going to court']),
    ('repairs-and-habitability', ARRAY['What your landlord has to fix', 'Ask in writing', 'Get your home inspected', 'If it is still not fixed', 'Moving out']),
    ('heat-not-working',         ARRAY['What the law says', 'Tell your landlord', 'Stay safe while you wait', 'If it is still not fixed']),
    ('landlord-entry',           ARRAY['When your landlord can come in', 'Your locks', 'If your landlord comes in without notice']),
    ('rent-increase',            ARRAY['How much notice you get', 'Check the new rent', 'If the increase is not allowed']),
    ('resource-directory',       ARRAY['Free legal help', 'Help lines', 'Help with rent and housing']),
    ('breaking-lease',           ARRAY['When you can leave early', 'What you may still owe', 'How to leave', 'After you move out']),
    ('lease-renewal',            ARRAY['How much notice you get', 'When your landlord needs a reason', 'Answer the notice', 'If your landlord goes to court']),
    ('locked-out',               ARRAY['Only a court can make you leave', 'Get back into your home', 'Get your things back', 'What a court can make your landlord do']),
    ('utility-shutoff',          ARRAY['If your landlord cut them off', 'If your landlord did not pay the bill', 'If you could not pay', 'Help paying the bill']),
    ('building-sold',            ARRAY['Your lease after a sale', 'Who to pay rent to', 'Your deposit', 'Foreclosure (when the bank takes the building)']),
    ('move-out-bill',            ARRAY['What your landlord can charge for', 'Check the bill', 'Tell your landlord you disagree', 'If you are sued or a debt collector calls']),
    ('discrimination',           ARRAY['What is against the law', 'Asking for a reasonable accommodation (a change because of a disability)', 'File a complaint', 'What happens after you file']),
    ('assistance-animal',        ARRAY['What the law says', 'Ask your landlord', 'If your landlord says no'])
) AS v(slug, stages) WHERE t.slug = v.slug;

-- Rules topics, one per situation topic that is home to concepts.
INSERT INTO topics (slug, name, is_core, rules_for)
SELECT v.slug, v.name, false, (SELECT id FROM topics WHERE slug = v.situation)
FROM (VALUES
    ('rent-payment-rules',       'Rent Payment Rules',           'cant-pay-rent'),
    ('eviction-rules',           'Eviction Rules',               'eviction-defense'),
    ('security-deposit-rules',   'Security Deposit Rules',       'security-deposits'),
    ('repair-rules',             'Repair Rules',                 'repairs-and-habitability'),
    ('heat-and-ac-rules',        'Heat and AC Rules',            'heat-not-working'),
    ('landlord-entry-rules',     'Landlord Entry Rules',         'landlord-entry'),
    ('rent-increase-rules',      'Rent Increase Rules',          'rent-increase'),
    ('lease-breaking-rules',     'Lease-Breaking Rules',         'breaking-lease'),
    ('lease-renewal-rules',      'Lease Renewal Rules',          'lease-renewal'),
    ('lockout-rules',            'Lockout Rules',                'locked-out'),
    ('utility-shutoff-rules',    'Utility Shutoff Rules',        'utility-shutoff'),
    ('building-sale-rules',      'Building Sale Rules',          'building-sold'),
    ('discrimination-rules',     'Housing Discrimination Rules', 'discrimination'),
    ('rental-application-rules', 'Rental Application Rules',     'rental-application')
) AS v(slug, name, situation)
ON CONFLICT (slug) DO NOTHING;

-- Deleted topics (D2). Guarded: a topic that somehow holds a page or a
-- concept is left alone rather than cascading anything away.
DELETE FROM topics t
 WHERE t.slug IN ('noise-complaints', 'rent-stabilization')
   AND NOT EXISTS (SELECT 1 FROM playbooks pb WHERE pb.topic_id = t.id)
   AND NOT EXISTS (SELECT 1 FROM concepts c WHERE c.topic_id = t.id)
   AND NOT EXISTS (SELECT 1 FROM statements s WHERE s.topic_ref = t.id);

-- ---- Pages --------------------------------------------------------------------

ALTER TABLE playbooks DROP CONSTRAINT IF EXISTS playbooks_page_kind_check;
ALTER TABLE playbooks ADD CONSTRAINT playbooks_page_kind_check
    CHECK (page_kind IN ('playbook', 'directory', 'faq', 'checklist', 'rules'));

-- A statement's stage on its page (D10). On the link row, not the
-- statement, so it stays out of the review hash: moving a step under a
-- different heading does not change the claim.
ALTER TABLE playbook_statements ADD COLUMN stage TEXT NOT NULL DEFAULT '';

-- ---- Concepts -----------------------------------------------------------------

ALTER TABLE concepts
    -- The renter's question the concept answers (ADR-020 D1). A rules page
    -- shows it as the heading with " in {Place}" added, so it is written to
    -- take the place at the end.
    ADD COLUMN question TEXT NOT NULL DEFAULT '',
    -- Order on a rules page, within the home topic.
    ADD COLUMN position INT NOT NULL DEFAULT 0,
    -- A national concept is answered once, by federal law, so no state
    -- rules page lists it as a gap (subsidized-rent-change, D3).
    ADD COLUMN national_only BOOLEAN NOT NULL DEFAULT false;

-- The 46 new concepts (D3). Name is the short label the authoring portal
-- and the concept page heading use; question is the renter's.
INSERT INTO concepts (slug, name, topic_id) SELECT v.slug, v.name, t.id
FROM (VALUES
    ('nonrefundable-fees',          'Fees you do not get back',             'security-deposits'),
    ('deposit-increase',            'Raising the deposit',                  'security-deposits'),
    ('move-in-condition-report',    'Move-in condition report',             'security-deposits'),
    ('holding-deposit',             'Holding deposit',                      'security-deposits'),
    ('deposit-last-month-rent',     'Deposit as last month''s rent',        'security-deposits'),
    ('deposit-after-sale',          'Deposit after a sale',                 'security-deposits'),
    ('move-out-inspection',         'Inspection before move-out',           'security-deposits'),
    ('forwarding-address',          'New address for the deposit',          'security-deposits'),
    ('move-out-charges',            'Charges beyond the deposit',           'security-deposits'),
    ('refused-rent',                'Landlord refuses rent',                'cant-pay-rent'),
    ('rent-receipt',                'Rent receipts',                        'cant-pay-rent'),
    ('subsidized-rent-change',      'Rent share when income drops',         'cant-pay-rent'),
    ('duty-to-mitigate',            'Landlord must look for a new renter',  'breaking-lease'),
    ('early-termination-rights',    'Ending a lease early',                 'breaking-lease'),
    ('default-judgment',            'Default judgment',                     'eviction-defense'),
    ('eviction-appeal',             'Appealing an eviction',                'eviction-defense'),
    ('time-to-move-after-judgment', 'Time to move after losing',            'eviction-defense'),
    ('abandoned-property',          'Things left behind',                   'eviction-defense'),
    ('right-to-counsel',            'Free lawyer in eviction court',        'eviction-defense'),
    ('end-of-tenancy-notice',       'Notice to end a month-to-month lease', 'lease-renewal'),
    ('just-cause-eviction',         'Just cause to make you leave',         'lease-renewal'),
    ('foreclosure-tenant-protection','Renters in a foreclosure',            'building-sold'),
    ('landlord-unpaid-utilities',   'Landlord did not pay the utility bill','utility-shutoff'),
    ('energy-assistance',           'Help paying energy bills',             'utility-shutoff'),
    ('heat-requirement',            'Heat and hot water',                   'heat-not-working'),
    ('ac-requirement',              'Air conditioning',                     'heat-not-working'),
    ('cooling-device-rights',       'Your own air conditioner',             'heat-not-working'),
    ('mold',                        'Mold',                                 'repairs-and-habitability'),
    ('pests-bed-bugs',              'Bed bugs, roaches and mice',           'repairs-and-habitability'),
    ('lead-paint',                  'Lead paint',                           'repairs-and-habitability'),
    ('smoke-co-detectors',          'Smoke and carbon monoxide alarms',     'repairs-and-habitability'),
    ('condemnation-relocation',     'Condemned home',                       'repairs-and-habitability'),
    ('casualty-damage',             'Fire or flood',                        'repairs-and-habitability'),
    ('refuse-entry',                'Saying no to entry',                   'landlord-entry'),
    ('landlord-harassment',         'Harassment',                           'landlord-entry'),
    ('added-fees',                  'New fees during a lease',              'rent-increase'),
    ('utility-billing',             'Billing for utilities',                'rent-increase'),
    ('emergency-price-gouging',     'Price gouging in a disaster',          'rent-increase'),
    ('reasonable-accommodation',    'Reasonable accommodation',             'discrimination'),
    ('assistance-animal-rules',     'Service and support animals',          'discrimination'),
    ('source-of-income',            'Housing vouchers',                     'discrimination'),
    ('fair-housing-complaint',      'Filing a discrimination complaint',    'discrimination'),
    ('domestic-violence-protections','Domestic violence',                   'discrimination'),
    ('application-fees',            'Application fees',                     'rental-application'),
    ('tenant-screening',            'Tenant screening reports',             'rental-application'),
    ('criminal-history-screening',  'Criminal records',                     'rental-application')
) AS v(slug, name, topic) JOIN topics t ON t.slug = v.topic
ON CONFLICT (slug) DO NOTHING;

UPDATE concepts SET national_only = true WHERE slug = 'subsidized-rent-change';

-- New homes for existing concepts (D3). Slugs never change.
UPDATE concepts c SET topic_id = t.id FROM (VALUES
    ('late-rent-notice',           'eviction-defense'),
    ('illegal-lockout',            'locked-out'),
    ('court-eviction-only',        'locked-out'),
    ('utility-shutoff-protection', 'utility-shutoff'),
    ('constructive-eviction',      'breaking-lease'),
    ('fair-housing',               'discrimination')
) AS v(slug, topic) JOIN topics t ON t.slug = v.topic
WHERE c.slug = v.slug;

-- Questions and rules-page order for every concept. Existing questions come
-- from docs/topic-map.md, rewritten where they broke the heading rule
-- (site-copy.md rule 10-11); new ones from docs/ADR-028-stages-and-questions.md.
UPDATE concepts c SET question = v.q, position = v.pos FROM (VALUES
    -- cant-pay-rent
    ('grace-period',             10, 'Is there a grace period before rent is late?'),
    ('late-fees',                20, 'How much can my landlord charge for late rent?'),
    ('partial-payments',         30, 'Can I pay part of my rent?'),
    ('refused-rent',             40, 'What if my landlord will not take my rent?'),
    ('rent-receipt',             50, 'Do I get a receipt when I pay rent?'),
    ('rent-assistance-programs', 60, 'Where can I get help paying rent?'),
    ('subsidized-rent-change',   70, 'If the government helps pay my rent, can my part go down when I earn less?'),
    ('rent-debt-collection',     80, 'What happens to rent I still owe?'),
    -- eviction-defense
    ('late-rent-notice',            10, 'How much notice does my landlord have to give when rent is late?'),
    ('notice-to-quit',              20, 'How much notice do I get before my landlord can take me to court?'),
    ('pay-and-stay',                30, 'Can I pay and stop an eviction?'),
    ('answer-the-case',             40, 'What do I do when I get eviction court papers?'),
    ('right-to-counsel',            50, 'Can I get a free lawyer if I am being evicted?'),
    ('eviction-court-process',      60, 'What happens in eviction court?'),
    ('default-judgment',            70, 'What happens if I get a default judgment (I lose because I missed my court date)?'),
    ('eviction-appeal',             80, 'Can I appeal (ask a different court to look at my case again) after I lose an eviction case?'),
    ('time-to-move-after-judgment', 90, 'How long do I have to move out after I lose in eviction court?'),
    ('abandoned-property',         100, 'What happens to things I leave behind when I move out?'),
    ('eviction-record',            110, 'Will an eviction stay on my record, and can I get it removed?'),
    -- security-deposits
    ('deposit-cap',              10, 'How much can my landlord charge for a deposit?'),
    ('nonrefundable-fees',       20, 'Can my landlord charge nonrefundable fees (money I will not get back) when I move in?'),
    ('holding-deposit',          30, 'Do I get back a holding deposit (money I paid to hold an apartment)?'),
    ('deposit-receipt',          40, 'Do I get a receipt for my deposit?'),
    ('move-in-condition-report', 50, 'Does my landlord have to give me a condition report (a list of damage already there) when I move in?'),
    ('deposit-escrow-interest',  60, 'Does my deposit earn interest, and where is it kept?'),
    ('deposit-increase',         70, 'Can my landlord raise my deposit?'),
    ('deposit-last-month-rent',  80, 'Can I use my deposit to pay my last month''s rent?'),
    ('deposit-after-sale',       90, 'Who gives my deposit back if the building is sold?'),
    ('move-out-inspection',     100, 'Can I ask my landlord to check the home before I move out?'),
    ('forwarding-address',      110, 'Do I have to give my landlord my new address to get my deposit back?'),
    ('deposit-return-deadline', 120, 'How long does my landlord have to return my deposit?'),
    ('deduction-itemization',   130, 'What can my landlord take out of my deposit?'),
    ('move-out-charges',        140, 'Can my landlord bill me for more than my deposit?'),
    ('deposit-damages',         150, 'What can I get if my landlord keeps my deposit unfairly?'),
    -- repairs-and-habitability
    ('habitability-standard',     10, 'What does my landlord have to keep working?'),
    ('repair-request-in-writing', 20, 'How do I ask for repairs?'),
    ('mold',                      30, 'Does my landlord have to get rid of mold?'),
    ('pests-bed-bugs',            40, 'Who has to get rid of bed bugs, roaches, or mice?'),
    ('lead-paint',                50, 'Does my landlord have to tell me about lead paint?'),
    ('smoke-co-detectors',        60, 'Does my landlord have to put in smoke and carbon monoxide alarms?'),
    ('code-inspection',           70, 'Who can inspect my home?'),
    ('rent-withholding',          80, 'Can I stop paying rent until repairs are made?'),
    ('repair-and-deduct',         90, 'Can I fix it myself and take it off the rent?'),
    ('condemnation-relocation',  100, 'What help do I get if my home is condemned (the city says no one can live there)?'),
    ('casualty-damage',          110, 'What happens to my lease after a fire or flood?'),
    -- heat-not-working
    ('heat-requirement',      10, 'How warm does my landlord have to keep my home, and do I get hot water?'),
    ('ac-requirement',        20, 'Does my landlord have to fix the air conditioning?'),
    ('cooling-device-rights', 30, 'Can I put in my own air conditioner?'),
    -- landlord-entry
    ('entry-notice-period',   10, 'How much notice does my landlord have to give before coming in?'),
    ('entry-allowed-reasons', 20, 'When can my landlord enter?'),
    ('emergency-entry',       30, 'Can my landlord enter in an emergency?'),
    ('refuse-entry',          40, 'Can I say no when my landlord wants to come in?'),
    ('entry-penalties',       50, 'What can I do if my landlord comes in when they should not?'),
    ('lock-change-rules',     60, 'Can I change the locks on my home?'),
    ('quiet-enjoyment',       70, 'Do I have a right to quiet enjoyment (to live in my home in peace)?'),
    ('landlord-harassment',   80, 'What can I do about harassment (my landlord bothering or threatening me)?'),
    -- rent-increase
    ('increase-notice-period',  10, 'How much notice do I get before my rent goes up?'),
    ('mid-lease-protection',    20, 'Can my rent go up during my lease?'),
    ('rent-control',            30, 'Is there a limit on rent increases where I live?'),
    ('added-fees',              40, 'Can my landlord add new fees during my lease?'),
    ('utility-billing',         50, 'Can my landlord charge me for water, gas, or electric?'),
    ('emergency-price-gouging', 60, 'Do price gouging laws (rules against raising prices a lot during a disaster) cover my rent?'),
    -- breaking-lease
    ('early-termination-rights', 10, 'When can I end my lease early without paying the rest?'),
    ('constructive-eviction',    20, 'Can I move out because my home is not safe to live in?'),
    ('duty-to-mitigate',         30, 'If I move out early, does my landlord have to look for a new renter?'),
    -- lease-renewal
    ('end-of-tenancy-notice', 10, 'How much notice does my landlord have to give to end a month-to-month lease?'),
    ('just-cause-eviction',   20, 'Does my landlord need just cause (a reason the law accepts) to make me leave?'),
    -- locked-out
    ('court-eviction-only', 10, 'Can my landlord evict me without going to court?'),
    ('illegal-lockout',     20, 'What if my landlord locks me out?'),
    -- utility-shutoff
    ('utility-shutoff-protection', 10, 'Can my utilities (water, electric, gas) be shut off?'),
    ('landlord-unpaid-utilities',  20, 'What if my landlord does not pay the water, gas, or electric bill?'),
    ('energy-assistance',          30, 'Where can I get help paying for heat and electricity?'),
    -- building-sold
    ('foreclosure-tenant-protection', 10, 'What happens to my lease in a foreclosure (when the bank takes the building from the owner)?'),
    -- discrimination
    ('fair-housing',                  10, 'What discrimination is against the law?'),
    ('source-of-income',              20, 'Can a landlord turn me down because I use a housing voucher?'),
    ('reasonable-accommodation',      30, 'Can I ask for a reasonable accommodation (a change in the rules because of a disability)?'),
    ('assistance-animal-rules',       40, 'Can I keep a service or support animal if pets are not allowed?'),
    ('domestic-violence-protections', 50, 'What rights do I have as a renter if I face domestic violence?'),
    ('fair-housing-complaint',        60, 'How do I report a landlord for treating me unfairly?'),
    -- rental-application
    ('application-fees',           10, 'How much can a landlord charge me to apply?'),
    ('tenant-screening',           20, 'What can a tenant screening report (a background check on renters) show a landlord?'),
    ('criminal-history-screening', 30, 'Can a landlord turn me down because of a criminal record?'),
    -- resource-directory
    ('help-lines',         10, 'Who can I call for help?'),
    ('legal-aid',          20, 'Where can I get free legal help?'),
    ('housing-counseling', 30, 'Where can I get housing counseling?'),
    -- renting-fundamentals (cross-cutting)
    ('retaliation-protection',     10, 'Can my landlord punish me for complaining?'),
    ('records-and-evidence',       20, 'What records should I keep?'),
    ('mediation',                  30, 'Can a mediator (a neutral person who helps both sides agree) help with my landlord?'),
    ('small-claims-court',         40, 'How do I use small claims court?'),
    ('complaint-line',             50, 'Where can I file a complaint?'),
    ('federal-housing-assistance', 60, 'What federal housing help is there?')
) AS v(slug, pos, q) WHERE c.slug = v.slug;

-- ---- Coverage records (D5) ----------------------------------------------------

-- "We searched and found no law" for one place and concept. A site fact
-- about our search, not a legal claim: it carries no citation and is never
-- rendered as "{State} has no rule". A statement with the tag in the place
-- always wins over a record.
CREATE TABLE coverage_records (
    id              BIGSERIAL PRIMARY KEY,
    jurisdiction_id BIGINT      NOT NULL REFERENCES jurisdictions(id) ON DELETE CASCADE,
    concept_id      BIGINT      NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    -- The official places searched, so the next check knows where to look.
    sources_checked TEXT[]      NOT NULL CHECK (cardinality(sources_checked) > 0),
    note            TEXT        NOT NULL DEFAULT '',
    checked_by      TEXT        NOT NULL,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (jurisdiction_id, concept_id)
);
