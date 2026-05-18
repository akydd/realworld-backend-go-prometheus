//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestFeed(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	profiles := pb.NewProfileServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	mainResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "feedm_" + uid,
			Email:    "feedm_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser main: %v", err)
	}
	mainCtx := withToken(ctx, mainResp.GetUser().GetToken())

	celebResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "feedc_" + uid,
			Email:    "feedc_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser celeb: %v", err)
	}
	celebCtx := withToken(ctx, celebResp.GetUser().GetToken())

	// ── Empty feed before following ────────────────────────────────────────────

	feedResp, err := articles.FeedArticles(mainCtx, &pb.FeedArticlesRequest{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("FeedArticles empty: %v", err)
	}
	if feedResp.GetArticlesCount() != 0 {
		t.Errorf("empty feed count: got %d, want 0", feedResp.GetArticlesCount())
	}
	if got := len(feedResp.GetArticles()); got != 0 {
		t.Errorf("empty feed array: got %d, want 0", got)
	}

	// ── Follow celeb ───────────────────────────────────────────────────────────

	_, err = profiles.FollowUser(mainCtx, &pb.FollowUserRequest{Username: "feedc_" + uid})
	if err != nil {
		t.Fatalf("FollowUser: %v", err)
	}

	// ── Celeb creates two articles ─────────────────────────────────────────────

	art1, err := articles.CreateArticle(celebCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Feed Article 1",
			Description: "d",
			Body:        "b",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle 1: %v", err)
	}
	slug1 := art1.GetArticle().GetSlug()

	art2, err := articles.CreateArticle(celebCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Feed Article 2",
			Description: "d",
			Body:        "b",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle 2: %v", err)
	}
	slug2 := art2.GetArticle().GetSlug()

	t.Cleanup(func() {
		articles.DeleteArticle(celebCtx, &pb.DeleteArticleRequest{Slug: slug1}) //nolint:errcheck
		articles.DeleteArticle(celebCtx, &pb.DeleteArticleRequest{Slug: slug2}) //nolint:errcheck
		profiles.UnfollowUser(mainCtx, &pb.UnfollowUserRequest{Username: "feedc_" + uid}) //nolint:errcheck
	})

	// ── Main sees both articles ────────────────────────────────────────────────

	feedResp, err = articles.FeedArticles(mainCtx, &pb.FeedArticlesRequest{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("FeedArticles full: %v", err)
	}
	if feedResp.GetArticlesCount() != 2 {
		t.Errorf("feed count: got %d, want 2", feedResp.GetArticlesCount())
	}
	if got := feedResp.GetArticles()[0].GetAuthor().GetUsername(); got != "feedc_"+uid {
		t.Errorf("feed author: got %q, want %q", got, "feedc_"+uid)
	}

	// ── Limit 1 returns most recent (slug2) ───────────────────────────────────

	feedResp, err = articles.FeedArticles(mainCtx, &pb.FeedArticlesRequest{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("FeedArticles limit 1: %v", err)
	}
	if got := len(feedResp.GetArticles()); got != 1 {
		t.Errorf("feed limit 1 len: got %d, want 1", got)
	}
	if feedResp.GetArticlesCount() != 2 {
		t.Errorf("feed limit 1 total: got %d, want 2", feedResp.GetArticlesCount())
	}
	if got := feedResp.GetArticles()[0].GetSlug(); got != slug2 {
		t.Errorf("feed limit 1 slug: got %q, want %q", got, slug2)
	}

	// ── Offset 1 returns older article (slug1) ────────────────────────────────

	feedResp, err = articles.FeedArticles(mainCtx, &pb.FeedArticlesRequest{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("FeedArticles offset 1: %v", err)
	}
	if got := feedResp.GetArticles()[0].GetSlug(); got != slug1 {
		t.Errorf("feed offset 1 slug: got %q, want %q", got, slug1)
	}
}
