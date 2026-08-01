package discover

// seedProvider returns curated, hand-verified primary sources for jurisdictions
// we have explicitly researched. It is the highest-precision provider: every
// entry is a known-good URL for that city/state.
type seedProvider struct{}

// seeds maps a jurisdiction slug to its curated candidate sources.
// To add a city, research its tenant ordinance, state statutes, and local
// legal-aid orgs, then add an entry here.
var seeds = map[string][]Candidate{
	"chicago": {
		{
			URL:        "https://codelibrary.amlegal.com/codes/chicago/latest/chicago_il/",
			Publisher:  "American Legal Publishing / City of Chicago",
			Title:      "Chicago Municipal Code — Residential Landlord and Tenant Ordinance (Ch. 5-12)",
			KindGuess:  "statute",
			Rationale:  "Chicago's primary residential tenant law (the RLTO).",
			Confidence: 1.0,
			Via:        "registry",
		},
		{
			URL:        "https://www.ilga.gov/legislation/ilcs/ilcs2.asp?ChapterID=62",
			Publisher:  "Illinois General Assembly",
			Title:      "Illinois Compiled Statutes — Ch. 765 (Property): Landlord & Tenant Acts",
			KindGuess:  "statute",
			Rationale:  "State landlord-tenant statutes applying statewide (incl. security deposits).",
			Confidence: 0.95,
			Via:        "registry",
		},
		{
			URL:        "https://www.chicago.gov/city/en/depts/doh/provdrs/renters/svcs/rlto.html",
			Publisher:  "City of Chicago, Department of Housing",
			Title:      "Residential Landlord and Tenant Ordinance — official summary",
			KindGuess:  "gov_guidance",
			Rationale:  "City's plain-language RLTO overview and renter resources.",
			Confidence: 0.9,
			Via:        "registry",
		},
		{
			URL:        "https://www.cookcountyil.gov/service/residential-tenant-and-landlord-ordinance",
			Publisher:  "Cook County, Illinois",
			Title:      "Cook County Residential Tenant and Landlord Ordinance (RTLO)",
			KindGuess:  "statute",
			Rationale:  "County ordinance covering suburban Cook County tenancies.",
			Confidence: 0.85,
			Via:        "registry",
		},
		{
			URL:        "https://www.illinoislegalaid.org/",
			Publisher:  "Illinois Legal Aid Online",
			Title:      "Illinois Legal Aid Online — housing self-help",
			KindGuess:  "nonprofit",
			Rationale:  "Statewide legal-aid guidance and court forms for tenants.",
			Confidence: 0.8,
			Via:        "registry",
		},
		{
			URL:        "https://www.tenants-rights.org/",
			Publisher:  "Metropolitan Tenants Organization",
			Title:      "Metropolitan Tenants Organization",
			KindGuess:  "nonprofit",
			Rationale:  "Chicago tenant-rights nonprofit with a renters' hotline.",
			Confidence: 0.8,
			Via:        "registry",
		},
		{
			URL:        "https://www.lcbh.org/",
			Publisher:  "Lawyers' Committee for Better Housing",
			Title:      "Lawyers' Committee for Better Housing (LCBH)",
			KindGuess:  "nonprofit",
			Rationale:  "Chicago legal aid focused on eviction defense and housing.",
			Confidence: 0.8,
			Via:        "registry",
		},
	},

	// The entries below were verified during the 2026-07 five-city pilot:
	// each URL was fetched and quote-checked by the drafting pipeline.
	"new-york-city": {
		{URL: "https://ag.ny.gov/publications/residential-tenants-rights-guide", Publisher: "New York State Office of the Attorney General", Title: "Residential Tenants' Rights Guide", KindGuess: "gov_guidance", Rationale: "The state's authoritative plain-language survey of NY tenant law.", Confidence: 1.0, Via: "registry"},
		{URL: "https://www.nyc.gov/site/hpd/services-and-information/tenants-rights-and-responsibilities.page", Publisher: "NYC Department of Housing Preservation and Development", Title: "Tenants' Rights and Responsibilities", KindGuess: "gov_guidance", Rationale: "The city's own statement of tenant rights, incl. habitability and heat rules.", Confidence: 0.95, Via: "registry"},
		{URL: "https://hcr.ny.gov/harassment", Publisher: "New York State Homes and Community Renewal", Title: "Tenant Harassment", KindGuess: "gov_guidance", Rationale: "State guidance on harassment and unlawful eviction.", Confidence: 0.85, Via: "registry"},
		{URL: "https://www.nyc.gov/site/mayorspeu/resources/right-to-counsel.page", Publisher: "NYC Mayor's Public Engagement Unit", Title: "Right to Counsel", KindGuess: "gov_guidance", Rationale: "NYC's free eviction lawyer program; belongs in every help section.", Confidence: 0.85, Via: "registry"},
		{URL: "https://access.nyc.gov/programs/one-shot-deal/", Publisher: "ACCESS NYC / NYC Human Resources Administration", Title: "One Shot Deal emergency assistance", KindGuess: "gov_guidance", Rationale: "Emergency rent assistance for renters behind on rent.", Confidence: 0.8, Via: "registry"},
	},

	"los-angeles": {
		{URL: "https://leginfo.legislature.ca.gov/faces/codes_displaySection.xhtml?lawCode=CIV&sectionNum=1950.5", Publisher: "California Legislative Information", Title: "Civ. Code § 1950.5 — security deposits", KindGuess: "statute", Rationale: "California's deposit statute; leginfo serves clean statute text per section.", Confidence: 1.0, Via: "registry"},
		{URL: "https://leginfo.legislature.ca.gov/faces/codes_displaySection.xhtml?lawCode=CIV&sectionNum=1946.2", Publisher: "California Legislative Information", Title: "Civ. Code § 1946.2 — just-cause termination", KindGuess: "statute", Rationale: "Statewide just-cause eviction protections (AB 1482).", Confidence: 1.0, Via: "registry"},
		{URL: "https://leginfo.legislature.ca.gov/faces/codes_displaySection.xhtml?lawCode=CIV&sectionNum=1954", Publisher: "California Legislative Information", Title: "Civ. Code § 1954 — landlord entry", KindGuess: "statute", Rationale: "California's entry-notice statute.", Confidence: 1.0, Via: "registry"},
		{URL: "https://leginfo.legislature.ca.gov/faces/codes_displaySection.xhtml?lawCode=CCP&sectionNum=1161", Publisher: "California Legislative Information", Title: "CCP § 1161 — unlawful detainer", KindGuess: "statute", Rationale: "The eviction-process statute.", Confidence: 0.95, Via: "registry"},
		{URL: "https://housing.lacity.gov/residents/just-cause-for-eviction-ordinance-jco", Publisher: "Los Angeles Housing Department", Title: "Just Cause for Eviction Ordinance (JCO)", KindGuess: "gov_guidance", Rationale: "LA's city-level eviction protections. Site blocks bots; the fetcher falls back to an archive snapshot.", Confidence: 0.9, Via: "registry"},
		{URL: "https://housing.lacity.gov/residents/file-a-complaint", Publisher: "Los Angeles Housing Department", Title: "File a complaint", KindGuess: "gov_guidance", Rationale: "The city's habitability-complaint channel; belongs in the help section.", Confidence: 0.85, Via: "registry"},
	},

	"austin": {
		{URL: "https://texas.public.law/statutes/tex._prop._code_section_92.103", Publisher: "Texas Legislature (via Texas.Public.Law)", Title: "Tex. Prop. Code § 92.103 — deposit refund", KindGuess: "statute", Rationale: "Verbatim mirror of the Property Code; statutes.capitol.texas.gov is a JS-only shell. Sibling sections share this URL pattern.", Confidence: 1.0, Via: "registry"},
		{URL: "https://texas.public.law/statutes/tex._prop._code_section_92.109", Publisher: "Texas Legislature (via Texas.Public.Law)", Title: "Tex. Prop. Code § 92.109 — bad-faith deposit retention", KindGuess: "statute", Rationale: "The $100 + 3x penalty renters rely on.", Confidence: 1.0, Via: "registry"},
		{URL: "https://texas.public.law/statutes/tex._prop._code_section_24.005", Publisher: "Texas Legislature (via Texas.Public.Law)", Title: "Tex. Prop. Code § 24.005 — notice to vacate", KindGuess: "statute", Rationale: "Eviction notice requirements.", Confidence: 0.95, Via: "registry"},
		{URL: "https://guides.sll.texas.gov/landlord-tenant-law/security-deposits", Publisher: "Texas State Law Library", Title: "Landlord/Tenant Law research guide", KindGuess: "gov_guidance", Rationale: "State law library guides across all tenant topics.", Confidence: 0.9, Via: "registry"},
		{URL: "https://texaslawhelp.org/article/security-deposits", Publisher: "TexasLawHelp.org (Texas Legal Services Center)", Title: "TexasLawHelp tenant articles", KindGuess: "nonprofit", Rationale: "Statewide legal-aid guidance; per-topic articles share this URL pattern.", Confidence: 0.85, Via: "registry"},
		{URL: "https://www.trla.org/atc", Publisher: "Texas RioGrande Legal Aid", Title: "Austin Tenants Council", KindGuess: "nonprofit", Rationale: "Austin's tenant counseling program; belongs in the help section.", Confidence: 0.85, Via: "registry"},
		{URL: "https://www.austintexas.gov/development-services/code-compliance-resources", Publisher: "City of Austin", Title: "Code compliance resources", KindGuess: "gov_guidance", Rationale: "Austin 311 / code-complaint channel for repairs.", Confidence: 0.8, Via: "registry"},
	},

	"philadelphia": {
		{URL: "https://codes.findlaw.com/pa/title-68-ps-real-and-personal-property/pa-st-sect-68-250-511a/", Publisher: "Pennsylvania Statutes (68 P.S.), via FindLaw", Title: "68 P.S. § 250.511a — escrow of deposits", KindGuess: "statute", Rationale: "PA statute text; the official source is PDF-only. Sibling sections share this URL pattern.", Confidence: 0.9, Via: "registry"},
		{URL: "https://codes.findlaw.com/pa/title-68-ps-real-and-personal-property/pa-st-sect-68-250-501/", Publisher: "Pennsylvania Statutes (68 P.S.), via FindLaw", Title: "68 P.S. § 250.501 — notice to quit", KindGuess: "statute", Rationale: "PA's eviction-notice statute.", Confidence: 0.9, Via: "registry"},
		{URL: "https://www.phila.gov/departments/fair-housing-commission/tenant-protections/rental-suitability/", Publisher: "City of Philadelphia — Fair Housing Commission", Title: "Tenant protections — rental suitability", KindGuess: "gov_guidance", Rationale: "Philadelphia's city-level tenant protections (Good Cause, suitability).", Confidence: 0.95, Via: "registry"},
		{URL: "https://phillytenant.org/security-deposits/", Publisher: "PhillyTenant.org (Philadelphia legal services collaboration)", Title: "PhillyTenant self-help articles", KindGuess: "nonprofit", Rationale: "Local legal-services guidance; per-topic articles share this URL pattern.", Confidence: 0.85, Via: "registry"},
		{URL: "https://phillytenant.org/eviction-diversion-program/", Publisher: "PhillyTenant.org", Title: "Eviction Diversion Program", KindGuess: "nonprofit", Rationale: "Philadelphia's mandatory pre-filing diversion program; belongs in the help section.", Confidence: 0.85, Via: "registry"},
	},

	"united-states": {
		{URL: "https://www.law.cornell.edu/uscode/text/42/3604", Publisher: "U.S. Code via Legal Information Institute, Cornell Law School", Title: "42 U.S.C. § 3604 — Fair Housing Act prohibitions", KindGuess: "statute", Rationale: "The federal anti-discrimination statute, full text.", Confidence: 1.0, Via: "registry"},
		{URL: "https://www.law.cornell.edu/wex/landlord-tenant_law", Publisher: "Legal Information Institute, Cornell Law School", Title: "Wex: landlord-tenant law", KindGuess: "nonprofit", Rationale: "Authoritative overview of common-law doctrines (habitability, entry, eviction process).", Confidence: 0.9, Via: "registry"},
		{URL: "https://www.consumerfinance.gov/housing/housing-insecurity/help-for-renters/", Publisher: "Consumer Financial Protection Bureau", Title: "Help for renters", KindGuess: "gov_guidance", Rationale: "Federal renter-help hub, incl. eviction guidance.", Confidence: 0.9, Via: "registry"},
		{URL: "https://www.lsc.gov/about-lsc", Publisher: "Legal Services Corporation", Title: "Legal Services Corporation", KindGuess: "nonprofit", Rationale: "The find-free-legal-aid front door for every help section.", Confidence: 0.85, Via: "registry"},
		{URL: "https://www.211.org/about-us", Publisher: "United Way Worldwide (211)", Title: "211", KindGuess: "nonprofit", Rationale: "National referral line for rent assistance and local services.", Confidence: 0.8, Via: "registry"},
	},
}

func (seedProvider) Discover(slug string) []Candidate {
	src := seeds[slug]
	out := make([]Candidate, len(src))
	copy(out, src)
	return out
}
