//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestFavorites(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	r1, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "fav_" + uid,
			Email:    "fav_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser owner: %v", err)
	}
	ownerCtx := withToken(ctx, r1.GetUser().GetToken())

	r2, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "fav2_" + uid,
			Email:    "fav2_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser fav2: %v", err)
	}
	fav2Ctx := withToken(ctx, r2.GetUser().GetToken())

	artResp, err := articles.CreateArticle(ownerCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Fav Article",
			Description: "d",
			Body:        "b",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(ownerCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	// Article starts unfavorited
	if artResp.GetArticle().GetFavorited() {
		t.Error("initial favorited: got true, want false")
	}
	if artResp.GetArticle().GetFavoritesCount() != 0 {
		t.Errorf("initial favoritesCount: got %d, want 0", artResp.GetArticle().GetFavoritesCount())
	}

	// ── Favorite the article ───────────────────────────────────────────────────

	favResp, err := articles.FavoriteArticle(fav2Ctx, &pb.FavoriteArticleRequest{Slug: slug})
	if err != nil {
		t.Fatalf("FavoriteArticle: %v", err)
	}
	if !favResp.GetArticle().GetFavorited() {
		t.Error("favorite favorited: got false, want true")
	}
	if favResp.GetArticle().GetFavoritesCount() != 1 {
		t.Errorf("favorite count: got %d, want 1", favResp.GetArticle().GetFavoritesCount())
	}

	// ── Get as fav2 — sees favorited=true ────────────────────────────────────

	getResp, err := articles.GetArticleBySlug(fav2Ctx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetArticleBySlug as fav2: %v", err)
	}
	if !getResp.GetArticle().GetFavorited() {
		t.Error("get as fav2 favorited: got false, want true")
	}
	if getResp.GetArticle().GetFavoritesCount() != 1 {
		t.Errorf("get as fav2 count: got %d, want 1", getResp.GetArticle().GetFavoritesCount())
	}

	// ── Get as owner — sees favorited=false (owner didn't favorite) ───────────

	getResp, err = articles.GetArticleBySlug(ownerCtx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetArticleBySlug as owner: %v", err)
	}
	if getResp.GetArticle().GetFavorited() {
		t.Error("get as owner favorited: got true, want false")
	}
	if getResp.GetArticle().GetFavoritesCount() != 1 {
		t.Errorf("get as owner count: got %d, want 1", getResp.GetArticle().GetFavoritesCount())
	}

	// ── List favorited by fav2 ─────────────────────────────────────────────────

	favoritedBy := "fav2_" + uid
	listResp, err := articles.ListArticles(fav2Ctx, &pb.ListArticlesRequest{
		Favorited: &favoritedBy,
		Limit:     20,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListArticles favorited: %v", err)
	}
	if listResp.GetArticlesCount() < 1 {
		t.Errorf("list favorited count: got %d, want >= 1", listResp.GetArticlesCount())
	}

	// ── Unfavorite ─────────────────────────────────────────────────────────────

	unfavResp, err := articles.UnfavoriteArticle(fav2Ctx, &pb.UnfavoriteArticleRequest{Slug: slug})
	if err != nil {
		t.Fatalf("UnfavoriteArticle: %v", err)
	}
	if unfavResp.GetArticle().GetFavorited() {
		t.Error("unfavorite favorited: got true, want false")
	}
	if unfavResp.GetArticle().GetFavoritesCount() != 0 {
		t.Errorf("unfavorite count: got %d, want 0", unfavResp.GetArticle().GetFavoritesCount())
	}

	// ── Verify unfavorite persisted ────────────────────────────────────────────

	getResp, err = articles.GetArticleBySlug(ctx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetArticleBySlug verify: %v", err)
	}
	if getResp.GetArticle().GetFavoritesCount() != 0 {
		t.Errorf("verify unfavorite count: got %d, want 0", getResp.GetArticle().GetFavoritesCount())
	}
}
