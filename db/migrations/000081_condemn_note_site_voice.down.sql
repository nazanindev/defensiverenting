UPDATE advice SET site_voice = false,
  body_md = 'One caution from legal aid: if the home is in very bad shape, an inspector can condemn it. Then everyone must move out, you too.'
WHERE slug = 'inspector-may-condemn';
ALTER TABLE advice ADD CONSTRAINT advice_warning_needs_source CHECK (NOT (site_voice AND warns IS NOT NULL));
