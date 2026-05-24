//go:build integration

package grpc_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestLiveArticleFeed(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	profiles := pb.NewProfileServiceClient(conn)
	ctx := context.Background()
	uid := genUID()

	// Register the author.
	authorResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "stream_art_author_" + uid,
			Email:    "stream_art_author_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser author: %v", err)
	}
	authorCtx := withToken(ctx, authorResp.GetUser().GetToken())

	// Register the subscriber, who will follow the author and open the stream.
	subResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "stream_art_sub_" + uid,
			Email:    "stream_art_sub_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser subscriber: %v", err)
	}
	subCtx := withToken(ctx, subResp.GetUser().GetToken())

	// Subscriber follows the author so articles appear in their live feed.
	if _, err = profiles.FollowUser(subCtx, &pb.FollowUserRequest{Username: authorResp.GetUser().GetUsername()}); err != nil {
		t.Fatalf("FollowUser: %v", err)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := articles.LiveArticleFeed(withToken(streamCtx, subResp.GetUser().GetToken()), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("LiveArticleFeed: %v", err)
	}

	received := make(chan *pb.ArticleListItem, 1)
	go func() {
		item, recvErr := stream.Recv()
		if recvErr != nil {
			return
		}
		received <- item
	}()

	time.Sleep(100 * time.Millisecond)

	title := "Streaming Test Article " + uid
	artResp, err := articles.CreateArticle(authorCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       title,
			Description: "desc",
			Body:        "body",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(authorCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	select {
	case item := <-received:
		if item.GetSlug() != slug {
			t.Errorf("streamed slug: got %q, want %q", item.GetSlug(), slug)
		}
		if item.GetTitle() != title {
			t.Errorf("streamed title: got %q, want %q", item.GetTitle(), title)
		}
	case <-time.After(3 * time.Second):
		t.Error("timed out waiting for streamed article")
	}
}

func TestLiveCommentFeed(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	comments := pb.NewCommentServiceClient(conn)
	ctx := context.Background()
	uid := genUID()

	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "stream_cmt_" + uid,
			Email:    "stream_cmt_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	authCtx := withToken(ctx, regResp.GetUser().GetToken())

	artResp, err := articles.CreateArticle(authCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{
			Title:       "Comment Stream Test " + uid,
			Description: "desc",
			Body:        "body",
		},
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	slug := artResp.GetArticle().GetSlug()
	t.Cleanup(func() {
		articles.DeleteArticle(authCtx, &pb.DeleteArticleRequest{Slug: slug}) //nolint:errcheck
	})

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := comments.LiveCommentFeed(streamCtx, &pb.LiveCommentFeedRequest{Slug: slug})
	if err != nil {
		t.Fatalf("LiveCommentFeed: %v", err)
	}

	received := make(chan *pb.CommentResponseInner, 1)
	go func() {
		item, recvErr := stream.Recv()
		if recvErr != nil {
			return
		}
		received <- item
	}()

	time.Sleep(100 * time.Millisecond)

	body := "hello from the stream " + uid
	_, err = comments.CreateComment(authCtx, &pb.CreateCommentRequest{
		Slug:    slug,
		Comment: &pb.CreateCommentRequestInner{Body: body},
	})
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	select {
	case item := <-received:
		if item.GetBody() != body {
			t.Errorf("streamed body: got %q, want %q", item.GetBody(), body)
		}
		if item.GetAuthor().GetUsername() != "stream_cmt_"+uid {
			t.Errorf("streamed author: got %q, want %q", item.GetAuthor().GetUsername(), "stream_cmt_"+uid)
		}
	case <-time.After(3 * time.Second):
		t.Error("timed out waiting for streamed comment")
	}
}

// TestLiveCommentFeedSlugIsolation verifies that comments posted to one article
// do not appear on a stream opened for a different article's slug.
func TestLiveCommentFeedSlugIsolation(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	articles := pb.NewArticleServiceClient(conn)
	comments := pb.NewCommentServiceClient(conn)
	ctx := context.Background()
	uid := genUID()

	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "stream_iso_" + uid,
			Email:    "stream_iso_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	authCtx := withToken(ctx, regResp.GetUser().GetToken())

	a1Resp, err := articles.CreateArticle(authCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{Title: "Iso Article One " + uid, Description: "d", Body: "b"},
	})
	if err != nil {
		t.Fatalf("CreateArticle a1: %v", err)
	}
	slug1 := a1Resp.GetArticle().GetSlug()

	a2Resp, err := articles.CreateArticle(authCtx, &pb.CreateArticleRequest{
		Article: &pb.CreateArticleRequestInner{Title: "Iso Article Two " + uid, Description: "d", Body: "b"},
	})
	if err != nil {
		t.Fatalf("CreateArticle a2: %v", err)
	}
	slug2 := a2Resp.GetArticle().GetSlug()

	t.Cleanup(func() {
		articles.DeleteArticle(authCtx, &pb.DeleteArticleRequest{Slug: slug1}) //nolint:errcheck
		articles.DeleteArticle(authCtx, &pb.DeleteArticleRequest{Slug: slug2}) //nolint:errcheck
	})

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := comments.LiveCommentFeed(streamCtx, &pb.LiveCommentFeedRequest{Slug: slug1})
	if err != nil {
		t.Fatalf("LiveCommentFeed: %v", err)
	}

	received := make(chan *pb.CommentResponseInner, 1)
	go func() {
		item, recvErr := stream.Recv()
		if recvErr != nil {
			return
		}
		received <- item
	}()

	time.Sleep(100 * time.Millisecond)

	// Comment on slug2 must not arrive on the slug1 stream.
	_, err = comments.CreateComment(authCtx, &pb.CreateCommentRequest{
		Slug:    slug2,
		Comment: &pb.CreateCommentRequestInner{Body: "wrong article"},
	})
	if err != nil {
		t.Fatalf("CreateComment on slug2: %v", err)
	}

	// Comment on slug1 must arrive.
	body := "right article " + uid
	_, err = comments.CreateComment(authCtx, &pb.CreateCommentRequest{
		Slug:    slug1,
		Comment: &pb.CreateCommentRequestInner{Body: body},
	})
	if err != nil {
		t.Fatalf("CreateComment on slug1: %v", err)
	}

	select {
	case item := <-received:
		if item.GetBody() != body {
			t.Errorf("isolation: got body %q, want %q", item.GetBody(), body)
		}
	case <-time.After(3 * time.Second):
		t.Error("timed out waiting for streamed comment")
	}
}
