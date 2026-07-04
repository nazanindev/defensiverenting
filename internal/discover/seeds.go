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
}

func (seedProvider) Discover(slug string) []Candidate {
	src := seeds[slug]
	out := make([]Candidate, len(src))
	copy(out, src)
	return out
}
