//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestTags(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	tags := pb.NewTagServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "tag_" + uid,
			Email:    "tag_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	authedCtx := withToken(ctx, regResp.GetUser().GetToken())

	tagA := "tag_" + uid + "_a"
	tagB := "tag_" + uid + "_b"

	artResp, err := articles.CreateArticle(authedCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Tag test",
			Description: "d",
			Body:        "b",
			TagList:     []string{tagA, tagB},
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(authedCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	tagsResp, err := tags.GetTags(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}

	tagSet := make(map[string]bool, len(tagsResp.GetTags()))
	for _, tg := range tagsResp.GetTags() {
		tagSet[tg] = true
	}
	if !tagSet[tagA] {
		t.Errorf("tag a present: %q not in tags", tagA)
	}
	if !tagSet[tagB] {
		t.Errorf("tag b present: %q not in tags", tagB)
	}
}
