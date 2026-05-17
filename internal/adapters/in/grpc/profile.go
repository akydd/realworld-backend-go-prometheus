package grpc

import (
	"context"
	"errors"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type profileService interface {
	GetProfile(ctx context.Context, profileUsername string, viewerID int) (*domain.Profile, error)
	FollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
	UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
}

type ProfileServer struct {
	pb.UnimplementedProfileServiceServer
	profileService profileService
}

func NewProfileServer(service profileService) *ProfileServer {
	return &ProfileServer{profileService: service}
}

func profileToProto(p *domain.Profile) *pb.ProfileResponse {
	return &pb.ProfileResponse{
		Profile: &pb.ProfileResponseInner{
			Username:  p.Username,
			Bio:       p.Bio,
			Image:     p.Image,
			Following: p.Following,
		},
	}
}

func profileErr(err error) error {
	var notFoundErr *domain.ProfileNotFoundError
	if errors.As(err, &notFoundErr) {
		return status.Error(codes.NotFound, "profile not found")
	}
	return status.Error(codes.Internal, err.Error())
}

func (s *ProfileServer) GetProfile(ctx context.Context, in *pb.GetProfileRequest) (*pb.ProfileResponse, error) {
	viewerID, _ := ctx.Value(UserIDKey).(int)

	profile, err := s.profileService.GetProfile(ctx, in.GetUsername(), viewerID)
	if err != nil {
		return nil, profileErr(err)
	}
	return profileToProto(profile), nil
}

func (s *ProfileServer) FollowUser(ctx context.Context, in *pb.FollowUserRequest) (*pb.ProfileResponse, error) {
	followerID := ctx.Value(UserIDKey).(int)

	profile, err := s.profileService.FollowUser(ctx, followerID, in.GetUsername())
	if err != nil {
		return nil, profileErr(err)
	}
	return profileToProto(profile), nil
}

func (s *ProfileServer) UnfollowUser(ctx context.Context, in *pb.UnfollowUserRequest) (*pb.ProfileResponse, error) {
	followerID := ctx.Value(UserIDKey).(int)

	profile, err := s.profileService.UnfollowUser(ctx, followerID, in.GetUsername())
	if err != nil {
		return nil, profileErr(err)
	}
	return profileToProto(profile), nil
}
