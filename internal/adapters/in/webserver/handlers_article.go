package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"realworld-backend-go/internal/domain"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

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
