DELETE FROM advice_citations
WHERE source_id = (SELECT id FROM sources WHERE url = 'https://nlihc.org/resource/evictions-101-eviction-process-how-it-works-and-what-know');
