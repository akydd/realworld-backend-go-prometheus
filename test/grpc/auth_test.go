//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestAuth(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	ctx := context.Background()

	uid := genUID()
	username := "auth_" + uid
	email := "auth_" + uid + "@test.com"
	username2 := "auth_" + uid + "_upd"
	email2 := "auth_" + uid + "_upd@test.com"

	// ── Register ──────────────────────────────────────────────────────────────

	regResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: username,
			Email:    email,
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	u := regResp.GetUser()
	if got := u.GetUsername(); got != username {
		t.Errorf("register username: got %q, want %q", got, username)
	}
	if got := u.GetEmail(); got != email {
		t.Errorf("register email: got %q, want %q", got, email)
	}
	if u.Bio != nil {
		t.Errorf("register bio: got %q, want nil", *u.Bio)
	}
	if u.Image != nil {
		t.Errorf("register image: got %q, want nil", *u.Image)
	}
	if u.GetToken() == "" {
		t.Fatal("register token: got empty, want non-empty")
	}

	// ── Login ─────────────────────────────────────────────────────────────────

	loginResp, err := users.LoginUser(ctx, &pb.LoginUserRequest{
		User: &pb.LoginUserRequestInner{
			Email:    email,
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("LoginUser: %v", err)
	}
	token := loginResp.GetUser().GetToken()
	if token == "" {
		t.Fatal("login token: got empty, want non-empty")
	}
	if got := loginResp.GetUser().GetUsername(); got != username {
		t.Errorf("login username: got %q, want %q", got, username)
	}

	authedCtx := withToken(ctx, token)

	// ── Get current user ──────────────────────────────────────────────────────

	getResp, err := users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	u = getResp.GetUser()
	if got := u.GetUsername(); got != username {
		t.Errorf("get user username: got %q, want %q", got, username)
	}
	if u.Bio != nil {
		t.Errorf("get user bio: got %q, want nil", *u.Bio)
	}
	if u.Image != nil {
		t.Errorf("get user image: got %q, want nil", *u.Image)
	}

	// ── Bio: set → verify → empty string → null ───────────────────────────────

	resp, err := users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Bio: nullableStr("Updated bio")},
	})
	if err != nil {
		t.Fatalf("UpdateUser set bio: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if bio := resp.GetUser().Bio; bio == nil || *bio != "Updated bio" {
		t.Errorf("update bio: got %v, want \"Updated bio\"", bio)
	}
	if resp.GetUser().Image != nil {
		t.Errorf("update bio: image should still be nil")
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser after set bio: %v", err)
	}
	if bio := getResp.GetUser().Bio; bio == nil || *bio != "Updated bio" {
		t.Errorf("verify bio persisted: got %v, want \"Updated bio\"", bio)
	}

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Bio: nullableStr("")},
	})
	if err != nil {
		t.Fatalf("UpdateUser bio empty string: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if resp.GetUser().Bio != nil {
		t.Errorf("bio empty string→null: got %q, want nil", *resp.GetUser().Bio)
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser after bio empty string: %v", err)
	}
	if getResp.GetUser().Bio != nil {
		t.Errorf("verify bio empty string→null: got %q, want nil", *getResp.GetUser().Bio)
	}

	// ── Bio: restore → clear via empty wrapper ─────────────────────────────────

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Bio: nullableStr("Temporary bio")},
	})
	if err != nil {
		t.Fatalf("UpdateUser restore bio: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if bio := resp.GetUser().Bio; bio == nil || *bio != "Temporary bio" {
		t.Errorf("restore bio: got %v, want \"Temporary bio\"", bio)
	}

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Bio: clearNullable()},
	})
	if err != nil {
		t.Fatalf("UpdateUser clear bio: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if resp.GetUser().Bio != nil {
		t.Errorf("bio clear→null: got %q, want nil", *resp.GetUser().Bio)
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser after bio clear: %v", err)
	}
	if getResp.GetUser().Bio != nil {
		t.Errorf("verify bio clear→null: got %q, want nil", *getResp.GetUser().Bio)
	}

	// ── Restore bio to "Updated bio" before image and username/email tests ─────

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Bio: nullableStr("Updated bio")},
	})
	if err != nil {
		t.Fatalf("UpdateUser restore bio for later: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if bio := resp.GetUser().Bio; bio == nil || *bio != "Updated bio" {
		t.Errorf("restore bio for later: got %v, want \"Updated bio\"", bio)
	}

	// ── Image: set → verify → empty string → null ─────────────────────────────

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Image: nullableStr("https://example.com/image.png")},
	})
	if err != nil {
		t.Fatalf("UpdateUser set image: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if img := resp.GetUser().Image; img == nil || *img != "https://example.com/image.png" {
		t.Errorf("update image: got %v, want \"https://example.com/image.png\"", img)
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser after set image: %v", err)
	}
	if img := getResp.GetUser().Image; img == nil || *img != "https://example.com/image.png" {
		t.Errorf("verify image persisted: got %v, want \"https://example.com/image.png\"", img)
	}

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Image: nullableStr("")},
	})
	if err != nil {
		t.Fatalf("UpdateUser image empty string: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if resp.GetUser().Image != nil {
		t.Errorf("image empty string→null: got %q, want nil", *resp.GetUser().Image)
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser after image empty string: %v", err)
	}
	if getResp.GetUser().Image != nil {
		t.Errorf("verify image empty string→null: got %q, want nil", *getResp.GetUser().Image)
	}

	// ── Image: set → clear via empty wrapper ──────────────────────────────────

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Image: nullableStr("https://example.com/temp.jpg")},
	})
	if err != nil {
		t.Fatalf("UpdateUser set temp image: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if img := resp.GetUser().Image; img == nil || *img != "https://example.com/temp.jpg" {
		t.Errorf("set temp image: got %v, want \"https://example.com/temp.jpg\"", img)
	}

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{Image: clearNullable()},
	})
	if err != nil {
		t.Fatalf("UpdateUser clear image: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	if resp.GetUser().Image != nil {
		t.Errorf("image clear→null: got %q, want nil", *resp.GetUser().Image)
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser after image clear: %v", err)
	}
	if getResp.GetUser().Image != nil {
		t.Errorf("verify image clear→null: got %q, want nil", *getResp.GetUser().Image)
	}

	// ── Update username and email ──────────────────────────────────────────────

	resp, err = users.UpdateUser(authedCtx, &pb.UpdateUserRequest{
		User: &pb.UpdateUserRequestInner{
			Username: strPtr(username2),
			Email:    strPtr(email2),
		},
	})
	if err != nil {
		t.Fatalf("UpdateUser username+email: %v", err)
	}
	token = resp.GetUser().GetToken()
	authedCtx = withToken(ctx, token)
	u = resp.GetUser()
	if got := u.GetUsername(); got != username2 {
		t.Errorf("update username: got %q, want %q", got, username2)
	}
	if got := u.GetEmail(); got != email2 {
		t.Errorf("update email: got %q, want %q", got, email2)
	}
	if bio := u.Bio; bio == nil || *bio != "Updated bio" {
		t.Errorf("update: bio unchanged: got %v, want \"Updated bio\"", bio)
	}
	if u.Image != nil {
		t.Errorf("update: image still nil: got %q, want nil", *u.Image)
	}

	getResp, err = users.GetUser(authedCtx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetUser final verify: %v", err)
	}
	u = getResp.GetUser()
	if got := u.GetUsername(); got != username2 {
		t.Errorf("verify username: got %q, want %q", got, username2)
	}
	if got := u.GetEmail(); got != email2 {
		t.Errorf("verify email: got %q, want %q", got, email2)
	}
	if bio := u.Bio; bio == nil || *bio != "Updated bio" {
		t.Errorf("verify bio unchanged: got %v, want \"Updated bio\"", bio)
	}
	if u.Image != nil {
		t.Errorf("verify image nil: got %q, want nil", *u.Image)
	}
}
