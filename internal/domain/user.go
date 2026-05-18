package domain

import (
	"context"
	"strconv"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

type userRepo interface {
	InsertUser(ctx context.Context, u *RegisterUser) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, string, error)
	GetUserByID(ctx context.Context, id int) (*User, error)
	GetFullUserByID(ctx context.Context, id int) (*User, string, error)
	UpdateUser(ctx context.Context, userID int, u *UpdateUserData) (*User, error)
}

// UserController implements the user management use-cases of the domain.
type UserController struct {
	repo      userRepo
	jwtSecret string
}

// New creates a UserController backed by the given repository and JWT signing secret.
func New(r userRepo, jwtSecret string) *UserController {
	return &UserController{
		repo:      r,
		jwtSecret: jwtSecret,
	}
}

// RegisterUser validates the registration request, hashes the password, persists the
// new account, and returns the created user with a fresh JWT token.
func (c *UserController) RegisterUser(ctx context.Context, u *RegisterUser) (*User, error) {
	if err := validateRegisterUser(u); err != nil {
		return nil, err
	}

	hash, err := argon2id.CreateHash((u.Password), argon2id.DefaultParams)
	if err != nil {
		return nil, err
	}
	u.Password = hash

	user, err := c.repo.InsertUser(ctx, u)
	if err != nil {
		return nil, err
	}

	token, err := generateToken(user.ID, c.jwtSecret)
	if err != nil {
		return nil, err
	}
	user.Token = token

	return user, nil
}

func generateToken(id int, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.Itoa(id),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// LoginUser authenticates the user with the provided credentials and returns the user with a fresh JWT token.
func (c *UserController) LoginUser(ctx context.Context, u *LoginUser) (*User, error) {
	if u.Email == "" {
		return nil, NewValidationError("email", blankFieldErrMsg)
	}
	if u.Password == "" {
		return nil, NewValidationError("password", blankFieldErrMsg)
	}

	user, hashedPassword, err := c.repo.GetUserByEmail(ctx, u.Email)
	if err != nil {
		return nil, err
	}

	match, err := argon2id.ComparePasswordAndHash(u.Password, hashedPassword)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, &CredentialsError{}
	}

	token, err := generateToken(user.ID, c.jwtSecret)
	if err != nil {
		return nil, err
	}
	user.Token = token

	return user, nil
}

// GetUser retrieves the currently authenticated user by ID and returns it with a refreshed JWT token.
func (c *UserController) GetUser(ctx context.Context, userID int) (*User, error) {
	user, err := c.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	newToken, err := generateToken(user.ID, c.jwtSecret)
	if err != nil {
		return nil, err
	}
	user.Token = newToken

	return user, nil
}

// UpdateUser applies the supplied changes to the user record and returns the updated user with a fresh JWT token.
func (c *UserController) UpdateUser(ctx context.Context, userID int, u *UpdateUser) (*User, error) {
	if err := validateUpdateUser(u); err != nil {
		return nil, err
	}

	current, hashedPassword, err := c.repo.GetFullUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	data := UpdateUserData{
		Email:    current.Email,
		Username: current.Username,
		Password: hashedPassword,
		Bio:      current.Bio,
		Image:    current.Image,
	}

	if u.Email != nil {
		data.Email = *u.Email
	}
	if u.Username != nil {
		data.Username = *u.Username
	}
	if u.Password != nil {
		hash, err := argon2id.CreateHash(*u.Password, argon2id.DefaultParams)
		if err != nil {
			return nil, err
		}
		data.Password = hash
	}
	if u.Bio != nil {
		data.Bio = *u.Bio
	}
	if u.Image != nil {
		data.Image = *u.Image
	}

	updated, err := c.repo.UpdateUser(ctx, userID, &data)
	if err != nil {
		return nil, err
	}

	newToken, err := generateToken(updated.ID, c.jwtSecret)
	if err != nil {
		return nil, err
	}
	updated.Token = newToken

	return updated, nil
}

func validateUpdateUser(u *UpdateUser) error {
	if u.Email == nil && u.Bio == nil && u.Image == nil && u.Username == nil && u.Password == nil {
		return NewValidationError("user", blankFieldErrMsg)
	}
	if u.Email != nil && *u.Email == "" {
		return NewValidationError("email", blankFieldErrMsg)
	}
	if u.Username != nil && *u.Username == "" {
		return NewValidationError("username", blankFieldErrMsg)
	}
	if u.Password != nil && *u.Password == "" {
		return NewValidationError("password", blankFieldErrMsg)
	}
	if u.Password != nil && len(*u.Password) < 8 {
		return NewValidationError("password", "is too short (minimum is 8 characters)")
	}
	return nil
}

func validateRegisterUser(r *RegisterUser) error {
	if r.Email == "" {
		return NewValidationError("email", blankFieldErrMsg)
	}

	if r.Password == "" {
		return NewValidationError("password", blankFieldErrMsg)
	}

	if r.Username == "" {
		return NewValidationError("username", blankFieldErrMsg)
	}

	return nil
}
