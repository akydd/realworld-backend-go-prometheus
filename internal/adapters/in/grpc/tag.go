package grpc

import (
	"context"
	"realworld-backend-go/api/proto/gen/pb"

	"google.golang.org/protobuf/types/known/emptypb"
)

type tagService interface {
	GetTags(ctx context.Context) ([]string, error)
}

type TagServer struct {
	pb.UnimplementedTagServiceServer
	tagService tagService
}

func NewTagServer(service tagService) *TagServer {
	return &TagServer{tagService: service}
}

func (s *TagServer) GetTags(ctx context.Context, in *emptypb.Empty) (*pb.TagsResponse, error) {
	tags, err := s.tagService.GetTags(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.TagsResponse{Tags: tags}, nil
}
