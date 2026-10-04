package store_test

import (
	"context"
	"testing"
)

func TestRecordClick_oncePerReaderPerSourceOnLivePagesOnly(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	_, _, orgStmt, host, _ := helpPage(t, pg, jID, tID)

	var src int64
	if err := pg.Pool().QueryRow(ctx,
		`SELECT source_id FROM citations WHERE statement_id = $1`, orgStmt).Scan(&src); err != nil {
		t.Fatal(err)
	}
	a, b := newClient(t), newClient(t)
	cases := []struct {
		name   string
		source int64
		client string
		want   bool
	}{
		{"first click", src, a, true},
		{"same reader again", src, a, false},
		{"another reader", src, b, true},
		{"a source no live page cites", 999999999, newClient(t), false},
		{"no address", src, "", false},
	}
	for _, c := range cases {
		got, err := pg.RecordClick(ctx, c.source, c.client)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: counted = %v, want %v", c.name, got, c.want)
		}
	}

	orgs, err := pg.ListHelpOrgs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range orgs {
		if o.Host == host {
			if o.Clicks30 != 2 {
				t.Fatalf("org clicks in 30 days = %d, want 2", o.Clicks30)
			}
			if o.LivePages != 1 {
				t.Fatalf("org live pages = %d, want 1", o.LivePages)
			}
			return
		}
	}
	t.Fatalf("org %s not on the Orgs list", host)
}
