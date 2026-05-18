//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestArticles(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	// Setup: register user
	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "art_" + uid,
			Email:    "art_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	token := regResp.GetUser().GetToken()
	authedCtx := withToken(ctx, token)

	// ── Create article with tags ───────────────────────────────────────────────

	createResp, err := articles.CreateArticle(authedCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Test Article " + uid,
			Description: "Test description",
			Body:        "Test body content",
			TagList:     []string{"d_" + uid, "t_" + uid},
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	a := createResp.GetArticle()
	slug := a.GetSlug()
	createdAt := a.GetCreatedAt()

	if got := a.GetTitle(); got != "Test Article "+uid {
		t.Errorf("create title: got %q, want %q", got, "Test Article "+uid)
	}
	if got := a.GetDescription(); got != "Test description" {
		t.Errorf("create description: got %q, want %q", got, "Test description")
	}
	if got := a.GetBody(); got != "Test body content" {
		t.Errorf("create body: got %q, want %q", got, "Test body content")
	}
	if slug == "" {
		t.Error("create slug: got empty, want non-empty")
	}
	if createdAt == nil {
		t.Error("create createdAt: got nil, want non-nil")
	}
	if a.GetFavorited() {
		t.Error("create favorited: got true, want false")
	}
	if a.GetFavoritesCount() != 0 {
		t.Errorf("create favoritesCount: got %d, want 0", a.GetFavoritesCount())
	}
	if got := a.GetAuthor().GetUsername(); got != "art_"+uid {
		t.Errorf("create author: got %q, want %q", got, "art_"+uid)
	}

	// ── List all articles (no auth) ────────────────────────────────────────────

	listResp, err := articles.ListArticles(ctx, &pb.ListArticlesRequest{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("ListArticles all: %v", err)
	}
	if listResp.GetArticlesCount() < 1 {
		t.Errorf("list all count: got %d, want >= 1", listResp.GetArticlesCount())
	}

	// ── List by author (no auth) ───────────────────────────────────────────────

	author := "art_" + uid
	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Author: &author,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticles by author: %v", err)
	}
	if listResp.GetArticlesCount() != 1 {
		t.Errorf("list by author count: got %d, want 1", listResp.GetArticlesCount())
	}
	if got := listResp.GetArticles()[0].GetAuthor().GetUsername(); got != "art_"+uid {
		t.Errorf("list by author username: got %q, want %q", got, "art_"+uid)
	}

	// ── List all with auth ─────────────────────────────────────────────────────

	listResp, err = articles.ListArticles(authedCtx, &pb.ListArticlesRequest{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("ListArticles authed: %v", err)
	}
	if listResp.GetArticlesCount() < 1 {
		t.Errorf("list auth count: got %d, want >= 1", listResp.GetArticlesCount())
	}

	// ── List by tag ────────────────────────────────────────────────────────────

	tag := "d_" + uid
	listResp, err = articles.ListArticles(ctx, &pb.ListArticlesRequest{
		Tag:    &tag,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListArticles by tag: %v", err)
	}
	if listResp.GetArticlesCount() < 1 {
		t.Errorf("list by tag count: got %d, want >= 1", listResp.GetArticlesCount())
	}

	// ── Get single article ─────────────────────────────────────────────────────

	getResp, err := articles.GetArticleBySlug(ctx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetArticleBySlug: %v", err)
	}
	a = getResp.GetArticle()
	if a.GetSlug() != slug {
		t.Errorf("get slug: got %q, want %q", a.GetSlug(), slug)
	}
	if a.GetTitle() != "Test Article "+uid {
		t.Errorf("get title: got %q, want %q", a.GetTitle(), "Test Article "+uid)
	}
	if a.GetBody() != "Test body content" {
		t.Errorf("get body: got %q, want %q", a.GetBody(), "Test body content")
	}
	if a.GetAuthor().GetUsername() != "art_"+uid {
		t.Errorf("get author: got %q, want %q", a.GetAuthor().GetUsername(), "art_"+uid)
	}

	// ── Update body ────────────────────────────────────────────────────────────

	updBody := strPtr("Updated body content")
	updResp, err := articles.UpdateArticle(authedCtx, &pb.UpdateArticleRequest{
		Slug:    slug,
		Article: &pb.UpdateArticleRequestInner{Body: updBody},
	})
	if err != nil {
		t.Fatalf("UpdateArticle body: %v", err)
	}
	a = updResp.GetArticle()
	if a.GetBody() != "Updated body content" {
		t.Errorf("update body: got %q, want %q", a.GetBody(), "Updated body content")
	}
	if a.GetTitle() != "Test Article "+uid {
		t.Errorf("update title unchanged: got %q, want %q", a.GetTitle(), "Test Article "+uid)
	}
	if got := len(a.GetTagList()); got != 2 {
		t.Errorf("update tagCount: got %d, want 2", got)
	}
	if !a.GetCreatedAt().AsTime().Equal(createdAt.AsTime()) {
		t.Errorf("update createdAt unchanged: got %v, want %v", a.GetCreatedAt(), createdAt)
	}
	if !a.GetUpdatedAt().AsTime().After(createdAt.AsTime()) {
		t.Errorf("update updatedAt changed: got %v, want after %v", a.GetUpdatedAt(), createdAt)
	}

	getResp, err = articles.GetArticleBySlug(ctx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetArticleBySlug after update: %v", err)
	}
	if getResp.GetArticle().GetBody() != "Updated body content" {
		t.Errorf("verify body update: got %q, want %q", getResp.GetArticle().GetBody(), "Updated body content")
	}

	// ── Update without tagList: tags preserved ────────────────────────────────

	noTagBody := strPtr("Body without touching tags")
	updResp, err = articles.UpdateArticle(authedCtx, &pb.UpdateArticleRequest{
		Slug:    slug,
		Article: &pb.UpdateArticleRequestInner{Body: noTagBody},
	})
	if err != nil {
		t.Fatalf("UpdateArticle no tagList: %v", err)
	}
	slug = updResp.GetArticle().GetSlug()
	if updResp.GetArticle().GetBody() != "Body without touching tags" {
		t.Errorf("no tagList body: got %q", updResp.GetArticle().GetBody())
	}
	if got := len(updResp.GetArticle().GetTagList()); got != 2 {
		t.Errorf("no tagList tag count: got %d, want 2", got)
	}

	// ── Remove all tags with empty list ───────────────────────────────────────

	updResp, err = articles.UpdateArticle(authedCtx, &pb.UpdateArticleRequest{
		Slug:    slug,
		Article: &pb.UpdateArticleRequestInner{TagList: &pb.TagListValue{Tags: []string{}}},
	})
	if err != nil {
		t.Fatalf("UpdateArticle remove tags: %v", err)
	}
	if got := len(updResp.GetArticle().GetTagList()); got != 0 {
		t.Errorf("remove tags count: got %d, want 0", got)
	}

	getResp, err = articles.GetArticleBySlug(ctx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetArticleBySlug after remove tags: %v", err)
	}
	if got := len(getResp.GetArticle().GetTagList()); got != 0 {
		t.Errorf("verify tags removed: got %d, want 0", got)
	}

	// ── Delete article ─────────────────────────────────────────────────────────

	_, err = articles.DeleteArticle(authedCtx, &pb.DeleteArticleRequest{Slug: slug})
	if err != nil {
		t.Fatalf("DeleteArticle: %v", err)
	}

	_, err = articles.GetArticleBySlug(ctx, &pb.GetArticleBySlugRequest{Slug: slug})
	if err == nil {
		t.Error("verify deleted: want error, got nil")
	}

	_ = emptypb.Empty{} // ensure import used via dial(t) helper indirectly
}
