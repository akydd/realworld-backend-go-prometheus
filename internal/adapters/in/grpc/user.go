package grpc

import (
	"context"
	"fmt"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type userService interface {
	RegisterUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error)
	LoginUser(ctx context.Context, u *domain.LoginUser) (*domain.User, error)
	GetUser(ctx context.Context, userID int) (*domain.User, error)
	UpdateUser(ctx context.Context, userID int, u *domain.UpdateUser) (*domain.User, error)
}
type UserServer struct {
	pb.UnimplementedUserServiceServer
	userService userService
}

func NewUserServer(service userService) *UserServer {
	return &UserServer{
		userService: service,
	}
}

func (u *UserServer) RegisterUser(ctx context.Context, in *pb.RegisterUserRequest) (*pb.UserResponse, error) {
	d := &domain.RegisterUser{
		Email:    in.GetUser().GetEmail(),
		Username: in.GetUser().GetUsername(),
		Password: in.GetUser().GetPassword(),
	}

	user, err := u.userService.RegisterUser(ctx, d)
	if err != nil {
		var validationErr *domain.ValidationError
		var dupErr *domain.DuplicateError
		if errors.As(err, &dupErr) {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("%s %s", dupErr.Field, dupErr.Msg))
		} else if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%s", err.Error()))
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &pb.UserResponse{
		User: &pb.UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}

	return resp, nil
}

func (u *UserServer) LoginUser(ctx context.Context, in *pb.LoginUserRequest) (*pb.UserResponse, error) {
	d := &domain.LoginUser{
		Email:    in.GetUser().GetEmail(),
		Password: in.GetUser().GetPassword(),
	}

	user, err := u.userService.LoginUser(ctx, d)
	if err != nil {
		var validationErr *domain.ValidationError
		var credErr *domain.CredentialsError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%s", err.Error()))
		} else if errors.As(err, &credErr) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &pb.UserResponse{
		User: &pb.UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	return resp, nil
}

func (u *UserServer) GetUser(ctx context.Context, in *emptypb.Empty) (*pb.UserResponse, error) {
	return nil, nil
}

func (u *UserServer) UpdateUser(ctx context.Context, in *pb.UpdateUserRequest) (*pb.UserResponse, error) {
	return nil, nil
}
