package main

import (
	"strings"
	"testing"
)

const statute = `1950.5. (a) This section applies to security for a rental agreement for residential property that is used as the dwelling of the tenant.
(b) As used in this section, "security" means any payment, fee, deposit, or charge, including, but not limited to, any payment, fee, deposit, or charge, except as provided in Section 1950.6, that is imposed at the beginning of the tenancy to be used to reimburse the landlord for costs associated with processing a new tenant or that is imposed as an advance payment of rent, used or to be used for any purpose, including, but not limited to, any of the following:
(1) The compensation of a landlord for a tenant's default in the payment of rent.
(2) The repair of damages to the premises, exclusive of ordinary wear and tear, caused by the tenant or by a guest or licensee of the tenant.
(3) The cleaning of the premises upon termination of the tenancy necessary to return the unit to the same level of cleanliness it was in at the inception of the tenancy. The amendments to this paragraph enacted by the act adding this sentence shall apply only to tenancies for which the tenant's right to occupy begins after January 1, 2003.
(c) A landlord may not demand or receive security, however denominated, in an amount or value in excess of an amount equal to one month's rent, in addition to any rent for the first month paid on or before initial occupancy. This subdivision does not prohibit an advance payment of not less than six months' rent if the term of the lease is six months or more.
(d) Any security shall be held by the landlord for the tenant who is party to the lease or agreement.
1950.6. (a) Notwithstanding Section 1950.5, when a landlord or his or her agent receives a request to rent a residential property from an applicant, the landlord may charge that applicant an application screening fee.`

func TestWidenQuote_subsection(t *testing.T) {
	got, why := widenQuote(statute, "in addition to any rent for the first month paid on or before initial occupancy", "Civ. Code § 1950.5(c)")
	if why != "" {
		t.Fatal(why)
	}
	if !strings.HasPrefix(got, "(c) A landlord may not demand") || !strings.HasSuffix(got, "six months or more.") {
		t.Errorf("widened to %q, want subdivision (c) whole", got)
	}
	// A paragraph inside a subdivision ends at the next paragraph.
	got, why = widenQuote(statute, "exclusive of ordinary wear and tear", "§ 1950.5(b)(2)")
	if why != "" {
		t.Fatal(why)
	}
	if got != "(2) The repair of damages to the premises, exclusive of ordinary wear and tear, caused by the tenant or by a guest or licensee of the tenant." {
		t.Errorf("paragraph (b)(2) = %q", got)
	}
	// The last paragraph of a subdivision ends at the next subdivision.
	got, why = widenQuote(statute, "necessary to return the unit to the same level of cleanliness", "§ 1950.5(b)(3)")
	if why != "" {
		t.Fatal(why)
	}
	if !strings.HasPrefix(got, "(3) The cleaning") || !strings.HasSuffix(got, "after January 1, 2003.") {
		t.Errorf("paragraph (b)(3) = %q", got)
	}
	// The last subdivision of a section ends at the next section heading.
	got, why = widenQuote(statute, "held by the landlord for the tenant", "§ 1950.5(d)")
	if why != "" {
		t.Fatal(why)
	}
	if got != "(d) Any security shall be held by the landlord for the tenant who is party to the lease or agreement." {
		t.Errorf("subdivision (d) = %q", got)
	}
	// A quote that opens with the marker starts there.
	got, why = widenQuote(statute, "(1) The compensation of a landlord", "§ 1950.5(b)(1)")
	if why != "" || !strings.HasSuffix(got, "payment of rent.") {
		t.Errorf("paragraph (b)(1) = %q, %s", got, why)
	}
}

func TestWidenQuote_refusals(t *testing.T) {
	if _, why := widenQuote(statute, "words that are not on the page", "§ 1950.5(c)"); !strings.Contains(why, "not found") {
		t.Errorf("missing quote: %q", why)
	}
	if _, why := widenQuote(statute, "held by the landlord for the tenant", "§ 1950.5(z)"); !strings.Contains(why, "marker (z)") {
		t.Errorf("missing marker: %q", why)
	}
	long := "(a) " + strings.Repeat("word ", widenCap+5) + "end. (b) Next."
	if _, why := widenQuote(long, "word word word", "§ 1(a)"); !strings.Contains(why, "cap") {
		t.Errorf("over cap: %q", why)
	}
}

func TestWidenQuote_wholeSection(t *testing.T) {
	text := "RCW 59.18.060 Landlord duties. The landlord will at all times during the tenancy keep the premises fit for human habitation. In each instance the burden shall be on the landlord. RCW 59.18.070 Landlord failure to remedy. If at any time during the tenancy the landlord fails to carry out the duties required by RCW 59.18.060, the tenant may deliver written notice. RCW 59.18.080 Payment of rent condition."
	got, why := widenQuote(text, "the tenant may deliver written notice", "RCW 59.18.070")
	if why != "" {
		t.Fatal(why)
	}
	if !strings.HasPrefix(got, "RCW 59.18.070 Landlord failure") || !strings.HasSuffix(got, "written notice.") {
		t.Errorf("whole section = %q", got)
	}
}

func TestWidenQuote_trailersAndInsertedSubsections(t *testing.T) {
	ca := "(m) Something else. (n) This section shall remain in effect only until January 1, 2030, and as of that date is repealed. (Amended by Stats. 2025, Ch. 203, Sec. 1. (AB 1529) Effective January 1, 2026.)"
	got, why := widenQuote(ca, "This section shall remain in effect only until January 1, 2030, and as of that date is repealed.", "Civ. Code § 1946.2(n)")
	if why != "" || got != "(n) This section shall remain in effect only until January 1, 2030, and as of that date is repealed." {
		t.Errorf("trailer not cut: %q %s", got, why)
	}
	tx := "(a) A landlord may not collect a late fee unless: (1) notice of the fee is included in a written lease; (2) the fee is reasonable; and (3) any portion of the rent has remained unpaid two full days after the date the rent was originally due. (a-1) For purposes of this section, a late fee is considered reasonable if: (1) the late fee is not more than 12 percent. (b) A late fee may include an initial fee."
	got, why = widenQuote(tx, "remained unpaid two full days", "§ 92.019(a)(3)")
	if why != "" || got != "(3) any portion of the rent has remained unpaid two full days after the date the rent was originally due." {
		t.Errorf("(a)(3) = %q %s", got, why)
	}
	got, why = widenQuote(tx, "notice of the fee is included", "§ 92.019(a)")
	if why != "" || !strings.HasSuffix(got, "originally due.") || !strings.HasPrefix(got, "(a) A landlord") {
		t.Errorf("(a) = %q %s", got, why)
	}
}

func TestWidenQuote_firstSubsectionAfterHeading(t *testing.T) {
	text := "Prop. Code Section 92.052 Landlord’s Duty to Repair or Remedy (a) A landlord shall make a diligent effort to repair or remedy a condition if: (1) the tenant specifies the condition in a notice; and (2) the tenant is not delinquent in the payment of rent. (b) Unless the condition was caused by normal wear and tear, the landlord does not have a duty."
	got, why := widenQuote(text, "the tenant is not delinquent in the payment of rent", "Tex. Prop. Code § 92.052(a)")
	if why != "" || !strings.HasPrefix(got, "(a) A landlord shall") || !strings.HasSuffix(got, "payment of rent.") {
		t.Errorf("(a) after a heading = %q %s", got, why)
	}
}

func TestWidenQuote_fusedMarkersAndChrome(t *testing.T) {
	wa := "(10) Provide reasonable locks. (11) Provide facilities adequate to supply heat and water and hot water as reasonably required by the tenant; (12)(a) The landlord may not effect an involuntary change. (b) More."
	got, why := widenQuote(wa, "supply heat and water", "RCW 59.18.060(11)")
	if why != "" || got != "(11) Provide facilities adequate to supply heat and water and hot water as reasonably required by the tenant;" {
		t.Errorf("(11) before (12)(a) = %q %s", got, why)
	}
	ny := "2. Something. 3. Any failure to comply with this section is a misdemeanor. NYSenate.gov Socials Follow the New York State Senate"
	got, why = widenQuote(ny, "is a misdemeanor", "GOL § 7-105(3)")
	if why != "" || got != "3. Any failure to comply with this section is a misdemeanor." {
		t.Errorf("chrome not cut = %q %s", got, why)
	}
}

func TestNextLabel(t *testing.T) {
	for in, want := range map[string]string{"a": "b", "z": "", "B": "C", "3": "4", "9": "10", "ii": "iii", "iv": "v", "IV": "V"} {
		if got := nextLabel(in); got != want {
			t.Errorf("nextLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
