package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// TagsResponse is the top-level JSON wrapper for the tags listing API response.
type TagsResponse struct {
	Tags []string `json:"tags"`
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
