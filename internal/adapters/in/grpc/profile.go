package grpc

import (
	"context"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"
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

func (s *ProfileServer) GetProfile(ctx context.Context, in *pb.GetProfileRequest) (*pb.ProfileResponse, error) {
	viewerID, _ := ctx.Value(UserIDKey).(int)

	profile, err := s.profileService.GetProfile(ctx, in.GetUsername(), viewerID)
	if err != nil {
		return nil, domainErr(err)
	}
	return profileToProto(profile), nil
}

func (s *ProfileServer) FollowUser(ctx context.Context, in *pb.FollowUserRequest) (*pb.ProfileResponse, error) {
	followerID := ctx.Value(UserIDKey).(int)

	profile, err := s.profileService.FollowUser(ctx, followerID, in.GetUsername())
	if err != nil {
		return nil, domainErr(err)
	}
	return profileToProto(profile), nil
}

func (s *ProfileServer) UnfollowUser(ctx context.Context, in *pb.UnfollowUserRequest) (*pb.ProfileResponse, error) {
	followerID := ctx.Value(UserIDKey).(int)

	profile, err := s.profileService.UnfollowUser(ctx, followerID, in.GetUsername())
	if err != nil {
		return nil, domainErr(err)
	}
	return profileToProto(profile), nil
}
