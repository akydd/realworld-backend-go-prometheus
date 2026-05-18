package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"realworld-backend-go/internal/domain"
)

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
