UPDATE concepts
   SET slug = 'hotline-211', name = '211 helpline',
       definition = 'A free helpline. Dial 211 to find rent help and services near you.'
 WHERE slug = 'help-lines';
UPDATE statements SET concept_id = NULL
    WHERE concept_id IN (SELECT id FROM concepts WHERE slug IN ('partial-payments', 'rent-debt-collection', 'grace-period'));
DELETE FROM concepts WHERE slug IN ('partial-payments', 'rent-debt-collection', 'grace-period');
