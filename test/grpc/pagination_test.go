//go:build integration

package grpc_test

import (
	"context"
	"fmt"
	"testing"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestPagination(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "pag_" + uid,
			Email:    "pag_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	authedCtx := withToken(ctx, regResp.GetUser().GetToken())

	// Create 5 articles
	slugs := make([]string, 5)
	for i := range 5 {
		resp, err := articles.CreateArticle(authedCtx, &pb.CreateArticleRequest{
			Article: &pb.CreateArticleRequestInner{
				Title:       fmt.Sprintf("Pag Article %d %s", i+1, uid),
				Description: "d",
				Body:        "b",
			},
		})
		if err != nil {
			t.Fatalf("CreateArticle %d: %v", i+1, err)
		}
		slugs[i] = resp.GetArticle().GetSlug()
	}
	t.Cleanup(func() {
		for _, s := range slugs {
			articles.DeleteArticle(authedCtx, &pb.DeleteArticleRequest{Slug: s}) //nolint:errcheck
		}
	})

	author := "pag_" + uid

	// ── Total count ────────────────────────────────────────────────────────────

	listResp, err := articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author, Limit: 20, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticles total: %v", err)
	}
	if listResp.GetArticlesCount() != 5 {
		t.Errorf("total count: got %d, want 5", listResp.GetArticlesCount())
	}
	if got := len(listResp.GetArticles()); got != 5 {
		t.Errorf("total array len: got %d, want 5", got)
	}

	// ── Limit 2 offset 0 ──────────────────────────────────────────────────────

	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author, Limit: 2, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticles limit 2: %v", err)
	}
	if got := len(listResp.GetArticles()); got != 2 {
		t.Errorf("limit 2 len: got %d, want 2", got)
	}
	if listResp.GetArticlesCount() != 5 {
		t.Errorf("limit 2 total: got %d, want 5", listResp.GetArticlesCount())
	}

	// ── Limit 2 offset 2 ──────────────────────────────────────────────────────

	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author, Limit: 2, Offset: 2,
	})
	if err != nil {
		t.Fatalf("ListArticles offset 2: %v", err)
	}
	if got := len(listResp.GetArticles()); got != 2 {
		t.Errorf("offset 2 len: got %d, want 2", got)
	}
	if listResp.GetArticlesCount() != 5 {
		t.Errorf("offset 2 total: got %d, want 5", listResp.GetArticlesCount())
	}

	// ── Limit 2 offset 4 — only 1 remains ────────────────────────────────────

	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author, Limit: 2, Offset: 4,
	})
	if err != nil {
		t.Fatalf("ListArticles offset 4: %v", err)
	}
	if got := len(listResp.GetArticles()); got != 1 {
		t.Errorf("offset 4 len: got %d, want 1", got)
	}
	if listResp.GetArticlesCount() != 5 {
		t.Errorf("offset 4 total: got %d, want 5", listResp.GetArticlesCount())
	}

	// ── Limit 2 offset 5 — empty page ─────────────────────────────────────────

	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author, Limit: 2, Offset: 5,
	})
	if err != nil {
		t.Fatalf("ListArticles offset 5: %v", err)
	}
	if got := len(listResp.GetArticles()); got != 0 {
		t.Errorf("offset 5 len: got %d, want 0", got)
	}
	if listResp.GetArticlesCount() != 5 {
		t.Errorf("offset 5 total: got %d, want 5", listResp.GetArticlesCount())
	}

	// ── Most recent first ──────────────────────────────────────────────────────

	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author, Limit: 1, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticles order: %v", err)
	}
	wantTitle := fmt.Sprintf("Pag Article 5 %s", uid)
	if got := listResp.GetArticles()[0].GetTitle(); got != wantTitle {
		t.Errorf("order first is newest: got %q, want %q", got, wantTitle)
	}
}
