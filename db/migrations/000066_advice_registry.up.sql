-- ADR-016 as amended 2026-10-09: practical advice is a registry pages
-- reference, written once by a person and backed by a government or nonprofit
-- quote. Two kinds:
--   tip        one quiet line under the statement it helps with
--   page_note  said once at the top of the page (the disclaimer, the risk
--              warning); a page note that warns satisfies the risky-step and
--              owe-nothing lints for every statement on the page
-- This migration is the authority on the entries, like topics and concepts.

CREATE TABLE advice (
    id                 BIGSERIAL PRIMARY KEY,
    slug               TEXT NOT NULL UNIQUE,
    kind               TEXT NOT NULL CHECK (kind IN ('tip', 'page_note')),
    -- The risk a page note states, matched to the step a statement names:
    -- 'owe' for a renter who leaves, 'evict' for one who stays and pays less.
    warns              TEXT CHECK (warns IN ('owe', 'evict')),
    body_md            TEXT NOT NULL CHECK (btrim(body_md) <> ''),
    language           TEXT NOT NULL DEFAULT 'en',
    -- The site speaking for itself (the disclaimer). The only entries that
    -- may publish with no backing quote.
    site_voice         BOOLEAN NOT NULL DEFAULT false,
    lawyer_reviewed_by TEXT,
    lawyer_reviewed_at TIMESTAMPTZ,
    retired_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (warns IS NULL OR kind = 'page_note'),
    CHECK (NOT site_voice OR kind = 'page_note')
);

CREATE TABLE advice_citations (
    id         BIGSERIAL PRIMARY KEY,
    advice_id  BIGINT NOT NULL REFERENCES advice(id) ON DELETE CASCADE,
    source_id  BIGINT NOT NULL REFERENCES sources(id),
    quote      TEXT NOT NULL CHECK (btrim(quote) <> ''),
    position   INT NOT NULL DEFAULT 0,
    -- Set by the source checker: when the quote was last found at the live
    -- source, and when it was last missing from it.
    checked_at TIMESTAMPTZ,
    drift_at   TIMESTAMPTZ,
    drift_note TEXT NOT NULL DEFAULT ''
);

-- Only government guidance and nonprofits back advice (A1). Statutes do not:
-- a habit a statute makes matter is a statement on that place's page (D3).
CREATE FUNCTION advice_citation_source_kind() RETURNS trigger AS $$
DECLARE k TEXT;
BEGIN
    SELECT kind INTO k FROM sources WHERE id = NEW.source_id;
    IF k NOT IN ('gov_guidance', 'nonprofit') THEN
        RAISE EXCEPTION 'advice may cite only government guidance or a nonprofit, not a % source', k;
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER advice_citation_source_kind
    BEFORE INSERT OR UPDATE ON advice_citations
    FOR EACH ROW EXECUTE FUNCTION advice_citation_source_kind();

-- A page's references. A tip names the statement it sits under by its
-- durable key; a page note names none.
CREATE TABLE playbook_advice (
    playbook_id   BIGINT NOT NULL REFERENCES playbooks(id) ON DELETE CASCADE,
    advice_id     BIGINT NOT NULL REFERENCES advice(id),
    statement_key UUID,
    position      INT NOT NULL DEFAULT 0,
    created_by    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (playbook_id, advice_id)
);

CREATE FUNCTION playbook_advice_shape() RETURNS trigger AS $$
DECLARE k TEXT;
BEGIN
    SELECT kind INTO k FROM advice WHERE id = NEW.advice_id;
    IF k = 'tip' AND NEW.statement_key IS NULL THEN
        RAISE EXCEPTION 'a tip must name the statement it sits under';
    END IF;
    IF k = 'page_note' AND NEW.statement_key IS NOT NULL THEN
        RAISE EXCEPTION 'a page note belongs to the page, not a statement';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER playbook_advice_shape
    BEFORE INSERT OR UPDATE ON playbook_advice
    FOR EACH ROW EXECUTE FUNCTION playbook_advice_shape();

-- Seed: the breaking-lease pilot set (Pennsylvania first, ADR-016 A6).
INSERT INTO sources (url, publisher, kind) VALUES
    ('https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf', 'Pennsylvania Office of Attorney General', 'gov_guidance'),
    ('https://www.consumerfinance.gov/ask-cfpb/can-a-debt-collector-take-or-garnish-my-wages-or-benefits-en-1439/', 'Consumer Financial Protection Bureau', 'gov_guidance')
ON CONFLICT (url) DO NOTHING;

INSERT INTO advice (slug, kind, warns, site_voice, body_md) VALUES
    ('breaking-lease-disclaimer', 'page_note', NULL, true,
     'Breaking a lease early is legally complicated. If you are in danger, leave and get safe first. Otherwise, talk to legal aid as soon as you can, before you move out or stop paying rent.'),
    ('leave-risk-owe', 'page_note', 'owe', false,
     'Even with a good reason to leave, your landlord can sue you for the rent. If a court disagrees with you, you can still owe it.'),
    ('read-your-lease', 'tip', NULL, false,
     'Read your lease before you decide. Check whether it lets you end it early and what penalty you must pay.'),
    ('write-down-the-problem', 'tip', NULL, false,
     'Write down each problem, every time you contacted your landlord, and the day you moved out.'),
    ('photos-when-you-leave', 'tip', NULL, false,
     'Before you leave, clean the home and take photos of how you left it.'),
    ('answer-court-papers', 'tip', NULL, false,
     'If you get court papers, do not ignore them. Go to court, or you can lose the case.'),
    -- No government or nonprofit source says this yet. It cannot publish
    -- until one does, or until a lawyer reviews it (A1).
    ('show-your-proof', 'tip', NULL, false,
     'Your landlord might sue if they want the money. Make sure they know you have proof.');

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, v.quote, v.position
FROM (VALUES
    ('leave-risk-owe', 'https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf',
     'You should get legal advice about your options before taking action; improperly invoking a remedy could lead to eviction if a court finds that you breached the lease.', 0),
    ('leave-risk-owe', 'https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf',
     'If you move out early, the landlord may be able to make you pay rent for the rest of the lease term.', 1),
    ('read-your-lease', 'https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf',
     'Whether you can terminate the lease early, including any penalty you must pay', 0),
    ('write-down-the-problem', 'https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf',
     'If you believe the landlord is violating the implied covenant of quiet enjoyment and has constructively evicted you, document the problem(s), all efforts to contact the landlord, and your actual vacating of the unit.', 0),
    ('photos-when-you-leave', 'https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf',
     'Before leaving, clean the unit as thoroughly as possible and take photos to document its condition.', 0),
    ('answer-court-papers', 'https://www.consumerfinance.gov/ask-cfpb/can-a-debt-collector-take-or-garnish-my-wages-or-benefits-en-1439/',
     'If you’ve had a lawsuit filed against you by a debt collector, it’s important not to ignore it because it could result in a judgment against you if you don’t appear in court.', 0)
) AS v(slug, url, quote, position)
JOIN advice a ON a.slug = v.slug
JOIN sources s ON s.url = v.url;
