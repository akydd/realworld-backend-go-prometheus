package grpc

import (
	"context"
	"realworld-backend-go/api/proto/gen/pb"
	"realworld-backend-go/internal/domain"

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

func (s *ArticleServer) CreateArticle(ctx context.Context, in *pb.CreateArticleRequest) (*pb.ArticleResponse, error) {
	return nil, nil
}

func (s *ArticleServer) GetArticleBySlug(ctx context.Context, in *pb.GetArticleBySlugRequest) (*pb.ArticleResponse, error) {
	article, err := s.articleService.GetArticleBySlug(ctx, in.GetSlug(), 0)
	if err != nil {
		return nil, err
	}
	return articleToProto(article), nil
}

func (s *ArticleServer) UpdateArticle(ctx context.Context, in *pb.UpdateArticleRequest) (*pb.ArticleResponse, error) {
	return nil, nil
}

func (s *ArticleServer) FavoriteArticle(ctx context.Context, in *pb.FavoriteArticleRequest) (*pb.ArticleResponse, error) {
	return nil, nil
}

func (s *ArticleServer) UnfavoriteArticle(ctx context.Context, in *pb.UnfavoriteArticleRequest) (*pb.ArticleResponse, error) {
	return nil, nil
}

func (s *ArticleServer) DeleteArticle(ctx context.Context, in *pb.DeleteArticleRequest) (*emptypb.Empty, error) {
	return nil, nil
}

func (s *ArticleServer) ListArticles(ctx context.Context, in *pb.ListArticlesRequest) (*pb.ArticlesResponse, error) {
	filter := domain.ListArticlesFilter{
		Limit:  20,
		Offset: 0,
	}
	filter.Tag = in.Tag
	filter.Author = in.Author
	filter.Favorited = in.Favorited
	if in.GetLimit() > 0 {
		filter.Limit = int(in.GetLimit())
	}
	if in.GetOffset() > 0 {
		filter.Offset = int(in.GetOffset())
	}

	list, err := s.articleService.ListArticles(ctx, filter, 0)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.ArticleListItem, 0, len(list.Articles))
	for _, a := range list.Articles {
		items = append(items, &pb.ArticleListItem{
			Slug:           a.Slug,
			Title:          a.Title,
			Description:    a.Description,
			TagList:        a.TagList,
			CreatedAt:      timestamppb.New(a.CreatedAt),
			UpdatedAt:      timestamppb.New(a.UpdatedAt),
			Favorited:      a.Favorited,
			FavoritesCount: int32(a.FavoritesCount),
			Author:         articleAuthorToProto(a.Author),
		})
	}
	return &pb.ArticlesResponse{
		Articles:      items,
		ArticlesCount: int32(list.TotalCount),
	}, nil
}

func (s *ArticleServer) FeedArticles(ctx context.Context, in *pb.FeedArticlesRequest) (*pb.ArticlesResponse, error) {
	return nil, nil
}
