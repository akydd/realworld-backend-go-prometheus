package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"realworld-backend-go/internal/domain"
)

type userService interface {
	RegisterUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error)
	LoginUser(ctx context.Context, u *domain.LoginUser) (*domain.User, error)
	GetUser(ctx context.Context, userID int) (*domain.User, error)
	UpdateUser(ctx context.Context, userID int, u *domain.UpdateUser) (*domain.User, error)
}

type profileService interface {
	GetProfile(ctx context.Context, profileUsername string, viewerID int) (*domain.Profile, error)
	FollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
	UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
}

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

type tagService interface {
	GetTags(ctx context.Context) ([]string, error)
}

type commentService interface {
	CreateComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error)
	GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error)
	DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
}

// Handler is the HTTP adapter that translates incoming requests into domain service calls
// and writes the corresponding JSON responses.
type Handler struct {
	service        userService
	profileService profileService
	articleService articleService
	tagService     tagService
	commentService commentService
}

// NewHandler creates a Handler wired to the provided domain service implementations.
func NewHandler(s userService, ps profileService, as articleService, ts tagService, cs commentService) *Handler {
	return &Handler{
		service:        s,
		profileService: ps,
		articleService: as,
		tagService:     ts,
		commentService: cs,
	}
}

// NullableString distinguishes a JSON field being absent (Present=false)
// from being explicitly set to null or "" (Present=true, Value=nil/"").
type NullableString struct {
	Value   *string
	Present bool
}

// UnmarshalJSON implements json.Unmarshaler for NullableString.
func (n *NullableString) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.Value = &s
	return nil
}

// NullableStringSlice distinguishes absent (Present=false), null (IsNull=true), and present slice.
type NullableStringSlice struct {
	Value   []string
	Present bool
	IsNull  bool
}

// UnmarshalJSON implements json.Unmarshaler for NullableStringSlice.
func (n *NullableStringSlice) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(data) == "null" {
		n.IsNull = true
		return nil
	}
	return json.Unmarshal(data, &n.Value)
}

// ErrorResponse is the standard JSON error envelope returned by the API.
type ErrorResponse struct {
	Errors map[string][]string `json:"errors"`
}

func createErrResponse(k string, v []string) []byte {
	errResp := ErrorResponse{
		Errors: map[string][]string{
			k: v,
		},
	}
	jsonErrResp, _ := json.Marshal(errResp)
	return jsonErrResp
}

// HealthCheck handles GET /api/healthcheck and returns 200 OK if the server is running.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
