package grpc

import (
	"context"
	"errors"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	authorID := ctx.Value(UserIDKey).(int)

	comment, err := s.commentService.CreateComment(ctx, authorID, in.GetSlug(), &domain.CreateComment{Body: in.GetComment().GetBody()})
	if err != nil {
		var validationErr *domain.ValidationError
		var notFoundErr *domain.ArticleNotFoundError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		} else if errors.As(err, &notFoundErr) {
			return nil, status.Error(codes.NotFound, "article not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CommentResponse{Comment: commentToProto(comment)}, nil
}

func (s *CommentServer) GetComments(ctx context.Context, in *pb.GetCommentsRequest) (*pb.CommentsResponse, error) {
	viewerID, _ := ctx.Value(UserIDKey).(int)

	comments, err := s.commentService.GetComments(ctx, in.GetSlug(), viewerID)
	if err != nil {
		var notFoundErr *domain.ArticleNotFoundError
		if errors.As(err, &notFoundErr) {
			return nil, status.Error(codes.NotFound, "article not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	items := make([]*pb.CommentResponseInner, 0, len(comments))
	for _, c := range comments {
		items = append(items, commentToProto(c))
	}
	return &pb.CommentsResponse{Comments: items}, nil
}

func (s *CommentServer) DeleteComment(ctx context.Context, in *pb.DeleteCommentRequest) (*emptypb.Empty, error) {
	callerID := ctx.Value(UserIDKey).(int)

	err := s.commentService.DeleteComment(ctx, callerID, in.GetSlug(), int(in.GetId()))
	if err != nil {
		var notFoundArticle *domain.ArticleNotFoundError
		var notFoundComment *domain.CommentNotFoundError
		var forbiddenErr *domain.ForbiddenError
		if errors.As(err, &notFoundArticle) {
			return nil, status.Error(codes.NotFound, "article not found")
		} else if errors.As(err, &notFoundComment) {
			return nil, status.Error(codes.NotFound, "comment not found")
		} else if errors.As(err, &forbiddenErr) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}
