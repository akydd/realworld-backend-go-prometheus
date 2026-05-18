package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"realworld-backend-go/internal/domain"

	"github.com/gorilla/mux"
)

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
