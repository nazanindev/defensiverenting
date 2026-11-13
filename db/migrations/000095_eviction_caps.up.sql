-- Eviction caps, decided by Nazanin on 2026-11-12 after the first audit of
-- the reworked drafts against the stored shape (ADR-035 D9).
--
-- 1. A statement may run to 90 words, not 75. 33 of the 42 statements over
--    75 were over by 15 words or fewer, most because a reader fix added a
--    condition the quote attaches. Plain over short.
-- 2. A city page may hold one more statement and one more extra than a state
--    page: it carries the state's answers and the city's own rule.

-- How many more statements, and how many more extras, a city page of the
-- topic may hold. 0: a city page has the same caps as a state page.
ALTER TABLE topics ADD COLUMN city_more INT NOT NULL DEFAULT 0 CHECK (city_more >= 0);

UPDATE topics
SET max_statement_words = 90,
    city_more = 1,
    order_rules = ARRAY[
        'A defense with exceptions may run to 100 words, and its page to 700.',
        'Unpaid rent leads each section, then lease violations. Narrow cases come last or are cut.',
        'Going to court follows the order things happen to the renter. If the hearing comes before a written answer is due, the hearing comes first.',
        'A page may also hold one local rule. It counts as an extra.',
        'A city page may hold one more statement and one more extra than a state page.']
WHERE slug = 'eviction-defense';
