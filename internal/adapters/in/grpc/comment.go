package grpc

import (
	"context"
	"fmt"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type commentService interface {
	CreateComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error)
	GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error)
	DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
	CommentSubscribe(ctx context.Context, slug string, viewerID int) (<-chan domain.Comment, error)
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
		return nil, domainErr(err)
	}

	return &pb.CommentResponse{Comment: commentToProto(comment)}, nil
}

func (s *CommentServer) GetComments(ctx context.Context, in *pb.GetCommentsRequest) (*pb.CommentsResponse, error) {
	viewerID, _ := ctx.Value(UserIDKey).(int)

	comments, err := s.commentService.GetComments(ctx, in.GetSlug(), viewerID)
	if err != nil {
		return nil, domainErr(err)
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
		return nil, domainErr(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *CommentServer) LiveCommentFeed(in *pb.LiveCommentFeedRequest, stream grpc.ServerStreamingServer[pb.CommentResponseInner]) error {
	userID, _ := stream.Context().Value(UserIDKey).(int)
	sub, err := s.commentService.CommentSubscribe(stream.Context(), in.GetSlug(), userID)
	if err != nil {
		return domainErr(err)
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case c, ok := <-sub:
			if !ok {
				return nil
			}
			err := stream.Send(&pb.CommentResponseInner{
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
			})
			if err != nil {
				return fmt.Errorf("could not stream comment id %d: %w", c.ID, err)
			}
		}
	}
}
