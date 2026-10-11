DELETE FROM advice_citations WHERE source_id = (SELECT id FROM sources WHERE url = 'https://clsaz.org/wp-content/uploads/2026/07/Housing-Page_AZ-Landlord-Tenant-Booklet-English.pdf');
