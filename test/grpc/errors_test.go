//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestErrorsAuth(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	// Setup: register a user for duplicate tests
	_, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "err_" + uid,
			Email:    "err_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser setup: %v", err)
	}

	// ── Register: missing username ─────────────────────────────────────────────

	_, err = users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{Email: "nousername@test.com", Password: "pass"},
	})
	if err == nil {
		t.Error("register missing username: want error, got nil")
	}

	// ── Register: missing email ────────────────────────────────────────────────

	_, err = users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{Username: "nomail_" + uid, Password: "pass"},
	})
	if err == nil {
		t.Error("register missing email: want error, got nil")
	}

	// ── Register: missing password ─────────────────────────────────────────────

	_, err = users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{Username: "nopass_" + uid, Email: "nopass_" + uid + "@test.com"},
	})
	if err == nil {
		t.Error("register missing password: want error, got nil")
	}

	// ── Register: duplicate username ───────────────────────────────────────────

	_, err = users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "err_" + uid,
			Email:    "other_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err == nil {
		t.Error("register duplicate username: want error, got nil")
	}

	// ── Register: duplicate email ──────────────────────────────────────────────

	_, err = users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "other_" + uid,
			Email:    "err_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err == nil {
		t.Error("register duplicate email: want error, got nil")
	}

	// ── Login: wrong password ──────────────────────────────────────────────────

	_, err = users.LoginUser(ctx, &pb.LoginUserRequest{
		User: &pb.LoginUserRequestInner{
			Email:    "err_" + uid + "@test.com",
			Password: "wrongpassword",
		},
	})
	if err == nil {
		t.Error("login wrong password: want error, got nil")
	}

	// ── Login: unknown email ───────────────────────────────────────────────────

	_, err = users.LoginUser(ctx, &pb.LoginUserRequest{
		User: &pb.LoginUserRequestInner{
			Email:    "nobody_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err == nil {
		t.Error("login unknown email: want error, got nil")
	}

	// ── GetUser: no token ──────────────────────────────────────────────────────

	_, err = users.GetUser(ctx, &emptypb.Empty{})
	if err == nil {
		t.Error("getUser no token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("getUser no token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UpdateUser: no token ───────────────────────────────────────────────────

	_, err = users.UpdateUser(ctx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{},
	})
	if err == nil {
		t.Error("updateUser no token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("updateUser no token code: got %v, want Unauthenticated", status.Code(err))
	}
}

func TestErrorsArticles(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	// Setup: register owner and a second user
	r1, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "artowner_" + uid,
			Email:    "artowner_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser owner: %v", err)
	}
	ownerCtx := withToken(ctx, r1.GetUser().GetToken())

	r2, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "artother_" + uid,
			Email:    "artother_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser other: %v", err)
	}
	otherCtx := withToken(ctx, r2.GetUser().GetToken())

	// ── CreateArticle: no auth ─────────────────────────────────────────────────

	_, err = articles.CreateArticle(ctx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Unauthed Article",
			Description: "d",
			Body:        "b",
		},
	})
	if err == nil {
		t.Error("createArticle no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("createArticle no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── CreateArticle: missing title ───────────────────────────────────────────

	_, err = articles.CreateArticle(ownerCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{Description: "d", Body: "b"},
	})
	if err == nil {
		t.Error("createArticle missing title: want error, got nil")
	}

	// ── GetArticleBySlug: non-existent ─────────────────────────────────────────

	_, err = articles.GetArticleBySlug(ctx, &pb.GetArticleBySlugRequest{Slug: "does-not-exist-" + uid})
	if err == nil {
		t.Error("getArticle non-existent: want error, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("getArticle non-existent code: got %v, want NotFound", status.Code(err))
	}

	// Setup: create article as owner for subsequent tests
	artResp, err := articles.CreateArticle(ownerCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Error Test Article " + uid,
			Description: "d",
			Body:        "b",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle setup: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(ownerCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	// ── UpdateArticle: no auth ─────────────────────────────────────────────────

	updBody := strPtr("new body")
	_, err = articles.UpdateArticle(ctx, &pb.UpdateArticleRequest{
		Slug:    slug,
		Article: &pb.UpdateArticleRequestInner{Body: updBody},
	})
	if err == nil {
		t.Error("updateArticle no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("updateArticle no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── DeleteArticle: no auth ─────────────────────────────────────────────────

	_, err = articles.DeleteArticle(ctx, &pb.DeleteArticleRequest{Slug: slug})
	if err == nil {
		t.Error("deleteArticle no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("deleteArticle no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── FavoriteArticle: no auth ───────────────────────────────────────────────

	_, err = articles.FavoriteArticle(ctx, &pb.FavoriteArticleRequest{Slug: slug})
	if err == nil {
		t.Error("favoriteArticle no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("favoriteArticle no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UpdateArticle: wrong user ──────────────────────────────────────────────

	_, err = articles.UpdateArticle(otherCtx, &pb.UpdateArticleRequest{
		Slug:    slug,
		Article: &pb.UpdateArticleRequestInner{Body: updBody},
	})
	if err == nil {
		t.Error("updateArticle wrong user: want error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("updateArticle wrong user code: got %v, want PermissionDenied", status.Code(err))
	}

	// ── DeleteArticle: wrong user ──────────────────────────────────────────────

	_, err = articles.DeleteArticle(otherCtx, &pb.DeleteArticleRequest{Slug: slug})
	if err == nil {
		t.Error("deleteArticle wrong user: want error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("deleteArticle wrong user code: got %v, want PermissionDenied", status.Code(err))
	}
}

func TestErrorsComments(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	comments := pb.NewCommentServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	// Setup: owner and other user
	r1, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "cmtowner_" + uid,
			Email:    "cmtowner_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser owner: %v", err)
	}
	ownerCtx := withToken(ctx, r1.GetUser().GetToken())

	r2, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "cmtother_" + uid,
			Email:    "cmtother_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser other: %v", err)
	}
	otherCtx := withToken(ctx, r2.GetUser().GetToken())

	artResp, err := articles.CreateArticle(ownerCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Comment Error Test " + uid,
			Description: "d",
			Body:        "b",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle setup: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(ownerCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	// ── CreateComment: no auth ─────────────────────────────────────────────────

	_, err = comments.CreateComment(ctx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: "nope"},
	})
	if err == nil {
		t.Error("createComment no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("createComment no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── CreateComment: bad slug ────────────────────────────────────────────────

	_, err = comments.CreateComment(ownerCtx, &pb.CreateCommentRequest{
		Slug:    "no-such-article-" + uid,
		Comment: &pb.CreateCommentRequestInner{Body: "nope"},
	})
	if err == nil {
		t.Error("createComment bad slug: want error, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("createComment bad slug code: got %v, want NotFound", status.Code(err))
	}

	// Setup: create a comment as owner for delete tests
	cmtResp, err := comments.CreateComment(ownerCtx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: "owner comment"},
	})
	if err != nil {
		t.Fatalf("CreateComment setup: %v", err)
	}
	commentID := cmtResp.GetComment().GetId()
	t.Cleanup(func() {
		comments.DeleteComment(ownerCtx, &pb.DeleteCommentRequest{Slug: slug, Id: commentID}) //nolint:errcheck
	})

	// ── DeleteComment: no auth ─────────────────────────────────────────────────

	_, err = comments.DeleteComment(ctx, &pb.DeleteCommentRequest{Slug: slug, Id: commentID})
	if err == nil {
		t.Error("deleteComment no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("deleteComment no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── DeleteComment: wrong user ──────────────────────────────────────────────

	_, err = comments.DeleteComment(otherCtx, &pb.DeleteCommentRequest{Slug: slug, Id: commentID})
	if err == nil {
		t.Error("deleteComment wrong user: want error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("deleteComment wrong user code: got %v, want PermissionDenied", status.Code(err))
	}
}

func TestErrorsProfiles(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	profiles := pb.NewProfileServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	r1, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "profmain_" + uid,
			Email:    "profmain_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser main: %v", err)
	}
	mainCtx := withToken(ctx, r1.GetUser().GetToken())

	// ── GetProfile: non-existent user ─────────────────────────────────────────

	_, err = profiles.GetProfile(ctx, &pb.GetProfileRequest{Username: "nobody_" + uid})
	if err == nil {
		t.Error("getProfile non-existent: want error, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("getProfile non-existent code: got %v, want NotFound", status.Code(err))
	}

	// ── FollowUser: no auth ────────────────────────────────────────────────────

	_, err = profiles.FollowUser(ctx, &pb.FollowUserRequest{Username: "profmain_" + uid})
	if err == nil {
		t.Error("followUser no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("followUser no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UnfollowUser: no auth ──────────────────────────────────────────────────

	_, err = profiles.UnfollowUser(ctx, &pb.UnfollowUserRequest{Username: "profmain_" + uid})
	if err == nil {
		t.Error("unfollowUser no auth: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("unfollowUser no auth code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UnfollowUser: unknown user ─────────────────────────────────────────────

	_, err = profiles.UnfollowUser(mainCtx, &pb.UnfollowUserRequest{Username: "nobody_" + uid})
	if err == nil {
		t.Error("unfollowUser unknown: want error, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("unfollowUser unknown code: got %v, want NotFound", status.Code(err))
	}
}

func TestErrorsAuthorization(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	profiles := pb.NewProfileServiceClient(conn)
	comments := pb.NewCommentServiceClient(conn)
	ctx := context.Background()

	badCtx := withToken(ctx, "invalid-token-xyz")
	uid := genUID()

	// Setup: a valid article slug for endpoints that need one
	r, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "authz_" + uid,
			Email:    "authz_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser setup: %v", err)
	}
	validCtx := withToken(ctx, r.GetUser().GetToken())

	artResp, err := articles.CreateArticle(validCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Authz Test " + uid,
			Description: "d",
			Body:        "b",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle setup: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(validCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	// ── GetUser: invalid token ─────────────────────────────────────────────────

	_, err = users.GetUser(badCtx, &emptypb.Empty{})
	if err == nil {
		t.Error("getUser invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("getUser invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UpdateUser: invalid token ──────────────────────────────────────────────

	_, err = users.UpdateUser(badCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{},
	})
	if err == nil {
		t.Error("updateUser invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("updateUser invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── CreateArticle: invalid token ───────────────────────────────────────────

	_, err = articles.CreateArticle(badCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{Title: "x", Description: "d", Body: "b"},
	})
	if err == nil {
		t.Error("createArticle invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("createArticle invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UpdateArticle: invalid token ───────────────────────────────────────────

	updBody := strPtr("x")
	_, err = articles.UpdateArticle(badCtx, &pb.UpdateArticleRequest{
		Slug:    slug,
		Article: &pb.UpdateArticleRequestInner{Body: updBody},
	})
	if err == nil {
		t.Error("updateArticle invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("updateArticle invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── DeleteArticle: invalid token ───────────────────────────────────────────

	_, err = articles.DeleteArticle(badCtx, &pb.DeleteArticleRequest{Slug: slug})
	if err == nil {
		t.Error("deleteArticle invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("deleteArticle invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── FavoriteArticle: invalid token ─────────────────────────────────────────

	_, err = articles.FavoriteArticle(badCtx, &pb.FavoriteArticleRequest{Slug: slug})
	if err == nil {
		t.Error("favoriteArticle invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("favoriteArticle invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── FeedArticles: invalid token ────────────────────────────────────────────

	_, err = articles.FeedArticles(badCtx, &pb.FeedArticlesRequest{Limit: 20, Offset: 0})
	if err == nil {
		t.Error("feedArticles invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("feedArticles invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── FollowUser: invalid token ──────────────────────────────────────────────

	_, err = profiles.FollowUser(badCtx, &pb.FollowUserRequest{Username: "authz_" + uid})
	if err == nil {
		t.Error("followUser invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("followUser invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── UnfollowUser: invalid token ────────────────────────────────────────────

	_, err = profiles.UnfollowUser(badCtx, &pb.UnfollowUserRequest{Username: "authz_" + uid})
	if err == nil {
		t.Error("unfollowUser invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("unfollowUser invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── CreateComment: invalid token ───────────────────────────────────────────

	_, err = comments.CreateComment(badCtx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: "x"},
	})
	if err == nil {
		t.Error("createComment invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("createComment invalid token code: got %v, want Unauthenticated", status.Code(err))
	}

	// ── DeleteComment: invalid token ───────────────────────────────────────────

	_, err = comments.DeleteComment(badCtx, &pb.DeleteCommentRequest{Slug: slug, Id: 1})
	if err == nil {
		t.Error("deleteComment invalid token: want error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("deleteComment invalid token code: got %v, want Unauthenticated", status.Code(err))
	}
}
