//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestComments(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	comments := pb.NewCommentServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "cmt_" + uid,
			Email:    "cmt_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	token := regResp.GetUser().GetToken()
	authedCtx := withToken(ctx, token)

	artResp, err := articles.CreateArticle(authedCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Comment Test",
			Description: "desc",
			Body:        "body",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(authedCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	// ── Create comment ─────────────────────────────────────────────────────────

	cmtResp, err := comments.CreateComment(authedCtx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: "Test comment body"},
	})
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	commentID := cmtResp.GetComment().GetId()
	if commentID == 0 {
		t.Error("create comment id: got 0, want non-zero")
	}
	if got := cmtResp.GetComment().GetBody(); got != "Test comment body" {
		t.Errorf("create comment body: got %q, want %q", got, "Test comment body")
	}
	if got := cmtResp.GetComment().GetAuthor().GetUsername(); got != "cmt_"+uid {
		t.Errorf("create comment author: got %q, want %q", got, "cmt_"+uid)
	}

	// ── List comments (authenticated) ──────────────────────────────────────────

	listResp, err := comments.GetComments(authedCtx, &pb.GetCommentsRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetComments authed: %v", err)
	}
	if got := len(listResp.GetComments()); got != 1 {
		t.Errorf("list comments count: got %d, want 1", got)
	}
	if got := listResp.GetComments()[0].GetBody(); got != "Test comment body" {
		t.Errorf("list comments body: got %q, want %q", got, "Test comment body")
	}

	// ── List comments (no auth) ────────────────────────────────────────────────

	listResp, err = comments.GetComments(ctx, &pb.GetCommentsRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetComments no-auth: %v", err)
	}
	if got := len(listResp.GetComments()); got != 1 {
		t.Errorf("list no-auth count: got %d, want 1", got)
	}
	if listResp.GetComments()[0].GetAuthor().GetFollowing() {
		t.Error("list no-auth following: got true, want false")
	}

	// ── Delete comment ─────────────────────────────────────────────────────────

	_, err = comments.DeleteComment(authedCtx, &pb.DeleteCommentRequest{Slug: slug, Id: commentID})
	if err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}

	listResp, err = comments.GetComments(ctx, &pb.GetCommentsRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetComments after delete: %v", err)
	}
	if got := len(listResp.GetComments()); got != 0 {
		t.Errorf("verify delete: got %d comments, want 0", got)
	}

	// ── Selective deletion: create two, delete first, verify second remains ────

	r1, err := comments.CreateComment(authedCtx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: "First comment"},
	})
	if err != nil {
		t.Fatalf("CreateComment first: %v", err)
	}
	id1 := r1.GetComment().GetId()

	r2, err := comments.CreateComment(authedCtx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: "Second comment"},
	})
	if err != nil {
		t.Fatalf("CreateComment second: %v", err)
	}
	id2 := r2.GetComment().GetId()

	listResp, err = comments.GetComments(ctx, &pb.GetCommentsRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetComments two: %v", err)
	}
	if got := len(listResp.GetComments()); got != 2 {
		t.Errorf("two comments exist: got %d, want 2", got)
	}

	_, err = comments.DeleteComment(authedCtx, &pb.DeleteCommentRequest{Slug: slug, Id: id1})
	if err != nil {
		t.Fatalf("DeleteComment first: %v", err)
	}

	listResp, err = comments.GetComments(ctx, &pb.GetCommentsRequest{Slug: slug})
	if err != nil {
		t.Fatalf("GetComments after selective delete: %v", err)
	}
	if got := len(listResp.GetComments()); got != 1 {
		t.Errorf("only second remains count: got %d, want 1", got)
	}
	if got := listResp.GetComments()[0].GetBody(); got != "Second comment" {
		t.Errorf("second comment body: got %q, want %q", got, "Second comment")
	}

	// Cleanup remaining comment
	comments.DeleteComment(authedCtx, &pb.DeleteCommentRequest{Slug: slug, Id: id2}) //nolint:errcheck
}
