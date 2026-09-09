package store

// ContentLanguages is the registry of languages the site authors, drafts,
// checks, and queues content in (ADR-015 D1). Spanish is built end to end
// (ADR-007, ADR-008) and deferred: fourteen drafts are parked until there is
// a reviewer who reads them and a link between English and Spanish
// statement keys (ADR-014 D6). Adding a language here is the switch; the
// voice rulesets are a separate, wider list.
var ContentLanguages = []string{"en"}

// LanguageActive reports whether content work happens in this language.
func LanguageActive(lang string) bool {
	for _, l := range ContentLanguages {
		if l == lang {
			return true
		}
	}
	return false
}
