-- Retitle every English playbook as the question a renter actually types.
--
-- Search Console (2026-09-08): every query that reaches the site is a problem
-- stated in the searcher's words ("no heat", "deposit not returned", "eviction
-- help seattle"), almost never a place or a legal term. Only 15 of 79 live
-- titles were questions, and every page in the top-clicks list was one of the
-- 15; the same topic in label form ("Security Deposits in Chicago") got
-- impressions and no clicks. This gives each topic one question title,
-- reusing the wording of the pages that already win.
--
-- Nationwide pages drop the place entirely: nobody types "in the United
-- States", and these pages exist to catch the searcher with no place in their
-- query and route them to their state (the picker at the top of the page).
--
-- Keyed (jurisdiction slug, topic slug) with language = 'en', ANY status, so a
-- pending draft revision of a published page is retitled too and publishing it
-- cannot bring the label title back. Spanish rows are untouched. A VALUES row
-- with no matching playbook is a no-op. Safe to run more than once. The final
-- SELECT lists every row touched; expect roughly 80 published + the wave 1
-- state drafts.
BEGIN;

WITH v(jslug, tslug, title) AS (VALUES
  -- Nationwide: no place in the title. These pages catch the searcher who does not know the law is by state, so the title is the bare question and the page routes them to their state.
  ('united-states', 'cant-pay-rent', 'Can''t Pay Rent: What Are My Options?'),
  ('united-states', 'eviction-defense', 'Facing Eviction: What Can I Do?'),
  ('united-states', 'landlord-entry', 'Landlord Entering Without Notice: What Are My Rights?'),
  ('united-states', 'rent-increase', 'Rent Increases: What Are My Rights?'),
  ('united-states', 'repairs-and-habitability', 'Landlord Won''t Make Repairs: What Can I Do?'),
  ('united-states', 'security-deposits', 'Security Deposit Not Returned: What Can I Do?'),
  ('united-states', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help?'),
  ('united-states', 'heat-not-working', 'No Heat in Your Rental: What Can I Do?'),
  ('united-states', 'constructive-eviction', 'Constructive Eviction: Can I Move Out Because of Bad Conditions?'),
  ('united-states', 'renting-fundamentals', 'What Rights Does Every Renter Have?'),
  -- States (7 core topics each; unpublished wave 1 drafts get the same title so publishing them does not reintroduce a label title)
  ('california', 'cant-pay-rent', 'Can''t Pay Rent in California: What Are My Options?'),
  ('california', 'eviction-defense', 'Facing Eviction in California: What Can I Do?'),
  ('california', 'landlord-entry', 'Landlord Entering Without Notice in California: What Are My Rights?'),
  ('california', 'rent-increase', 'Rent Increases in California: What Are My Rights?'),
  ('california', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in California: What Can I Do?'),
  ('california', 'security-deposits', 'Security Deposit Not Returned in California: What Can I Do?'),
  ('california', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in California?'),
  ('illinois', 'cant-pay-rent', 'Can''t Pay Rent in Illinois: What Are My Options?'),
  ('illinois', 'eviction-defense', 'Facing Eviction in Illinois: What Can I Do?'),
  ('illinois', 'landlord-entry', 'Landlord Entering Without Notice in Illinois: What Are My Rights?'),
  ('illinois', 'rent-increase', 'Rent Increases in Illinois: What Are My Rights?'),
  ('illinois', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Illinois: What Can I Do?'),
  ('illinois', 'security-deposits', 'Security Deposit Not Returned in Illinois: What Can I Do?'),
  ('illinois', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Illinois?'),
  ('massachusetts', 'cant-pay-rent', 'Can''t Pay Rent in Massachusetts: What Are My Options?'),
  ('massachusetts', 'eviction-defense', 'Facing Eviction in Massachusetts: What Can I Do?'),
  ('massachusetts', 'landlord-entry', 'Landlord Entering Without Notice in Massachusetts: What Are My Rights?'),
  ('massachusetts', 'rent-increase', 'Rent Increases in Massachusetts: What Are My Rights?'),
  ('massachusetts', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Massachusetts: What Can I Do?'),
  ('massachusetts', 'security-deposits', 'Security Deposit Not Returned in Massachusetts: What Can I Do?'),
  ('massachusetts', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Massachusetts?'),
  ('new-york', 'cant-pay-rent', 'Can''t Pay Rent in New York: What Are My Options?'),
  ('new-york', 'eviction-defense', 'Facing Eviction in New York: What Can I Do?'),
  ('new-york', 'landlord-entry', 'Landlord Entering Without Notice in New York: What Are My Rights?'),
  ('new-york', 'rent-increase', 'Rent Increases in New York: What Are My Rights?'),
  ('new-york', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in New York: What Can I Do?'),
  ('new-york', 'security-deposits', 'Security Deposit Not Returned in New York: What Can I Do?'),
  ('new-york', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in New York?'),
  ('pennsylvania', 'cant-pay-rent', 'Can''t Pay Rent in Pennsylvania: What Are My Options?'),
  ('pennsylvania', 'eviction-defense', 'Facing Eviction in Pennsylvania: What Can I Do?'),
  ('pennsylvania', 'landlord-entry', 'Landlord Entering Without Notice in Pennsylvania: What Are My Rights?'),
  ('pennsylvania', 'rent-increase', 'Rent Increases in Pennsylvania: What Are My Rights?'),
  ('pennsylvania', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Pennsylvania: What Can I Do?'),
  ('pennsylvania', 'security-deposits', 'Security Deposit Not Returned in Pennsylvania: What Can I Do?'),
  ('pennsylvania', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Pennsylvania?'),
  ('texas', 'cant-pay-rent', 'Can''t Pay Rent in Texas: What Are My Options?'),
  ('texas', 'eviction-defense', 'Facing Eviction in Texas: What Can I Do?'),
  ('texas', 'landlord-entry', 'Landlord Entering Without Notice in Texas: What Are My Rights?'),
  ('texas', 'rent-increase', 'Rent Increases in Texas: What Are My Rights?'),
  ('texas', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Texas: What Can I Do?'),
  ('texas', 'security-deposits', 'Security Deposit Not Returned in Texas: What Can I Do?'),
  ('texas', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Texas?'),
  ('washington', 'cant-pay-rent', 'Can''t Pay Rent in Washington: What Are My Options?'),
  ('washington', 'eviction-defense', 'Facing Eviction in Washington: What Can I Do?'),
  ('washington', 'landlord-entry', 'Landlord Entering Without Notice in Washington: What Are My Rights?'),
  ('washington', 'rent-increase', 'Rent Increases in Washington: What Are My Rights?'),
  ('washington', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Washington: What Can I Do?'),
  ('washington', 'security-deposits', 'Security Deposit Not Returned in Washington: What Can I Do?'),
  ('washington', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Washington?'),
  -- Cities
  ('los-angeles', 'cant-pay-rent', 'Can''t Pay Rent in Los Angeles: What Are My Options?'),
  ('los-angeles', 'eviction-defense', 'Facing Eviction in Los Angeles: What Can I Do?'),
  ('los-angeles', 'landlord-entry', 'Landlord Entering Without Notice in Los Angeles: What Are My Rights?'),
  ('los-angeles', 'rent-increase', 'Rent Increases in Los Angeles: What Are My Rights?'),
  ('los-angeles', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Los Angeles: What Can I Do?'),
  ('los-angeles', 'security-deposits', 'Security Deposit Not Returned in Los Angeles: What Can I Do?'),
  ('los-angeles', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Los Angeles?'),
  ('chicago', 'cant-pay-rent', 'Can''t Pay Rent in Chicago: What Are My Options?'),
  ('chicago', 'eviction-defense', 'Facing Eviction in Chicago: What Can I Do?'),
  ('chicago', 'landlord-entry', 'Landlord Entering Without Notice in Chicago: What Are My Rights?'),
  ('chicago', 'rent-increase', 'Rent Increases in Chicago: What Are My Rights?'),
  ('chicago', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Chicago: What Can I Do?'),
  ('chicago', 'security-deposits', 'Security Deposit Not Returned in Chicago: What Can I Do?'),
  ('chicago', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Chicago?'),
  ('boston', 'cant-pay-rent', 'Can''t Pay Rent in Boston: What Are My Options?'),
  ('boston', 'eviction-defense', 'Facing Eviction in Boston: What Can I Do?'),
  ('boston', 'landlord-entry', 'Landlord Entering Without Notice in Boston: What Are My Rights?'),
  ('boston', 'rent-increase', 'Rent Increases in Boston: What Are My Rights?'),
  ('boston', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Boston: What Can I Do?'),
  ('boston', 'security-deposits', 'Security Deposit Not Returned in Boston: What Can I Do?'),
  ('boston', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Boston?'),
  ('new-york-city', 'cant-pay-rent', 'Can''t Pay Rent in New York City: What Are My Options?'),
  ('new-york-city', 'eviction-defense', 'Facing Eviction in New York City: What Can I Do?'),
  ('new-york-city', 'landlord-entry', 'Landlord Entering Without Notice in New York City: What Are My Rights?'),
  ('new-york-city', 'rent-increase', 'Rent Increases in New York City: What Are My Rights?'),
  ('new-york-city', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in New York City: What Can I Do?'),
  ('new-york-city', 'security-deposits', 'Security Deposit Not Returned in New York City: What Can I Do?'),
  ('new-york-city', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in New York City?'),
  ('philadelphia', 'cant-pay-rent', 'Can''t Pay Rent in Philadelphia: What Are My Options?'),
  ('philadelphia', 'eviction-defense', 'Facing Eviction in Philadelphia: What Can I Do?'),
  ('philadelphia', 'landlord-entry', 'Landlord Entering Without Notice in Philadelphia: What Are My Rights?'),
  ('philadelphia', 'rent-increase', 'Rent Increases in Philadelphia: What Are My Rights?'),
  ('philadelphia', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Philadelphia: What Can I Do?'),
  ('philadelphia', 'security-deposits', 'Security Deposit Not Returned in Philadelphia: What Can I Do?'),
  ('philadelphia', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Philadelphia?'),
  ('pittsburgh', 'cant-pay-rent', 'Can''t Pay Rent in Pittsburgh: What Are My Options?'),
  ('pittsburgh', 'eviction-defense', 'Facing Eviction in Pittsburgh: What Can I Do?'),
  ('pittsburgh', 'landlord-entry', 'Landlord Entering Without Notice in Pittsburgh: What Are My Rights?'),
  ('pittsburgh', 'rent-increase', 'Rent Increases in Pittsburgh: What Are My Rights?'),
  ('pittsburgh', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Pittsburgh: What Can I Do?'),
  ('pittsburgh', 'security-deposits', 'Security Deposit Not Returned in Pittsburgh: What Can I Do?'),
  ('pittsburgh', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Pittsburgh?'),
  ('austin', 'cant-pay-rent', 'Can''t Pay Rent in Austin: What Are My Options?'),
  ('austin', 'eviction-defense', 'Facing Eviction in Austin: What Can I Do?'),
  ('austin', 'landlord-entry', 'Landlord Entering Without Notice in Austin: What Are My Rights?'),
  ('austin', 'rent-increase', 'Rent Increases in Austin: What Are My Rights?'),
  ('austin', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Austin: What Can I Do?'),
  ('austin', 'security-deposits', 'Security Deposit Not Returned in Austin: What Can I Do?'),
  ('austin', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Austin?'),
  ('seattle', 'cant-pay-rent', 'Can''t Pay Rent in Seattle: What Are My Options?'),
  ('seattle', 'eviction-defense', 'Facing Eviction in Seattle: What Can I Do?'),
  ('seattle', 'landlord-entry', 'Landlord Entering Without Notice in Seattle: What Are My Rights?'),
  ('seattle', 'rent-increase', 'Rent Increases in Seattle: What Are My Rights?'),
  ('seattle', 'repairs-and-habitability', 'Landlord Won''t Make Repairs in Seattle: What Can I Do?'),
  ('seattle', 'security-deposits', 'Security Deposit Not Returned in Seattle: What Can I Do?'),
  ('seattle', 'resource-directory', 'Where Can I Get Rent Assistance or Eviction Help in Seattle?'),
  ('boston', 'heat-not-working', 'No Heat in Your Boston Rental: What Can I Do?'),
  ('pittsburgh', 'heat-not-working', 'No Heat in Your Pittsburgh Rental: What Can I Do?'),
  ('seattle', 'heat-not-working', 'No Heat in Your Seattle Rental: What Can I Do?'),
  ('pittsburgh', 'discrimination', 'Housing Discrimination in Pittsburgh: What Are My Rights?')
),
touched AS (
  UPDATE playbooks p
     SET title = v.title
    FROM v, jurisdictions j, topics t
   WHERE j.slug = v.jslug AND t.slug = v.tslug
     AND p.jurisdiction_id = j.id AND p.topic_id = t.id
     AND p.language = 'en'
     AND p.title IS DISTINCT FROM v.title
  RETURNING j.slug AS jurisdiction, t.slug AS topic, p.status, p.title
)
SELECT * FROM touched ORDER BY jurisdiction, topic, status;

COMMIT;
