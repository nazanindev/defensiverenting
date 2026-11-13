UPDATE topics
SET max_statement_words = 75,
    order_rules = ARRAY[
        'A defense with exceptions may run to 100 words, and its page to 700.',
        'Unpaid rent leads each section, then lease violations. Narrow cases come last or are cut.',
        'Going to court follows the order things happen to the renter. If the hearing comes before a written answer is due, the hearing comes first.',
        'A page may also hold one local rule. It counts as an extra.']
WHERE slug = 'eviction-defense';

ALTER TABLE topics DROP COLUMN city_more;
