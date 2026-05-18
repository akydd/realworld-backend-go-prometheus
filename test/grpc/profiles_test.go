//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"realworld-backend-go/api/proto/gen/pb"
)

func TestProfiles(t *testing.T) {
	conn := dial(t)
	users := pb.NewUserServiceClient(conn)
	profiles := pb.NewProfileServiceClient(conn)
	ctx := context.Background()

	uid := genUID()

	mainResp, err := users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "main_" + uid,
			Email:    "main_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser main: %v", err)
	}
	mainToken := mainResp.GetUser().GetToken()
	mainCtx := withToken(ctx, mainToken)

	_, err = users.RegisterUser(ctx, &pb.RegisterUserRequest{
		User: &pb.RegisterUserRequestInner{
			Username: "celeb_" + uid,
			Email:    "celeb_" + uid + "@test.com",
			Password: "password123",
		},
	})
	if err != nil {
		t.Fatalf("RegisterUser celeb: %v", err)
	}

	// ── Get profile without auth ───────────────────────────────────────────────

	profResp, err := profiles.GetProfile(ctx, &pb.GetProfileRequest{Username: "celeb_" + uid})
	if err != nil {
		t.Fatalf("GetProfile no-auth: %v", err)
	}
	if got := profResp.GetProfile().GetUsername(); got != "celeb_"+uid {
		t.Errorf("get profile username: got %q, want %q", got, "celeb_"+uid)
	}
	if profResp.GetProfile().GetFollowing() {
		t.Error("get profile following: got true, want false")
	}

	// ── Get profile with auth ──────────────────────────────────────────────────

	profResp, err = profiles.GetProfile(mainCtx, &pb.GetProfileRequest{Username: "celeb_" + uid})
	if err != nil {
		t.Fatalf("GetProfile authed: %v", err)
	}
	if got := profResp.GetProfile().GetUsername(); got != "celeb_"+uid {
		t.Errorf("get profile auth username: got %q, want %q", got, "celeb_"+uid)
	}
	if profResp.GetProfile().GetFollowing() {
		t.Error("get profile auth following: got true, want false")
	}

	// ── Follow ─────────────────────────────────────────────────────────────────

	profResp, err = profiles.FollowUser(mainCtx, &pb.FollowUserRequest{Username: "celeb_" + uid})
	if err != nil {
		t.Fatalf("FollowUser: %v", err)
	}
	if got := profResp.GetProfile().GetUsername(); got != "celeb_"+uid {
		t.Errorf("follow username: got %q, want %q", got, "celeb_"+uid)
	}
	if !profResp.GetProfile().GetFollowing() {
		t.Error("follow following: got false, want true")
	}

	// ── Unfollow ───────────────────────────────────────────────────────────────

	profResp, err = profiles.UnfollowUser(mainCtx, &pb.UnfollowUserRequest{Username: "celeb_" + uid})
	if err != nil {
		t.Fatalf("UnfollowUser: %v", err)
	}
	if got := profResp.GetProfile().GetUsername(); got != "celeb_"+uid {
		t.Errorf("unfollow username: got %q, want %q", got, "celeb_"+uid)
	}
	if profResp.GetProfile().GetFollowing() {
		t.Error("unfollow following: got true, want false")
	}

	// ── Verify unfollow persisted ──────────────────────────────────────────────

	profResp, err = profiles.GetProfile(mainCtx, &pb.GetProfileRequest{Username: "celeb_" + uid})
	if err != nil {
		t.Fatalf("GetProfile verify: %v", err)
	}
	if profResp.GetProfile().GetFollowing() {
		t.Error("verify unfollow persisted: got true, want false")
	}
}
