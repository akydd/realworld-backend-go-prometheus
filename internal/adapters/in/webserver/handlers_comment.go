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
