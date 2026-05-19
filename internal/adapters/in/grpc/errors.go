package grpc

import (
	"errors"
	"strings"

	"realworld-backend-go/internal/domain"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// domainErr maps all domain error types to structured gRPC status errors with
// attached errdetails so callers can inspect both code and structured fields.
func domainErr(err error) error {
	var validationErr *domain.ValidationError
	var dupErr *domain.DuplicateError
	var credErr *domain.CredentialsError
	var profileNotFound *domain.ProfileNotFoundError
	var articleNotFound *domain.ArticleNotFoundError
	var commentNotFound *domain.CommentNotFoundError
	var forbiddenErr *domain.ForbiddenError

	switch {
	case errors.As(err, &validationErr):
		st, detailErr := status.New(codes.InvalidArgument, "validation failed").
			WithDetails(&errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{{
					Field:       validationErr.Field,
					Description: strings.Join(validationErr.Errors, ", "),
				}},
			})
		if detailErr != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		return st.Err()

	case errors.As(err, &dupErr):
		st, detailErr := status.New(codes.AlreadyExists, "already exists").
			WithDetails(&errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{{
					Field:       dupErr.Field,
					Description: dupErr.Msg,
				}},
			})
		if detailErr != nil {
			return status.Error(codes.AlreadyExists, err.Error())
		}
		return st.Err()

	case errors.As(err, &credErr):
		st, detailErr := status.New(codes.Unauthenticated, "invalid credentials").
			WithDetails(&errdetails.ErrorInfo{
				Reason: "INVALID_CREDENTIALS",
				Domain: "realworld",
			})
		if detailErr != nil {
			return status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return st.Err()

	case errors.As(err, &profileNotFound):
		st, detailErr := status.New(codes.NotFound, "profile not found").
			WithDetails(&errdetails.ResourceInfo{
				ResourceType: "profile",
				Description:  "profile not found",
			})
		if detailErr != nil {
			return status.Error(codes.NotFound, "profile not found")
		}
		return st.Err()

	case errors.As(err, &articleNotFound):
		st, detailErr := status.New(codes.NotFound, "article not found").
			WithDetails(&errdetails.ResourceInfo{
				ResourceType: "article",
				Description:  "article not found",
			})
		if detailErr != nil {
			return status.Error(codes.NotFound, "article not found")
		}
		return st.Err()

	case errors.As(err, &commentNotFound):
		st, detailErr := status.New(codes.NotFound, "comment not found").
			WithDetails(&errdetails.ResourceInfo{
				ResourceType: "comment",
				Description:  "comment not found",
			})
		if detailErr != nil {
			return status.Error(codes.NotFound, "comment not found")
		}
		return st.Err()

	case errors.As(err, &forbiddenErr):
		st, detailErr := status.New(codes.PermissionDenied, "forbidden").
			WithDetails(&errdetails.ErrorInfo{
				Reason: "PERMISSION_DENIED",
				Domain: "realworld",
			})
		if detailErr != nil {
			return status.Error(codes.PermissionDenied, "forbidden")
		}
		return st.Err()

	default:
		return status.Error(codes.Internal, err.Error())
	}
}
