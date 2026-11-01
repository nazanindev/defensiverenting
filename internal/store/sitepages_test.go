package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestSitePageSaveKeepsHistory(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	if _, ok, err := pg.GetSitePage(ctx, "test-page"); err != nil || ok {
		t.Fatalf("fresh page: ok=%v err=%v", ok, err)
	}
	for _, body := range []string{"first", "second"} {
		if err := pg.SaveSitePage(ctx, store.SitePage{Slug: "test-page", Title: "T", Description: "D", Body: body, UpdatedBy: "editor"}); err != nil {
			t.Fatal(err)
		}
	}
	p, ok, err := pg.GetSitePage(ctx, "test-page")
	if err != nil || !ok || p.Body != "second" || p.UpdatedBy != "editor" {
		t.Fatalf("got %+v ok=%v err=%v", p, ok, err)
	}
	var n int
	if err := pg.Pool().QueryRow(ctx, `SELECT count(*) FROM site_page_history WHERE slug = 'test-page'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("history rows = %d, want 2", n)
	}
	all, err := pg.ListSitePages(ctx)
	if err != nil || all["test-page"].Body != "second" {
		t.Fatalf("list: %v %+v", err, all)
	}
	_, _ = pg.Pool().Exec(ctx, `DELETE FROM site_pages WHERE slug = 'test-page'`)
	_, _ = pg.Pool().Exec(ctx, `DELETE FROM site_page_history WHERE slug = 'test-page'`)
}
