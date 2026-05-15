package grpc

import (
	"context"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type commentService interface {
	CreateComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error)
	GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error)
	DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
}

type CommentServer struct {
	pb.UnimplementedCommentServiceServer
	commentService commentService
}

func NewCommentServer(service commentService) *CommentServer {
	return &CommentServer{commentService: service}
}

func commentToProto(c *domain.Comment) *pb.CommentResponseInner {
	return &pb.CommentResponseInner{
		Id:        int64(c.ID),
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
		Body:      c.Body,
		Author: &pb.CommentAuthor{
			Username:  c.Author.Username,
			Bio:       c.Author.Bio,
			Image:     c.Author.Image,
			Following: c.Author.Following,
		},
	}
}

func (s *CommentServer) CreateComment(ctx context.Context, in *pb.CreateCommentRequest) (*pb.CommentResponse, error) {
	return nil, nil
}

func (s *CommentServer) GetComments(ctx context.Context, in *pb.GetCommentsRequest) (*pb.CommentsResponse, error) {
	comments, err := s.commentService.GetComments(ctx, in.GetSlug(), 0)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.CommentResponseInner, 0, len(comments))
	for _, c := range comments {
		items = append(items, commentToProto(c))
	}
	return &pb.CommentsResponse{Comments: items}, nil
}

func (s *CommentServer) DeleteComment(ctx context.Context, in *pb.DeleteCommentRequest) (*emptypb.Empty, error) {
	return nil, nil
}
