package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"realworld-backend-go/internal/domain"
	"strconv"
	"time"

	"github.com/gorilla/mux"
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

// LoginUserInner holds the credentials fields within a login request body.
type LoginUserInner struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginUserRequest is the top-level JSON wrapper for POST /api/users/login.
type LoginUserRequest struct {
	User LoginUserInner `json:"user"`
}

// RegisterUserInner holds the registration fields within a registration request body.
type RegisterUserInner struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterUserRequest is the top-level JSON wrapper for POST /api/users.
type RegisterUserRequest struct {
	User RegisterUserInner `json:"user"`
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

// UpdateUserInner holds the optional fields within a user update request body.
type UpdateUserInner struct {
	Email    *string        `json:"email"`
	Bio      NullableString `json:"bio"`
	Image    NullableString `json:"image"`
	Username *string        `json:"username"`
	Password *string        `json:"password"`
}

// UpdateUserRequest is the top-level JSON wrapper for PUT /api/user.
type UpdateUserRequest struct {
	User UpdateUserInner `json:"user"`
}

// UserResponseInner holds the user fields returned in API responses.
type UserResponseInner struct {
	Email    string  `json:"email"`
	Token    string  `json:"token"`
	Username string  `json:"username"`
	Bio      *string `json:"bio"`
	Image    *string `json:"image"`
}

// UserResponse is the top-level JSON wrapper for user API responses.
type UserResponse struct {
	User UserResponseInner `json:"user"`
}

// ErrorResponse is the standard JSON error envelope returned by the API.
type ErrorResponse struct {
	Errors map[string][]string `json:"errors"`
}

// RegisterUser handles POST /api/users and creates a new user account.
func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var regUser RegisterUserRequest
	err := json.NewDecoder(r.Body).Decode(&regUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.RegisterUser(regUser.User)

	w.Header().Set("Content-Type", "application/json")

	user, err := h.service.RegisterUser(r.Context(), &d)
	if err != nil {
		var errResp []byte
		var validationErr *domain.ValidationError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			errResp = createErrResponse(validationErr.Field, validationErr.Errors)
			w.WriteHeader(http.StatusUnprocessableEntity)
		} else if errors.As(err, &dupErr) {
			errResp = createErrResponse(dupErr.Field, []string{dupErr.Msg})
			w.WriteHeader(http.StatusConflict)
		} else {
			fmt.Println(err.Error())
			errResp = createErrResponse("unknown_error", []string{err.Error()})
			w.WriteHeader(http.StatusInternalServerError)
		}

		_, _ = w.Write(errResp)
		return
	}

	resp := UserResponse{
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// LoginUser handles POST /api/users/login and authenticates a user.
func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req LoginUserRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.LoginUser(req.User)

	w.Header().Set("Content-Type", "application/json")

	user, err := h.service.LoginUser(r.Context(), &d)
	if err != nil {
		var errResp []byte
		var validationErr *domain.ValidationError
		var credErr *domain.CredentialsError
		if errors.As(err, &validationErr) {
			errResp = createErrResponse(validationErr.Field, validationErr.Errors)
			w.WriteHeader(http.StatusUnprocessableEntity)
		} else if errors.As(err, &credErr) {
			errResp = createErrResponse("credentials", []string{"invalid"})
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			fmt.Println(err.Error())
			errResp = createErrResponse("unknown_error", []string{err.Error()})
			w.WriteHeader(http.StatusInternalServerError)
		}
		_, _ = w.Write(errResp)
		return
	}

	resp := UserResponse{
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetUser handles GET /api/user and returns the currently authenticated user.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)

	user, err := h.service.GetUser(r.Context(), userID)
	if err != nil {
		var credErr *domain.CredentialsError
		if errors.As(err, &credErr) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	resp := UserResponse{
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateUser handles PUT /api/user and updates the currently authenticated user's profile.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.UpdateUser{
		Email:    req.User.Email,
		Username: req.User.Username,
		Password: req.User.Password,
	}
	if req.User.Bio.Present {
		if req.User.Bio.Value != nil && *req.User.Bio.Value != "" {
			d.Bio = &req.User.Bio.Value
		} else {
			d.Bio = new(*string) // pointer to nil *string = set to null
		}
	}
	if req.User.Image.Present {
		if req.User.Image.Value != nil && *req.User.Image.Value != "" {
			d.Image = &req.User.Image.Value
		} else {
			d.Image = new(*string)
		}
	}

	user, err := h.service.UpdateUser(r.Context(), userID, &d)
	if err != nil {
		var validationErr *domain.ValidationError
		var credErr *domain.CredentialsError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &credErr) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
		} else if errors.As(err, &dupErr) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write(createErrResponse(dupErr.Field, []string{dupErr.Msg}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	resp := UserResponse{
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// ProfileResponseInner holds the profile fields returned in API responses.
type ProfileResponseInner struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

// ProfileResponse is the top-level JSON wrapper for profile API responses.
type ProfileResponse struct {
	Profile ProfileResponseInner `json:"profile"`
}

func profileResponse(profile *domain.Profile) ProfileResponse {
	return ProfileResponse{
		Profile: ProfileResponseInner{
			Username:  profile.Username,
			Bio:       profile.Bio,
			Image:     profile.Image,
			Following: profile.Following,
		},
	}
}

func writeArticleErr(w http.ResponseWriter, err error) {
	var notFoundErr *domain.ArticleNotFoundError
	if errors.As(err, &notFoundErr) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(createErrResponse("article", []string{"not found"}))
	} else {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
	}
}

func writeProfileErr(w http.ResponseWriter, err error) {
	var notFoundErr *domain.ProfileNotFoundError
	if errors.As(err, &notFoundErr) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(createErrResponse("profile", []string{"not found"}))
	} else {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
	}
}

// GetProfile handles GET /api/profiles/{username} and returns the requested user profile.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	profileUsername := mux.Vars(r)["username"]
	viewerID, _ := r.Context().Value(userIDKey).(int)

	w.Header().Set("Content-Type", "application/json")

	profile, err := h.profileService.GetProfile(r.Context(), profileUsername, viewerID)
	if err != nil {
		writeProfileErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileResponse(profile))
}

// FollowUser handles POST /api/profiles/{username}/follow and subscribes the caller to the target user.
func (h *Handler) FollowUser(w http.ResponseWriter, r *http.Request) {
	followerID := r.Context().Value(userIDKey).(int)
	followeeUsername := mux.Vars(r)["username"]

	w.Header().Set("Content-Type", "application/json")

	profile, err := h.profileService.FollowUser(r.Context(), followerID, followeeUsername)
	if err != nil {
		writeProfileErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileResponse(profile))
}

// UnfollowUser handles DELETE /api/profiles/{username}/follow and removes the caller's subscription.
func (h *Handler) UnfollowUser(w http.ResponseWriter, r *http.Request) {
	followerID := r.Context().Value(userIDKey).(int)
	followeeUsername := mux.Vars(r)["username"]

	w.Header().Set("Content-Type", "application/json")

	profile, err := h.profileService.UnfollowUser(r.Context(), followerID, followeeUsername)
	if err != nil {
		writeProfileErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileResponse(profile))
}

// CreateArticleInner holds the article fields within a create-article request body.
type CreateArticleInner struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	TagList     []string `json:"tagList"`
}

// CreateArticleRequest is the top-level JSON wrapper for POST /api/articles.
type CreateArticleRequest struct {
	Article CreateArticleInner `json:"article"`
}

// ArticleAuthor holds the author profile fields embedded in article API responses.
type ArticleAuthor struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

// ArticleResponseInner holds the full article fields returned in single-article API responses.
type ArticleResponseInner struct {
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Body           string        `json:"body"`
	TagList        []string      `json:"tagList"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	Favorited      bool          `json:"favorited"`
	FavoritesCount int           `json:"favoritesCount"`
	Author         ArticleAuthor `json:"author"`
}

// ArticleResponse is the top-level JSON wrapper for single-article API responses.
type ArticleResponse struct {
	Article ArticleResponseInner `json:"article"`
}

// ArticleListItemInner holds the article fields (without body) used in list API responses.
type ArticleListItemInner struct {
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	TagList        []string      `json:"tagList"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	Favorited      bool          `json:"favorited"`
	FavoritesCount int           `json:"favoritesCount"`
	Author         ArticleAuthor `json:"author"`
}

// ArticlesResponse is the top-level JSON wrapper for article list API responses.
type ArticlesResponse struct {
	Articles      []ArticleListItemInner `json:"articles"`
	ArticlesCount int                    `json:"articlesCount"`
}

func articleResponse(a *domain.Article) ArticleResponse {
	return ArticleResponse{
		Article: ArticleResponseInner{
			Slug:           a.Slug,
			Title:          a.Title,
			Description:    a.Description,
			Body:           a.Body,
			TagList:        a.TagList,
			CreatedAt:      a.CreatedAt,
			UpdatedAt:      a.UpdatedAt,
			Favorited:      a.Favorited,
			FavoritesCount: a.FavoritesCount,
			Author: ArticleAuthor{
				Username:  a.Author.Username,
				Bio:       a.Author.Bio,
				Image:     a.Author.Image,
				Following: a.Author.Following,
			},
		},
	}
}

// CreateArticle handles POST /api/articles and publishes a new article.
func (h *Handler) CreateArticle(w http.ResponseWriter, r *http.Request) {
	authorID := r.Context().Value(userIDKey).(int)

	var req CreateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.CreateArticle{
		Title:       req.Article.Title,
		Description: req.Article.Description,
		Body:        req.Article.Body,
		TagList:     req.Article.TagList,
	}

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.CreateArticle(r.Context(), authorID, &d)
	if err != nil {
		var validationErr *domain.ValidationError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &dupErr) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write(createErrResponse(dupErr.Field, []string{dupErr.Msg}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

// GetArticle handles GET /api/articles/{slug} and returns a single article.
func (h *Handler) GetArticle(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	viewerID, _ := r.Context().Value(userIDKey).(int)

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.GetArticleBySlug(r.Context(), slug, viewerID)
	if err != nil {
		writeArticleErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

// UpdateArticleInner holds the optional fields within an update-article request body.
type UpdateArticleInner struct {
	Title       *string             `json:"title"`
	Description *string             `json:"description"`
	Body        *string             `json:"body"`
	TagList     NullableStringSlice `json:"tagList"`
}

// UpdateArticleRequest is the top-level JSON wrapper for PUT /api/articles/{slug}.
type UpdateArticleRequest struct {
	Article UpdateArticleInner `json:"article"`
}

// UpdateArticle handles PUT /api/articles/{slug} and applies partial updates to an article.
func (h *Handler) UpdateArticle(w http.ResponseWriter, r *http.Request) {
	callerID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	var req UpdateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Article.TagList.Present && req.Article.TagList.IsNull {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write(createErrResponse("tagList", []string{"can't be null"}))
		return
	}

	u := domain.UpdateArticle{
		Title:       req.Article.Title,
		Description: req.Article.Description,
		Body:        req.Article.Body,
	}
	if req.Article.TagList.Present {
		tags := req.Article.TagList.Value
		if tags == nil {
			tags = []string{}
		}
		u.TagList = &tags
	}

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.UpdateArticle(r.Context(), callerID, slug, &u)
	if err != nil {
		var validationErr *domain.ValidationError
		var notFoundErr *domain.ArticleNotFoundError
		var forbiddenErr *domain.ForbiddenError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else if errors.As(err, &forbiddenErr) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write(createErrResponse("article", []string{"forbidden"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

// FavoriteArticle handles POST /api/articles/{slug}/favorite and marks an article as favorited.
func (h *Handler) FavoriteArticle(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.FavoriteArticle(r.Context(), userID, slug)
	if err != nil {
		writeArticleErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

// UnfavoriteArticle handles DELETE /api/articles/{slug}/favorite and removes the favorite mark.
func (h *Handler) UnfavoriteArticle(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.UnfavoriteArticle(r.Context(), userID, slug)
	if err != nil {
		writeArticleErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

// CommentAuthor holds the author profile fields embedded in comment API responses.
type CommentAuthor struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

// CommentResponseInner holds the comment fields returned in comment API responses.
type CommentResponseInner struct {
	ID        int           `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Body      string        `json:"body"`
	Author    CommentAuthor `json:"author"`
}

// CommentResponse is the top-level JSON wrapper for single-comment API responses.
type CommentResponse struct {
	Comment CommentResponseInner `json:"comment"`
}

// CommentsResponse is the top-level JSON wrapper for comment list API responses.
type CommentsResponse struct {
	Comments []CommentResponseInner `json:"comments"`
}

// CreateArticleComment handles POST /api/articles/{slug}/comments and adds a comment to an article.
func (h *Handler) CreateArticleComment(w http.ResponseWriter, r *http.Request) {
	authorID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	var req struct {
		Comment struct {
			Body string `json:"body"`
		} `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	comment, err := h.commentService.CreateComment(r.Context(), authorID, slug, &domain.CreateComment{Body: req.Comment.Body})
	if err != nil {
		var validationErr *domain.ValidationError
		var notFoundErr *domain.ArticleNotFoundError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CommentResponse{
		Comment: CommentResponseInner{
			ID:        comment.ID,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Body:      comment.Body,
			Author: CommentAuthor{
				Username:  comment.Author.Username,
				Bio:       comment.Author.Bio,
				Image:     comment.Author.Image,
				Following: comment.Author.Following,
			},
		},
	})
}

// DeleteArticle handles DELETE /api/articles/{slug} and removes an article authored by the caller.
func (h *Handler) DeleteArticle(w http.ResponseWriter, r *http.Request) {
	callerID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	w.Header().Set("Content-Type", "application/json")

	if err := h.articleService.DeleteArticle(r.Context(), callerID, slug); err != nil {
		var notFoundErr *domain.ArticleNotFoundError
		var forbiddenErr *domain.ForbiddenError
		if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else if errors.As(err, &forbiddenErr) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write(createErrResponse("article", []string{"forbidden"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListArticles handles GET /api/articles and returns a filtered, paginated list of articles.
func (h *Handler) ListArticles(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := r.Context().Value(userIDKey).(int)

	q := r.URL.Query()

	filter := domain.ListArticlesFilter{
		Limit:  20,
		Offset: 0,
	}
	if v := q.Get("tag"); v != "" {
		filter.Tag = &v
	}
	if v := q.Get("author"); v != "" {
		filter.Author = &v
	}
	if v := q.Get("favorited"); v != "" {
		filter.Favorited = &v
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		filter.Limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil {
		filter.Offset = v
	}

	w.Header().Set("Content-Type", "application/json")

	list, err := h.articleService.ListArticles(r.Context(), filter, viewerID)
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		return
	}

	resp := ArticlesResponse{
		Articles:      make([]ArticleListItemInner, 0, len(list.Articles)),
		ArticlesCount: list.TotalCount,
	}
	for _, a := range list.Articles {
		resp.Articles = append(resp.Articles, ArticleListItemInner{
			Slug:           a.Slug,
			Title:          a.Title,
			Description:    a.Description,
			TagList:        a.TagList,
			CreatedAt:      a.CreatedAt,
			UpdatedAt:      a.UpdatedAt,
			Favorited:      a.Favorited,
			FavoritesCount: a.FavoritesCount,
			Author: ArticleAuthor{
				Username:  a.Author.Username,
				Bio:       a.Author.Bio,
				Image:     a.Author.Image,
				Following: a.Author.Following,
			},
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetArticleFeed handles GET /api/articles/feed and returns articles from followed authors.
func (h *Handler) GetArticleFeed(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)

	q := r.URL.Query()

	filter := domain.ArticleFeedFilter{
		Limit:  20,
		Offset: 0,
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		filter.Limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil {
		filter.Offset = v
	}

	w.Header().Set("Content-Type", "application/json")

	list, err := h.articleService.FeedArticles(r.Context(), filter, userID)
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		return
	}

	resp := ArticlesResponse{
		Articles:      make([]ArticleListItemInner, 0, len(list.Articles)),
		ArticlesCount: list.TotalCount,
	}
	for _, a := range list.Articles {
		resp.Articles = append(resp.Articles, ArticleListItemInner{
			Slug:           a.Slug,
			Title:          a.Title,
			Description:    a.Description,
			TagList:        a.TagList,
			CreatedAt:      a.CreatedAt,
			UpdatedAt:      a.UpdatedAt,
			Favorited:      a.Favorited,
			FavoritesCount: a.FavoritesCount,
			Author: ArticleAuthor{
				Username:  a.Author.Username,
				Bio:       a.Author.Bio,
				Image:     a.Author.Image,
				Following: a.Author.Following,
			},
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// DeleteArticleComment handles DELETE /api/articles/{slug}/comments/{id} and removes a comment authored by the caller.
func (h *Handler) DeleteArticleComment(w http.ResponseWriter, r *http.Request) {
	callerID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	commentID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(createErrResponse("id", []string{"must be an integer"}))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := h.commentService.DeleteComment(r.Context(), callerID, slug, commentID); err != nil {
		var notFoundArticle *domain.ArticleNotFoundError
		var notFoundComment *domain.CommentNotFoundError
		var forbiddenErr *domain.ForbiddenError
		if errors.As(err, &notFoundArticle) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else if errors.As(err, &notFoundComment) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("comment", []string{"not found"}))
		} else if errors.As(err, &forbiddenErr) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write(createErrResponse("comment", []string{"forbidden"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetArticleComments handles GET /api/articles/{slug}/comments and returns all comments on an article.
func (h *Handler) GetArticleComments(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	viewerID, _ := r.Context().Value(userIDKey).(int)

	w.Header().Set("Content-Type", "application/json")

	comments, err := h.commentService.GetComments(r.Context(), slug, viewerID)
	if err != nil {
		var notFoundErr *domain.ArticleNotFoundError
		if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	resp := CommentsResponse{Comments: make([]CommentResponseInner, 0, len(comments))}
	for _, c := range comments {
		resp.Comments = append(resp.Comments, CommentResponseInner{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			Author: CommentAuthor{
				Username:  c.Author.Username,
				Bio:       c.Author.Bio,
				Image:     c.Author.Image,
				Following: c.Author.Following,
			},
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
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

// TagsResponse is the top-level JSON wrapper for the tags listing API response.
type TagsResponse struct {
	Tags []string `json:"tags"`
}

// HealthCheck handles GET /api/healthcheck and returns 200 OK if the server is running.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// GetTags handles GET /api/tags and returns all tags used on published articles.
func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tags, err := h.tagService.GetTags(r.Context())
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(TagsResponse{Tags: tags})
}
