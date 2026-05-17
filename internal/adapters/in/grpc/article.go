package grpc

import (
	"context"
	"errors"
	"fmt"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type articleService interface {
	CreateArticle(ctx context.Context, authorID int, a *domain.CreateArticle) (*domain.Article, error)
	GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error)
	UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error)
	FavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error)
	UnfavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error)
	DeleteArticle(ctx context.Context, callerID int, slug string) error
	ListArticles(ctx context.Context, filter domain.ListArticlesFilter, viewerID int) (*domain.ArticleList, error)
	FeedArticles(ctx context.Context, filter domain.ArticleFeedFilter, viewerID int) (*domain.ArticleList, error)
}

type ArticleServer struct {
	pb.UnimplementedArticleServiceServer
	articleService articleService
}

func NewArticleServer(service articleService) *ArticleServer {
	return &ArticleServer{articleService: service}
}

func articleAuthorToProto(p domain.Profile) *pb.ArticleAuthor {
	return &pb.ArticleAuthor{
		Username:  p.Username,
		Bio:       p.Bio,
		Image:     p.Image,
		Following: p.Following,
	}
}

func articleToProto(a *domain.Article) *pb.ArticleResponse {
	return &pb.ArticleResponse{
		Article: &pb.ArticleResponseInner{
			Slug:           a.Slug,
			Title:          a.Title,
			Description:    a.Description,
			Body:           a.Body,
			TagList:        a.TagList,
			CreatedAt:      timestamppb.New(a.CreatedAt),
			UpdatedAt:      timestamppb.New(a.UpdatedAt),
			Favorited:      a.Favorited,
			FavoritesCount: int32(a.FavoritesCount),
			Author:         articleAuthorToProto(a.Author),
		},
	}
}

func articleListItemToProto(a *domain.Article) *pb.ArticleListItem {
	return &pb.ArticleListItem{
		Slug:           a.Slug,
		Title:          a.Title,
		Description:    a.Description,
		TagList:        a.TagList,
		CreatedAt:      timestamppb.New(a.CreatedAt),
		UpdatedAt:      timestamppb.New(a.UpdatedAt),
		Favorited:      a.Favorited,
		FavoritesCount: int32(a.FavoritesCount),
		Author:         articleAuthorToProto(a.Author),
	}
}

func articleErr(err error) error {
	var notFoundErr *domain.ArticleNotFoundError
	var forbiddenErr *domain.ForbiddenError
	if errors.As(err, &notFoundErr) {
		return status.Error(codes.NotFound, "article not found")
	} else if errors.As(err, &forbiddenErr) {
		return status.Error(codes.PermissionDenied, "forbidden")
	}
	return status.Error(codes.Internal, err.Error())
}

func articlesResponse(list *domain.ArticleList) *pb.ArticlesResponse {
	items := make([]*pb.ArticleListItem, 0, len(list.Articles))
	for _, a := range list.Articles {
		items = append(items, articleListItemToProto(a))
	}
	return &pb.ArticlesResponse{
		Articles:      items,
		ArticlesCount: int32(list.TotalCount),
	}
}

func (s *ArticleServer) CreateArticle(ctx context.Context, in *pb.CreateArticleRequest) (*pb.ArticleResponse, error) {
	authorID := ctx.Value(UserIDKey).(int)

	d := &domain.CreateArticle{
		Title:       in.GetArticle().GetTitle(),
		Description: in.GetArticle().GetDescription(),
		Body:        in.GetArticle().GetBody(),
		TagList:     in.GetArticle().GetTagList(),
	}

	article, err := s.articleService.CreateArticle(ctx, authorID, d)
	if err != nil {
		var validationErr *domain.ValidationError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		} else if errors.As(err, &dupErr) {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("%s %s", dupErr.Field, dupErr.Msg))
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return articleToProto(article), nil
}

func (s *ArticleServer) GetArticleBySlug(ctx context.Context, in *pb.GetArticleBySlugRequest) (*pb.ArticleResponse, error) {
	viewerID, _ := ctx.Value(UserIDKey).(int)

	article, err := s.articleService.GetArticleBySlug(ctx, in.GetSlug(), viewerID)
	if err != nil {
		return nil, articleErr(err)
	}
	return articleToProto(article), nil
}

func (s *ArticleServer) UpdateArticle(ctx context.Context, in *pb.UpdateArticleRequest) (*pb.ArticleResponse, error) {
	callerID := ctx.Value(UserIDKey).(int)

	inner := in.GetArticle()
	u := &domain.UpdateArticle{
		Title:       inner.Title,
		Description: inner.Description,
		Body:        inner.Body,
	}
	if tags := inner.GetTagList(); len(tags) > 0 {
		u.TagList = &tags
	}

	article, err := s.articleService.UpdateArticle(ctx, callerID, in.GetSlug(), u)
	if err != nil {
		var validationErr *domain.ValidationError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, articleErr(err)
	}
	return articleToProto(article), nil
}

func (s *ArticleServer) FavoriteArticle(ctx context.Context, in *pb.FavoriteArticleRequest) (*pb.ArticleResponse, error) {
	userID := ctx.Value(UserIDKey).(int)

	article, err := s.articleService.FavoriteArticle(ctx, userID, in.GetSlug())
	if err != nil {
		return nil, articleErr(err)
	}
	return articleToProto(article), nil
}

func (s *ArticleServer) UnfavoriteArticle(ctx context.Context, in *pb.UnfavoriteArticleRequest) (*pb.ArticleResponse, error) {
	userID := ctx.Value(UserIDKey).(int)

	article, err := s.articleService.UnfavoriteArticle(ctx, userID, in.GetSlug())
	if err != nil {
		return nil, articleErr(err)
	}
	return articleToProto(article), nil
}

func (s *ArticleServer) DeleteArticle(ctx context.Context, in *pb.DeleteArticleRequest) (*emptypb.Empty, error) {
	callerID := ctx.Value(UserIDKey).(int)

	if err := s.articleService.DeleteArticle(ctx, callerID, in.GetSlug()); err != nil {
		return nil, articleErr(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *ArticleServer) ListArticles(ctx context.Context, in *pb.ListArticlesRequest) (*pb.ArticlesResponse, error) {
	viewerID, _ := ctx.Value(UserIDKey).(int)

	filter := domain.ListArticlesFilter{
		Limit:     20,
		Tag:       in.Tag,
		Author:    in.Author,
		Favorited: in.Favorited,
	}
	if in.GetLimit() > 0 {
		filter.Limit = int(in.GetLimit())
	}
	if in.GetOffset() > 0 {
		filter.Offset = int(in.GetOffset())
	}

	list, err := s.articleService.ListArticles(ctx, filter, viewerID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return articlesResponse(list), nil
}

func (s *ArticleServer) FeedArticles(ctx context.Context, in *pb.FeedArticlesRequest) (*pb.ArticlesResponse, error) {
	userID := ctx.Value(UserIDKey).(int)

	filter := domain.ArticleFeedFilter{Limit: 20}
	if in.GetLimit() > 0 {
		filter.Limit = int(in.GetLimit())
	}
	if in.GetOffset() > 0 {
		filter.Offset = int(in.GetOffset())
	}

	list, err := s.articleService.FeedArticles(ctx, filter, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return articlesResponse(list), nil
}
