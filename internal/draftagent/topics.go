package draftagent

// Topic is a playbook topic to draft: its URL slug and display name.
//
// The canonical set of topics lives in the database, not here. A hardcoded list
// in this package used to disagree with the one in cmd/authoring, which is how
// two vocabularies for the same subjects came to exist. Callers resolve topics
// against store.ListTopicRegistry. See docs/ADRs/ADR-005 D5.
type Topic struct {
	Slug string
	Name string
}
