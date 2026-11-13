DROP TABLE IF EXISTS concept_situations;
DROP TABLE IF EXISTS topic_advice;
DROP TABLE IF EXISTS topic_questions;
ALTER TABLE topics
    DROP COLUMN IF EXISTS title_pattern,
    DROP COLUMN IF EXISTS reader,
    DROP COLUMN IF EXISTS intro_opens,
    DROP COLUMN IF EXISTS covers,
    DROP COLUMN IF EXISTS not_here,
    DROP COLUMN IF EXISTS max_statements,
    DROP COLUMN IF EXISTS max_words,
    DROP COLUMN IF EXISTS max_statement_words,
    DROP COLUMN IF EXISTS max_extras,
    DROP COLUMN IF EXISTS order_rules,
    DROP COLUMN IF EXISTS example_statement;
