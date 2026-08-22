-- Statement concepts. ADR-011.
--
-- A concept names a claim that recurs across jurisdictions ("protection from
-- retaliation", "deadline to return the deposit") so the statements making
-- that claim on different pages are connected instead of being copies nobody
-- can find. Like topics (migration 000014, ADR-005 D5) this is a closed
-- registry: the vocabulary is seeded here and edited by migration, never
-- extended at save time. The same day this table was designed, an open
-- vocabulary (sources.kind='editorial') put a law firm's blog behind the
-- editorial checkbox on three live pages; registries stay closed.
CREATE TABLE concepts (
    id       BIGSERIAL PRIMARY KEY,
    slug     TEXT   NOT NULL UNIQUE,
    name     TEXT   NOT NULL,
    -- The owning topic keeps pickers short. Concepts owned by
    -- renting-fundamentals are cross-cutting (retaliation, discrimination,
    -- lockouts) and may be tagged on a page of any topic; every other concept
    -- may only be tagged on its own topic's pages. Enforced by both tagging
    -- paths, not by the schema: the pairing is editorial, not relational.
    topic_id BIGINT NOT NULL REFERENCES topics(id)
);

-- One concept per statement, optional. A statement is one atomic claim
-- (ADR-003), so a statement needing two concepts is two statements. Most
-- statements never need one: concepts exist for claims that vary by place,
-- not for page-specific procedure.
ALTER TABLE statements ADD COLUMN concept_id BIGINT REFERENCES concepts(id);

-- Starter vocabulary, derived by reading the national guides' statements
-- (they are the generalized instances the vocabulary describes). Names are
-- authoring-facing labels; slugs are public statement anchors (ADR-011 D4),
-- so both follow the editorial voice.
INSERT INTO concepts (slug, name, topic_id) VALUES
    -- Cross-cutting, taggable on any page.
    ('retaliation-protection',   'Protection from retaliation',        (SELECT id FROM topics WHERE slug = 'renting-fundamentals')),
    ('fair-housing',             'Housing discrimination is illegal',  (SELECT id FROM topics WHERE slug = 'renting-fundamentals')),
    ('court-eviction-only',      'Only a court can evict',             (SELECT id FROM topics WHERE slug = 'renting-fundamentals')),
    ('illegal-lockout',          'Lockouts and shutoffs are illegal',  (SELECT id FROM topics WHERE slug = 'renting-fundamentals')),

    ('late-rent-notice',         'Notice before a late-rent eviction', (SELECT id FROM topics WHERE slug = 'cant-pay-rent')),
    ('rent-assistance-programs', 'Rent help programs',                 (SELECT id FROM topics WHERE slug = 'cant-pay-rent')),

    ('notice-to-quit',           'Notice before an eviction case',     (SELECT id FROM topics WHERE slug = 'eviction-defense')),
    ('pay-and-stay',             'Paying back rent can stop the case', (SELECT id FROM topics WHERE slug = 'eviction-defense')),
    ('answer-the-case',          'Respond to the lawsuit and go to court', (SELECT id FROM topics WHERE slug = 'eviction-defense')),
    ('eviction-record',          'Eviction records and sealing',       (SELECT id FROM topics WHERE slug = 'eviction-defense')),

    ('entry-notice-period',      'Advance notice before entry',        (SELECT id FROM topics WHERE slug = 'landlord-entry')),
    ('entry-allowed-reasons',    'Allowed reasons for entry',          (SELECT id FROM topics WHERE slug = 'landlord-entry')),
    ('emergency-entry',          'Emergency entry',                    (SELECT id FROM topics WHERE slug = 'landlord-entry')),
    ('entry-penalties',          'Penalties for illegal entry',        (SELECT id FROM topics WHERE slug = 'landlord-entry')),
    ('lock-change-rules',        'Changing your locks',                (SELECT id FROM topics WHERE slug = 'landlord-entry')),

    ('rent-control',             'Rent caps and rent control',         (SELECT id FROM topics WHERE slug = 'rent-increase')),
    ('mid-lease-protection',     'No increase during a lease term',    (SELECT id FROM topics WHERE slug = 'rent-increase')),
    ('increase-notice-period',   'Written notice before an increase',  (SELECT id FROM topics WHERE slug = 'rent-increase')),

    ('habitability-standard',    'Right to a safe, livable home',      (SELECT id FROM topics WHERE slug = 'repairs-and-habitability')),
    ('repair-request-in-writing','Repair requests in writing',         (SELECT id FROM topics WHERE slug = 'repairs-and-habitability')),
    ('rent-withholding',         'Withholding rent for repairs',       (SELECT id FROM topics WHERE slug = 'repairs-and-habitability')),
    ('repair-and-deduct',        'Repair and deduct',                  (SELECT id FROM topics WHERE slug = 'repairs-and-habitability')),
    ('code-inspection',          'Reporting to a housing inspector',   (SELECT id FROM topics WHERE slug = 'repairs-and-habitability')),

    ('deposit-cap',              'Deposit amount cap',                 (SELECT id FROM topics WHERE slug = 'security-deposits')),
    ('deposit-receipt',          'Deposit receipts',                   (SELECT id FROM topics WHERE slug = 'security-deposits')),
    ('deposit-escrow-interest',  'Deposit holding and interest',       (SELECT id FROM topics WHERE slug = 'security-deposits')),
    ('deposit-return-deadline',  'Deadline to return the deposit',     (SELECT id FROM topics WHERE slug = 'security-deposits')),
    ('deduction-itemization',    'Itemized deductions list',           (SELECT id FROM topics WHERE slug = 'security-deposits')),
    ('deposit-damages',          'Penalties for a wrongly kept deposit', (SELECT id FROM topics WHERE slug = 'security-deposits')),

    ('free-legal-help',          'Free legal help',                    (SELECT id FROM topics WHERE slug = 'resource-directory')),
    ('hotline-211',              '211 helpline',                       (SELECT id FROM topics WHERE slug = 'resource-directory')),
    ('housing-counseling',       'HUD housing counseling',             (SELECT id FROM topics WHERE slug = 'resource-directory'));
