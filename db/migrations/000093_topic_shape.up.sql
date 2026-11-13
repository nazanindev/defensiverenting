-- The topic shape (ADR-036 D4): what a page of a topic holds, as data. The
-- shape is the same in every state. It has four parts, each stored on its
-- own so a change to one part touches nothing in the others:
--
--   1. Title and scope          columns on topics
--   2. Tips and warning notes   topic_advice
--   3. Size and order           columns on topics, and the extra rows of topic_questions
--   4. Must-answer questions    topic_questions, concept_situations
--
-- Eviction and breaking a lease are filled: their four parts were approved
-- on 2026-11-12. A topic with nothing here has no shape yet.

-- ---- 1. Title and scope, 3. Size and order ------------------------------------

ALTER TABLE topics
    -- The page title, a renter's question, with {Place} where the place
    -- goes. The nationwide page drops " in {Place}".
    ADD COLUMN title_pattern TEXT   NOT NULL DEFAULT '',
    -- Who the page is for, in one sentence.
    ADD COLUMN reader        TEXT   NOT NULL DEFAULT '',
    -- How the intro opens, when the topic sets it. Empty: no rule.
    ADD COLUMN intro_opens   TEXT   NOT NULL DEFAULT '',
    -- What is on the page, in one line.
    ADD COLUMN covers        TEXT   NOT NULL DEFAULT '',
    -- What goes to another topic, one line each.
    ADD COLUMN not_here      TEXT[] NOT NULL DEFAULT '{}',

    -- Caps. 0 means no cap.
    ADD COLUMN max_statements      INT NOT NULL DEFAULT 0 CHECK (max_statements >= 0),
    ADD COLUMN max_words           INT NOT NULL DEFAULT 0 CHECK (max_words >= 0),
    ADD COLUMN max_statement_words INT NOT NULL DEFAULT 0 CHECK (max_statement_words >= 0),
    -- How many extra questions (topic_questions.role = 'extra') a page may hold.
    ADD COLUMN max_extras          INT NOT NULL DEFAULT 0 CHECK (max_extras >= 0),
    -- Rules of order and size an agent reads, one line each: what leads a
    -- section, how a section is ordered, where a cap bends.
    ADD COLUMN order_rules       TEXT[] NOT NULL DEFAULT '{}',
    -- One statement that sounds right for this topic.
    ADD COLUMN example_statement TEXT   NOT NULL DEFAULT '';

-- ---- 4. Must-answer questions (and the extras of part 3) ----------------------

-- The questions of a topic. A link row, not a flag on the concept, because
-- a situation page asks questions whose home is another topic.
CREATE TABLE topic_questions (
    topic_id   BIGINT NOT NULL REFERENCES topics(id)   ON DELETE CASCADE,
    concept_id BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    -- must: every page answers it, or its place carries a "searched, no law
    -- found" record. extra: a page may also answer it, up to max_extras.
    role       TEXT   NOT NULL DEFAULT 'must' CHECK (role IN ('must', 'extra')),
    -- The section heading a must answer sits under: one of the topic's
    -- stages. Empty on an extra.
    stage      TEXT   NOT NULL DEFAULT '',
    -- Order on the page, across sections.
    position   INT    NOT NULL,
    -- How the question is answered (ADR-036 D7). days: every place has a
    -- stored number. days_where_set: words everywhere, and a stored number
    -- only where the law sets one. list: one statement per protection the
    -- place has. Empty: words only.
    answer     TEXT   NOT NULL DEFAULT '' CHECK (answer IN ('', 'days', 'days_where_set', 'list')),
    -- An extra that is never cut to fit the caps.
    never_cut  BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (topic_id, concept_id),
    CHECK (role = 'extra' OR stage <> ''),
    CHECK (role = 'extra' OR NOT never_cut)
);

-- The situations of a question (ADR-036 D2): the short fixed list a rule
-- value's condition is drawn from. A question with no rows here has only
-- its base case, stored with an empty condition, or has not been given a
-- list yet. The list belongs to the question, not to a place.
CREATE TABLE concept_situations (
    concept_id BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    name       TEXT   NOT NULL CHECK (btrim(name) <> ''),
    position   INT    NOT NULL,
    -- The ordinary case most people search for. At most one per question.
    is_base    BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (concept_id, name)
);
CREATE UNIQUE INDEX concept_situations_one_base ON concept_situations (concept_id) WHERE is_base;

-- ---- 2. Tips and warning notes ------------------------------------------------

-- Which tips and warning notes a topic uses. Where a tip sits on a page is
-- the tip's own rule and describes the statement above it; this table only
-- says whether the topic uses it.
CREATE TABLE topic_advice (
    topic_id  BIGINT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    advice_id BIGINT NOT NULL REFERENCES advice(id),
    -- every_page: each page of the topic carries it. where_it_fits: a page
    -- carries it when a statement meets the tip's rule. never: no page of
    -- the topic carries it.
    use       TEXT   NOT NULL DEFAULT 'where_it_fits' CHECK (use IN ('every_page', 'where_it_fits', 'never')),
    PRIMARY KEY (topic_id, advice_id)
);

-- ---- Eviction (approved 2026-11-12) -------------------------------------------

UPDATE topics SET
    title_pattern = 'Facing Eviction in {Place}: What Can I Do?',
    reader        = 'A renter who got an eviction notice or court papers.',
    covers        = 'Notices. Paying to stop the case. Answering court papers. The hearing. Missing court. Time to move. Appeal.',
    not_here      = ARRAY[
        'Lockouts and shutoffs go to Locked Out.',
        'A lease that will not be renewed goes to Lease Renewal.'],
    max_statements      = 12,
    max_words           = 650,
    max_statement_words = 75,
    max_extras          = 4,
    order_rules   = ARRAY[
        'A defense with exceptions may run to 100 words, and its page to 700.',
        'Unpaid rent leads each section, then lease violations. Narrow cases come last or are cut.',
        'Going to court follows the order things happen to the renter. If the hearing comes before a written answer is due, the hearing comes first.',
        'A page may also hold one local rule. It counts as an extra.'],
    example_statement = 'Go to your trial. If you do not come after you were properly served, the court tries the case without you.'
WHERE slug = 'eviction-defense';

INSERT INTO topic_questions (topic_id, concept_id, role, stage, position, answer, never_cut)
SELECT t.id, c.id, v.role, v.stage, v.pos, v.answer, v.never_cut
FROM (VALUES
    ('late-rent-notice',            'must',  'The notice',              10, 'days',           false),
    ('notice-to-quit',              'must',  'The notice',              20, 'days',           false),
    ('pay-and-stay',                'must',  'Paying to stop the case', 30, 'days_where_set', false),
    ('answer-the-case',             'must',  'Going to court',          40, 'days',           false),
    ('eviction-court-process',      'must',  'Going to court',          50, '',               false),
    ('default-judgment',            'must',  'Going to court',          60, '',               false),
    ('time-to-move-after-judgment', 'must',  'After the court decides', 70, 'days',           false),
    ('eviction-appeal',             'must',  'After the court decides', 80, 'days',           false),
    ('retaliation-protection',      'extra', '',                        110, '',      true),
    ('illegal-lockout',             'extra', '',                        120, '',      true),
    ('right-to-counsel',            'extra', '',                        130, '',      false),
    ('eviction-record',             'extra', '',                        140, '',      false)
) AS v(concept, role, stage, pos, answer, never_cut)
JOIN topics t ON t.slug = 'eviction-defense'
JOIN concepts c ON c.slug = v.concept;

-- The situations the eviction table was filed under (2026-11-12). The
-- names are the conditions on the rule values, letter for letter.
INSERT INTO concept_situations (concept_id, name, position, is_base)
SELECT c.id, v.name, v.pos, v.is_base
FROM (VALUES
    ('notice-to-quit',              'lease violation you can fix',    10, true),
    ('notice-to-quit',              'lease violation you cannot fix', 20, false),
    ('notice-to-quit',              'serious harm or illegal act',    30, false),
    ('pay-and-stay',                'after the court papers',         10, false),
    ('pay-and-stay',                'after the judgment',             20, false),
    ('answer-the-case',             'to file a written answer',       10, false),
    ('answer-the-case',             'until the court date',           20, false),
    ('time-to-move-after-judgment', 'after the judgment',             10, false),
    ('time-to-move-after-judgment', 'after the officer''s notice',    20, false)
) AS v(concept, name, pos, is_base)
JOIN concepts c ON c.slug = v.concept;

INSERT INTO topic_advice (topic_id, advice_id, use)
SELECT t.id, a.id, v.use
FROM (VALUES
    ('answer-court-papers',   'every_page'),
    ('award-win-and-pays',    'where_it_fits'),
    ('ask-for-a-receipt',     'where_it_fits'),
    ('plan-in-writing',       'where_it_fits'),
    ('keep-a-record',         'where_it_fits'),
    ('inspector-may-condemn', 'never')
) AS v(advice, use)
JOIN topics t ON t.slug = 'eviction-defense'
JOIN advice a ON a.slug = v.advice AND a.language = 'en';

-- ---- Breaking a lease (approved 2026-11-12) -----------------------------------

UPDATE topics SET
    title_pattern = 'Breaking a Lease Early in {Place}: What Are My Options?',
    reader        = 'A renter who needs to leave before the lease ends.',
    intro_opens   = 'The common case: "If you leave early without a legal reason, you can owe rent." Then the real path in one or two sentences.',
    covers        = 'What you may still owe. The landlord''s duty to look for a new renter. Leaving over an unlivable home. Giving notice. Special protections (abuse, military). The deposit after leaving early.',
    not_here      = ARRAY['The landlord ending the lease goes to Lease Renewal or Eviction.'],
    max_statements      = 12,
    max_words           = 650,
    max_statement_words = 75,
    max_extras          = 3,
    order_rules   = ARRAY[
        'The two debt collector statements close After you move out, in the same words on every page. They are not counted in the caps.',
        'The common case leads each section. Abuse and military exits never lead.',
        'How to leave holds steps and no must-answer question: give notice in writing, what the notice says.',
        'A page may also hold the deposit after leaving early, the notice steps, and one local rule.'],
    example_statement = 'The law gives your landlord 10 days after they get your notice to fix it. If they do not fix it, your lease ends.'
WHERE slug = 'breaking-lease';

INSERT INTO topic_questions (topic_id, concept_id, role, stage, position, answer)
SELECT t.id, c.id, 'must', v.stage, v.pos, v.answer
FROM (VALUES
    ('duty-to-mitigate',         'If you leave without a legal reason', 10, ''),
    ('constructive-eviction',    'When you can leave early',            20, 'days_where_set'),
    ('early-termination-rights', 'Special protections',                 30, 'list')
) AS v(concept, stage, pos, answer)
JOIN topics t ON t.slug = 'breaking-lease'
JOIN concepts c ON c.slug = v.concept;

-- Every tip and note the breaking-a-lease pages carry on prod
-- (2026-11-12). every_page: on all 51 pages. The rest sit on some.
INSERT INTO topic_advice (topic_id, advice_id, use)
SELECT t.id, a.id, v.use
FROM (VALUES
    ('breaking-lease-disclaimer', 'every_page'),
    ('inspector-may-condemn',     'every_page'),
    ('answer-court-papers',       'every_page'),
    ('keep-a-record',             'every_page'),
    ('read-your-lease',           'every_page'),
    ('photos-when-you-leave',     'every_page'),
    ('write-down-the-problem',    'where_it_fits'),
    ('money-example',             'where_it_fits'),
    ('leave-owe-short',           'where_it_fits'),
    ('leave-court-may-disagree',  'where_it_fits'),
    ('leave-risk-owe',            'where_it_fits')
) AS v(advice, use)
JOIN topics t ON t.slug = 'breaking-lease'
JOIN advice a ON a.slug = v.advice AND a.language = 'en';

-- A must question sits under a stage its topic has.
DO $$
DECLARE bad TEXT;
BEGIN
    SELECT string_agg(t.slug || ': ' || q.stage, '; ') INTO bad
    FROM topic_questions q JOIN topics t ON t.id = q.topic_id
    WHERE q.role = 'must' AND NOT q.stage = ANY (t.stages);
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'must-answer question under a stage the topic does not have: %', bad;
    END IF;
END $$;
