package templates

import "testing"

func TestPlaceCount(t *testing.T) {
	fifty := make([]string, 50)
	for i := range fifty {
		fifty[i] = "state"
	}
	cases := []struct {
		states []string
		cities int
		want   string
	}{
		{nil, 3, "3 cities"},
		{[]string{"ohio"}, 0, "1 state"},
		{[]string{"ohio", "utah"}, 12, "2 states and 12 cities"},
		{append(fifty, "washington-dc"), 40, "50 states, DC, and 40 cities"},
		{append(fifty, "guam", "puerto-rico", "washington-dc"), 40, "50 states, Guam, Puerto Rico, DC, and 40 cities"},
		{[]string{"washington-dc"}, 0, "DC"},
		{nil, 0, "0 cities"},
	}
	for _, c := range cases {
		if got := PlaceCount(c.states, c.cities); got != c.want {
			t.Errorf("PlaceCount(%v, %d) = %q, want %q", len(c.states), c.cities, got, c.want)
		}
	}
}
