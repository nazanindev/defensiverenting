package draftagent

// Topic is a playbook topic to draft: its URL slug and display name.
type Topic struct {
	Slug string
	Name string
}

// CoreTopics is the predetermined set every city is seeded with — the most
// common situations a renter needs help with. Editing this list changes what
// `draft -topics core` generates.
var CoreTopics = []Topic{
	{"security-deposits", "Security Deposits"},
	{"eviction-defense", "Eviction & Notice to Quit"},
	{"repairs-and-habitability", "Repairs & Habitability"},
	{"cant-pay-rent", "Can't Pay Rent"},
	{"landlord-entry", "Landlord Entry & Privacy"},
}
