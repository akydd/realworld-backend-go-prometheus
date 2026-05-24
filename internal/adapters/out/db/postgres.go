package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"embed"
	"realworld-backend-go/internal/domain"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

const (
	articleNotifyChannel = "articles"
	commentNotifyPrefix  = "comments:"
)

// Postgres is the PostgreSQL adapter that satisfies all repository interfaces used by the domain layer.
type Postgres struct {
	db  *sqlx.DB
	dsn string
}

// DBConfig holds the connection parameters required to open a PostgreSQL database.
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func validateConfig(c *DBConfig) error {
	if c.Host == "" {
		return errors.New("db config must contain a host")
	}

	if c.Port == "" {
		return errors.New("db config must contain a port")
	}

	if c.User == "" {
		return errors.New("db config must contain a user")
	}

	if c.Password == "" {
		return errors.New("db config must contain a password")
	}

	if c.Name == "" {
		return errors.New("db config must contain a name")
	}
	return nil
}

// New opens a PostgreSQL connection using the provided config, runs all pending
// database migrations via goose, and returns a ready-to-use Postgres adapter.
func New(config *DBConfig) (*Postgres, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Name)

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}

	if err := goose.Up(db.DB, "migrations"); err != nil {
		return nil, err
	}

	return &Postgres{
		db:  db,
		dsn: dsn,
	}, nil
}

type user struct {
	ID       int            `db:"id"`
	Email    string         `db:"email"`
	Username string         `db:"username"`
	Bio      sql.NullString `db:"bio"`
	Image    sql.NullString `db:"image"`
}

type userWithPassword struct {
	user
	Password string `db:"password"`
}

func convertUser(u user) domain.User {
	d := domain.User{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
	}

	if u.Bio.Valid {
		s := u.Bio.String
		d.Bio = &s
	}

	if u.Image.Valid {
		s := u.Image.String
		d.Image = &s
	}

	return d
}

type profileRow struct {
	Username  string         `db:"username"`
	Bio       sql.NullString `db:"bio"`
	Image     sql.NullString `db:"image"`
	Following bool           `db:"following"`
}

func convertProfile(r profileRow) *domain.Profile {
	p := &domain.Profile{
		Username:  r.Username,
		Following: r.Following,
	}
	if r.Bio.Valid {
		s := r.Bio.String
		p.Bio = &s
	}
	if r.Image.Valid {
		s := r.Image.String
		p.Image = &s
	}
	return p
}

// GetProfileByUsername retrieves the public profile of the user with the given username,
// setting the Following flag based on whether viewerID follows that user.
func (p *Postgres) GetProfileByUsername(ctx context.Context, profileUsername string, viewerID int) (*domain.Profile, error) {
	query := `
		SELECT u.username, u.bio, u.image,
			CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
		FROM users u
		LEFT JOIN follows f ON f.followee_id = u.id AND f.follower_id = $2
		WHERE u.username = $1`
	var row profileRow

	err := p.db.QueryRowxContext(ctx, query, profileUsername, viewerID).StructScan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ProfileNotFoundError{}
		}
		return nil, err
	}

	return convertProfile(row), nil
}

// FollowUser records a follow relationship from followerID to the user identified by followeeUsername
// and returns the updated profile.
func (p *Postgres) FollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error) {
	var followeeID int
	err := p.db.QueryRowxContext(ctx, "SELECT id FROM users WHERE username = $1", followeeUsername).Scan(&followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ProfileNotFoundError{}
		}
		return nil, err
	}

	_, err = p.db.ExecContext(ctx,
		"INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		followerID, followeeID)
	if err != nil {
		return nil, err
	}

	return p.GetProfileByUsername(ctx, followeeUsername, followerID)
}

// UnfollowUser removes the follow relationship from followerID to the user identified by followeeUsername
// and returns the updated profile.
func (p *Postgres) UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error) {
	var followeeID int
	err := p.db.QueryRowxContext(ctx, "SELECT id FROM users WHERE username = $1", followeeUsername).Scan(&followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ProfileNotFoundError{}
		}
		return nil, err
	}

	_, err = p.db.ExecContext(ctx,
		"DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2",
		followerID, followeeID)
	if err != nil {
		return nil, err
	}

	return p.GetProfileByUsername(ctx, followeeUsername, followerID)
}

// GetUserByID retrieves a user by their primary-key ID, returning a CredentialsError if not found.
func (p *Postgres) GetUserByID(ctx context.Context, id int) (*domain.User, error) {
	query := "SELECT id, username, email, bio, image FROM users WHERE id = $1"
	var dbUser user

	err := p.db.QueryRowxContext(ctx, query, id).StructScan(&dbUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.CredentialsError{}
		}
		return nil, err
	}

	u := convertUser(dbUser)
	return &u, nil
}

// GetFullUserByID retrieves a user together with their hashed password by primary-key ID.
func (p *Postgres) GetFullUserByID(ctx context.Context, id int) (*domain.User, string, error) {
	query := "SELECT id, username, email, bio, image, password FROM users WHERE id = $1"
	var dbUser userWithPassword

	err := p.db.QueryRowxContext(ctx, query, id).StructScan(&dbUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", &domain.CredentialsError{}
		}
		return nil, "", err
	}

	u := convertUser(dbUser.user)
	return &u, dbUser.Password, nil
}

// UpdateUser writes the fully resolved UpdateUserData to the users table and returns the updated record.
func (p *Postgres) UpdateUser(ctx context.Context, userID int, u *domain.UpdateUserData) (*domain.User, error) {
	query := `UPDATE users SET email=$1, username=$2, password=$3, bio=$4, image=$5 WHERE id=$6 RETURNING id, username, email, bio, image`
	var dbUser user

	err := p.db.QueryRowxContext(ctx, query, u.Email, u.Username, u.Password, u.Bio, u.Image, userID).StructScan(&dbUser)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "users_email_unique":
				return nil, domain.NewDuplicateError("email")
			case "users_username_unique":
				return nil, domain.NewDuplicateError("username")
			}
		}
		return nil, err
	}

	updated := convertUser(dbUser)
	return &updated, nil
}

// GetUserByEmail retrieves a user together with their hashed password by email address.
func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (*domain.User, string, error) {
	query := "SELECT id, username, email, bio, image, password FROM users WHERE email = $1"
	var dbUser userWithPassword

	err := p.db.QueryRowxContext(ctx, query, email).StructScan(&dbUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", &domain.CredentialsError{}
		}
		return nil, "", err
	}

	user := convertUser(dbUser.user)
	return &user, dbUser.Password, nil
}

type articleRow struct {
	ID          int       `db:"id"`
	Slug        string    `db:"slug"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	Body        string    `db:"body"`
	AuthorID    int       `db:"author_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// InsertArticle persists a new article (with tags) in a transaction, auto-disambiguating the slug if needed.
func (p *Postgres) InsertArticle(ctx context.Context, authorID int, slug string, a *domain.CreateArticle) (*domain.Article, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	query := `
		INSERT INTO articles (slug, title, description, body, author_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, slug, title, description, body, author_id, created_at, updated_at`
	var row articleRow

	baseSlug := slug
	for i := 2; ; i++ {
		var exists bool
		if err = tx.QueryRowxContext(ctx, `SELECT EXISTS(SELECT 1 FROM articles WHERE slug = $1)`, slug).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			break
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}

	err = tx.QueryRowxContext(ctx, query, slug, a.Title, a.Description, a.Body, authorID).StructScan(&row)
	if err != nil {
		return nil, err
	}

	if len(a.TagList) > 0 {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO tags (name) SELECT unnest($1::text[]) ON CONFLICT (name) DO NOTHING`,
			pq.Array(a.TagList))
		if err != nil {
			return nil, err
		}

		var tagIDs []int
		err = tx.SelectContext(ctx, &tagIDs,
			`SELECT id FROM tags WHERE name = ANY($1) ORDER BY array_position($1::text[], name)`,
			pq.Array(a.TagList))
		if err != nil {
			return nil, err
		}

		for _, tagID := range tagIDs {
			_, err = tx.ExecContext(ctx,
				`INSERT INTO article_tags (article_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				row.ID, tagID)
			if err != nil {
				return nil, err
			}
		}
	}

	var authorRow struct {
		Username string         `db:"username"`
		Bio      sql.NullString `db:"bio"`
		Image    sql.NullString `db:"image"`
	}
	err = tx.QueryRowxContext(ctx, "SELECT username, bio, image FROM users WHERE id = $1", authorID).StructScan(&authorRow)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	author := domain.Profile{Username: authorRow.Username, Following: false}
	if authorRow.Bio.Valid {
		s := authorRow.Bio.String
		author.Bio = &s
	}
	if authorRow.Image.Valid {
		s := authorRow.Image.String
		author.Image = &s
	}

	tagList := a.TagList
	if tagList == nil {
		tagList = []string{}
	}

	return &domain.Article{
		Slug:           row.Slug,
		Title:          row.Title,
		Description:    row.Description,
		Body:           row.Body,
		TagList:        tagList,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Favorited:      false,
		FavoritesCount: 0,
		Author:         author,
	}, nil
}

type articleWithTagsRow struct {
	Slug           string         `db:"slug"`
	Title          string         `db:"title"`
	Description    string         `db:"description"`
	Body           string         `db:"body"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	AuthorUsername string         `db:"author_username"`
	AuthorBio      sql.NullString `db:"author_bio"`
	AuthorImage    sql.NullString `db:"author_image"`
	Following      bool           `db:"following"`
	TagList        pq.StringArray `db:"tag_list"`
	Favorited      bool           `db:"favorited"`
	FavoritesCount int            `db:"favorites_count"`
}

// GetArticleBySlug retrieves a single article by slug, enriched with viewer-specific favorited and
// following flags.
func (p *Postgres) GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error) {
	query := `
		SELECT
			a.slug,
			a.title,
			a.description,
			a.body,
			a.created_at,
			a.updated_at,
			u.username  AS author_username,
			u.bio       AS author_bio,
			u.image     AS author_image,
			CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following,
			COALESCE(ARRAY_AGG(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tag_list,
			CASE WHEN fav.user_id IS NOT NULL THEN true ELSE false END AS favorited,
			(SELECT COUNT(*) FROM article_favorites WHERE article_id = a.id) AS favorites_count
		FROM articles a
		JOIN users u ON u.id = a.author_id
		LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $2
		LEFT JOIN article_tags at ON at.article_id = a.id
		LEFT JOIN tags t ON t.id = at.tag_id
		LEFT JOIN article_favorites fav ON fav.article_id = a.id AND fav.user_id = $2
		WHERE a.slug = $1
		GROUP BY a.id, u.username, u.bio, u.image, f.follower_id, fav.user_id`

	var row articleWithTagsRow
	err := p.db.QueryRowxContext(ctx, query, slug, viewerID).StructScan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ArticleNotFoundError{}
		}
		return nil, err
	}

	author := domain.Profile{
		Username:  row.AuthorUsername,
		Following: row.Following,
	}
	if row.AuthorBio.Valid {
		s := row.AuthorBio.String
		author.Bio = &s
	}
	if row.AuthorImage.Valid {
		s := row.AuthorImage.String
		author.Image = &s
	}

	tagList := []string(row.TagList)
	if tagList == nil {
		tagList = []string{}
	}

	return &domain.Article{
		Slug:           row.Slug,
		Title:          row.Title,
		Description:    row.Description,
		Body:           row.Body,
		TagList:        tagList,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Favorited:      row.Favorited,
		FavoritesCount: row.FavoritesCount,
		Author:         author,
	}, nil
}

// FavoriteArticle inserts a favorite record for the given user and article and returns the updated article.
func (p *Postgres) FavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error) {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO article_favorites (user_id, article_id)
		 SELECT $1, id FROM articles WHERE slug = $2
		 ON CONFLICT DO NOTHING`,
		userID, slug)
	if err != nil {
		return nil, err
	}
	return p.GetArticleBySlug(ctx, slug, userID)
}

// UnfavoriteArticle removes the favorite record for the given user and article and returns the updated article.
func (p *Postgres) UnfavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error) {
	_, err := p.db.ExecContext(ctx,
		`DELETE FROM article_favorites
		 WHERE user_id = $1 AND article_id = (SELECT id FROM articles WHERE slug = $2)`,
		userID, slug)
	if err != nil {
		return nil, err
	}
	return p.GetArticleBySlug(ctx, slug, userID)
}

// UpdateArticle applies partial updates to the article identified by slug in a transaction,
// enforcing authorship and auto-disambiguating the new slug when the title changes.
func (p *Postgres) UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var cur struct {
		ID          int    `db:"id"`
		AuthorID    int    `db:"author_id"`
		Title       string `db:"title"`
		Description string `db:"description"`
		Body        string `db:"body"`
	}
	err = tx.QueryRowxContext(ctx,
		`SELECT id, author_id, title, description, body FROM articles WHERE slug = $1`,
		slug).StructScan(&cur)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ArticleNotFoundError{}
		}
		return nil, err
	}

	if cur.AuthorID != callerID {
		return nil, &domain.ForbiddenError{}
	}

	newTitle := cur.Title
	newDescription := cur.Description
	newBody := cur.Body
	newSlug := slug

	if u.Title != nil {
		newTitle = *u.Title
		baseSlug := domain.GenerateSlug(newTitle)
		newSlug = baseSlug
		for i := 2; ; i++ {
			var exists bool
			if err = tx.QueryRowxContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM articles WHERE slug = $1 AND id != $2)`, newSlug, cur.ID).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				break
			}
			newSlug = fmt.Sprintf("%s-%d", baseSlug, i)
		}
	}
	if u.Description != nil {
		newDescription = *u.Description
	}
	if u.Body != nil {
		newBody = *u.Body
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE articles SET slug=$1, title=$2, description=$3, body=$4, updated_at=now() WHERE id=$5`,
		newSlug, newTitle, newDescription, newBody, cur.ID)
	if err != nil {
		return nil, err
	}

	if u.TagList != nil {
		if _, err = tx.ExecContext(ctx,
			`DELETE FROM article_tags WHERE article_id = $1`, cur.ID); err != nil {
			return nil, err
		}
		if len(*u.TagList) > 0 {
			if _, err = tx.ExecContext(ctx,
				`INSERT INTO tags (name) SELECT unnest($1::text[]) ON CONFLICT (name) DO NOTHING`,
				pq.Array(*u.TagList)); err != nil {
				return nil, err
			}
			var tagIDs []int
			if err = tx.SelectContext(ctx, &tagIDs,
				`SELECT id FROM tags WHERE name = ANY($1)`,
				pq.Array(*u.TagList)); err != nil {
				return nil, err
			}
			for _, tagID := range tagIDs {
				if _, err = tx.ExecContext(ctx,
					`INSERT INTO article_tags (article_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
					cur.ID, tagID); err != nil {
					return nil, err
				}
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return p.GetArticleBySlug(ctx, newSlug, callerID)
}

// InsertComment persists a new comment on the article identified by articleSlug and returns the created comment.
func (p *Postgres) InsertComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error) {
	query := `
		WITH ins AS (
			INSERT INTO comments (body, author_id, article_id)
			SELECT $1, $2, id FROM articles WHERE slug = $3
			RETURNING id, body, author_id, created_at, updated_at
		)
		SELECT ins.id, ins.body, ins.created_at, ins.updated_at,
		       u.username, u.bio, u.image
		FROM ins
		JOIN users u ON u.id = ins.author_id`

	var row struct {
		ID        int            `db:"id"`
		Body      string         `db:"body"`
		CreatedAt time.Time      `db:"created_at"`
		UpdatedAt time.Time      `db:"updated_at"`
		Username  string         `db:"username"`
		Bio       sql.NullString `db:"bio"`
		Image     sql.NullString `db:"image"`
	}

	err := p.db.QueryRowxContext(ctx, query, c.Body, authorID, articleSlug).StructScan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ArticleNotFoundError{}
		}
		return nil, err
	}

	author := domain.Profile{Username: row.Username, Following: false}
	if row.Bio.Valid {
		s := row.Bio.String
		author.Bio = &s
	}
	if row.Image.Valid {
		s := row.Image.String
		author.Image = &s
	}

	return &domain.Comment{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		Body:      row.Body,
		Author:    author,
	}, nil
}

// GetCommentsByArticleSlug retrieves all comments for the article identified by articleSlug,
// ordered by creation time ascending.
func (p *Postgres) GetCommentsByArticleSlug(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error) {
	var articleID int
	err := p.db.QueryRowxContext(ctx, `SELECT id FROM articles WHERE slug = $1`, articleSlug).Scan(&articleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ArticleNotFoundError{}
		}
		return nil, err
	}

	query := `
		SELECT
			c.id, c.body, c.created_at, c.updated_at,
			u.username, u.bio, u.image,
			CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
		FROM comments c
		JOIN users u ON u.id = c.author_id
		LEFT JOIN follows f ON f.followee_id = c.author_id AND f.follower_id = $2
		WHERE c.article_id = $1
		ORDER BY c.created_at ASC`

	var rows []struct {
		ID        int            `db:"id"`
		Body      string         `db:"body"`
		CreatedAt time.Time      `db:"created_at"`
		UpdatedAt time.Time      `db:"updated_at"`
		Username  string         `db:"username"`
		Bio       sql.NullString `db:"bio"`
		Image     sql.NullString `db:"image"`
		Following bool           `db:"following"`
	}

	if err := p.db.SelectContext(ctx, &rows, query, articleID, viewerID); err != nil {
		return nil, err
	}

	comments := make([]*domain.Comment, 0, len(rows))
	for _, row := range rows {
		author := domain.Profile{Username: row.Username, Following: row.Following}
		if row.Bio.Valid {
			s := row.Bio.String
			author.Bio = &s
		}
		if row.Image.Valid {
			s := row.Image.String
			author.Image = &s
		}
		comments = append(comments, &domain.Comment{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			Body:      row.Body,
			Author:    author,
		})
	}
	return comments, nil
}

type listRow struct {
	Slug           string         `db:"slug"`
	Title          string         `db:"title"`
	Description    string         `db:"description"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	AuthorUsername string         `db:"author_username"`
	AuthorBio      sql.NullString `db:"author_bio"`
	AuthorImage    sql.NullString `db:"author_image"`
	Following      bool           `db:"following"`
	TagList        pq.StringArray `db:"tag_list"`
	Favorited      bool           `db:"favorited"`
	FavoritesCount int            `db:"favorites_count"`
	TotalCount     int            `db:"total_count"`
}

func buildArticleListQuery(conditions []string, args []any, limit, offset int) (string, []any) {
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	query := `
		SELECT
			a.slug,
			a.title,
			a.description,
			a.created_at,
			a.updated_at,
			u.username  AS author_username,
			u.bio       AS author_bio,
			u.image     AS author_image,
			CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following,
			COALESCE(ARRAY_AGG(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tag_list,
			CASE WHEN fav.user_id IS NOT NULL THEN true ELSE false END AS favorited,
			(SELECT COUNT(*) FROM article_favorites WHERE article_id = a.id) AS favorites_count,
			COUNT(*) OVER() AS total_count
		FROM articles a
		JOIN users u ON u.id = a.author_id
		LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $1
		LEFT JOIN article_tags at ON at.article_id = a.id
		LEFT JOIN tags t ON t.id = at.tag_id
		LEFT JOIN article_favorites fav ON fav.article_id = a.id AND fav.user_id = $1`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	limitArg := nextArg(limit)
	offsetArg := nextArg(offset)
	query += fmt.Sprintf(`
		GROUP BY a.id, u.username, u.bio, u.image, f.follower_id, fav.user_id
		ORDER BY a.created_at DESC
		LIMIT %s OFFSET %s`, limitArg, offsetArg)

	return query, args
}

func convertListRows(rows []listRow) *domain.ArticleList {
	result := &domain.ArticleList{
		Articles:   make([]*domain.Article, 0, len(rows)),
		TotalCount: 0,
	}
	for _, row := range rows {
		result.TotalCount = row.TotalCount
		author := domain.Profile{Username: row.AuthorUsername, Following: row.Following}
		if row.AuthorBio.Valid {
			s := row.AuthorBio.String
			author.Bio = &s
		}
		if row.AuthorImage.Valid {
			s := row.AuthorImage.String
			author.Image = &s
		}
		tagList := []string(row.TagList)
		if tagList == nil {
			tagList = []string{}
		}
		result.Articles = append(result.Articles, &domain.Article{
			Slug:           row.Slug,
			Title:          row.Title,
			Description:    row.Description,
			TagList:        tagList,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
			Favorited:      row.Favorited,
			FavoritesCount: row.FavoritesCount,
			Author:         author,
		})
	}
	return result
}

// totalArticleCount returns the total number of articles matching the given conditions,
// used to report an accurate count even when the paginated result is empty.
func (p *Postgres) totalArticleCount(ctx context.Context, conditions []string, args []any) (int, error) {
	q := `SELECT COUNT(DISTINCT a.id)
		FROM articles a
		JOIN users u ON u.id = a.author_id
		LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $1`
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}
	var count int
	if err := p.db.GetContext(ctx, &count, q, args...); err != nil {
		return 0, err
	}
	return count, nil
}

// ListArticles returns a paginated, optionally filtered list of articles from the global feed.
func (p *Postgres) ListArticles(ctx context.Context, filter domain.ListArticlesFilter, viewerID int) (*domain.ArticleList, error) {
	args := []any{viewerID}
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	var conditions []string

	if filter.Tag != nil {
		p := nextArg(*filter.Tag)
		conditions = append(conditions, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM article_tags at2 JOIN tags t2 ON t2.id = at2.tag_id WHERE at2.article_id = a.id AND lower(t2.name) = lower(%s))`, p))
	}
	if filter.Author != nil {
		p := nextArg(*filter.Author)
		conditions = append(conditions, fmt.Sprintf(`lower(u.username) = lower(%s)`, p))
	}
	if filter.Favorited != nil {
		p := nextArg(*filter.Favorited)
		conditions = append(conditions, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM article_favorites af2 JOIN users u2 ON u2.id = af2.user_id WHERE af2.article_id = a.id AND lower(u2.username) = lower(%s))`, p))
	}

	// Save args before buildArticleListQuery appends limit/offset.
	countArgs := append([]any{}, args...)

	query, args := buildArticleListQuery(conditions, args, filter.Limit, filter.Offset)

	var rows []listRow
	if err := p.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	result := convertListRows(rows)
	if len(rows) == 0 {
		total, err := p.totalArticleCount(ctx, conditions, countArgs)
		if err != nil {
			return nil, err
		}
		result.TotalCount = total
	}
	return result, nil
}

// FeedArticles returns a paginated list of articles from authors that viewerID follows.
func (p *Postgres) FeedArticles(ctx context.Context, filter domain.ArticleFeedFilter, viewerID int) (*domain.ArticleList, error) {
	args := []any{viewerID}
	conditions := []string{"f.follower_id IS NOT NULL"}

	// Save args before buildArticleListQuery appends limit/offset.
	countArgs := append([]any{}, args...)

	query, args := buildArticleListQuery(conditions, args, filter.Limit, filter.Offset)

	var rows []listRow
	if err := p.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	result := convertListRows(rows)
	if len(rows) == 0 {
		total, err := p.totalArticleCount(ctx, conditions, countArgs)
		if err != nil {
			return nil, err
		}
		result.TotalCount = total
	}
	return result, nil
}

// DeleteArticle removes the article identified by slug if callerID is its author.
func (p *Postgres) DeleteArticle(ctx context.Context, callerID int, slug string) error {
	var authorID int
	err := p.db.QueryRowxContext(ctx, `SELECT author_id FROM articles WHERE slug = $1`, slug).Scan(&authorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &domain.ArticleNotFoundError{}
		}
		return err
	}

	if authorID != callerID {
		return &domain.ForbiddenError{}
	}

	_, err = p.db.ExecContext(ctx, `DELETE FROM articles WHERE slug = $1`, slug)
	return err
}

// DeleteComment removes the comment identified by commentID from the given article if callerID is its author.
func (p *Postgres) DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error {
	var articleID int
	err := p.db.QueryRowxContext(ctx, `SELECT id FROM articles WHERE slug = $1`, articleSlug).Scan(&articleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &domain.ArticleNotFoundError{}
		}
		return err
	}

	var authorID int
	err = p.db.QueryRowxContext(ctx,
		`SELECT author_id FROM comments WHERE id = $1 AND article_id = $2`,
		commentID, articleID).Scan(&authorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &domain.CommentNotFoundError{}
		}
		return err
	}

	if authorID != callerID {
		return &domain.ForbiddenError{}
	}

	_, err = p.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, commentID)
	return err
}

// GetAllTags returns all tag names stored in the database, ordered alphabetically.
func (p *Postgres) GetAllTags(ctx context.Context) ([]string, error) {
	var tags []string
	err := p.db.SelectContext(ctx, &tags, `SELECT name FROM tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// InsertUser persists a new user record and returns the created user without the hashed password.
func (p *Postgres) InsertUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error) {
	query := "insert into users (username, email, password) values ($1, $2, $3) returning id, username, email, bio, image"
	var dbUser user

	err := p.db.QueryRowxContext(ctx, query, u.Username, u.Email, u.Password).StructScan(&dbUser)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "users_email_unique":
				return nil, domain.NewDuplicateError("email")
			case "users_username_unique":
				return nil, domain.NewDuplicateError("username")
			}
		}
		return nil, err
	}

	user := convertUser(dbUser)
	return &user, nil
}

type articleNotifyPayload struct {
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Body           string    `json:"body"`
	TagList        []string  `json:"tag_list"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Favorited      bool      `json:"favorited"`
	FavoritesCount int       `json:"favorites_count"`
	AuthorUsername string    `json:"author_username"`
	AuthorBio      *string   `json:"author_bio"`
	AuthorImage    *string   `json:"author_image"`
	AuthorFollowing bool     `json:"author_following"`
}

// PublishArticle notifies all listeners on the articles channel with the article payload.
func (p *Postgres) PublishArticle(ctx context.Context, a *domain.Article) error {
	payload := articleNotifyPayload{
		Slug:            a.Slug,
		Title:           a.Title,
		Description:     a.Description,
		Body:            a.Body,
		TagList:         a.TagList,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
		Favorited:       a.Favorited,
		FavoritesCount:  a.FavoritesCount,
		AuthorUsername:  a.Author.Username,
		AuthorBio:       a.Author.Bio,
		AuthorImage:     a.Author.Image,
		AuthorFollowing: a.Author.Following,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, "SELECT pg_notify($1, $2)", articleNotifyChannel, string(data))
	return err
}

// ArticleSubscribe listens on the Postgres articles notification channel and forwards
// each received article to the returned channel. The channel is closed when ctx is done.
func (p *Postgres) ArticleSubscribe(ctx context.Context) <-chan domain.Article {
	out := make(chan domain.Article)

	listener := pq.NewListener(p.dsn, 10*time.Second, time.Minute, nil)
	if err := listener.Listen(articleNotifyChannel); err != nil {
		listener.Close() //nolint:errcheck
		close(out)
		return out
	}

	go func() {
		defer close(out)
		defer listener.Close() //nolint:errcheck

		for {
			select {
			case <-ctx.Done():
				return
			case n := <-listener.NotificationChannel():
				if n == nil {
					return
				}
				var payload articleNotifyPayload
				if err := json.Unmarshal([]byte(n.Extra), &payload); err != nil {
					continue
				}
				tagList := payload.TagList
				if tagList == nil {
					tagList = []string{}
				}
				out <- domain.Article{
					Slug:           payload.Slug,
					Title:          payload.Title,
					Description:    payload.Description,
					Body:           payload.Body,
					TagList:        tagList,
					CreatedAt:      payload.CreatedAt,
					UpdatedAt:      payload.UpdatedAt,
					Favorited:      payload.Favorited,
					FavoritesCount: payload.FavoritesCount,
					Author: domain.Profile{
						Username:  payload.AuthorUsername,
						Bio:       payload.AuthorBio,
						Image:     payload.AuthorImage,
						Following: payload.AuthorFollowing,
					},
				}
			}
		}
	}()

	return out
}

type commentNotifyPayload struct {
	ID              int       `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Body            string    `json:"body"`
	AuthorUsername  string    `json:"author_username"`
	AuthorBio       *string   `json:"author_bio"`
	AuthorImage     *string   `json:"author_image"`
	AuthorFollowing bool      `json:"author_following"`
}

// PublishComment notifies all listeners on the per-slug comment channel with the comment payload.
func (p *Postgres) PublishComment(ctx context.Context, slug string, c *domain.Comment) error {
	payload := commentNotifyPayload{
		ID:              c.ID,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		Body:            c.Body,
		AuthorUsername:  c.Author.Username,
		AuthorBio:       c.Author.Bio,
		AuthorImage:     c.Author.Image,
		AuthorFollowing: c.Author.Following,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, "SELECT pg_notify($1, $2)", commentNotifyPrefix+slug, string(data))
	return err
}

// CommentSubscribe listens on the per-slug Postgres comment notification channel and forwards
// each received comment to the returned channel. The channel is closed when ctx is done.
func (p *Postgres) CommentSubscribe(ctx context.Context, slug string) <-chan domain.Comment {
	out := make(chan domain.Comment)

	listener := pq.NewListener(p.dsn, 10*time.Second, time.Minute, nil)
	if err := listener.Listen(commentNotifyPrefix + slug); err != nil {
		listener.Close() //nolint:errcheck
		close(out)
		return out
	}

	go func() {
		defer close(out)
		defer listener.Close() //nolint:errcheck

		for {
			select {
			case <-ctx.Done():
				return
			case n := <-listener.NotificationChannel():
				if n == nil {
					return
				}
				var payload commentNotifyPayload
				if err := json.Unmarshal([]byte(n.Extra), &payload); err != nil {
					continue
				}
				out <- domain.Comment{
					ID:        payload.ID,
					CreatedAt: payload.CreatedAt,
					UpdatedAt: payload.UpdatedAt,
					Body:      payload.Body,
					Author: domain.Profile{
						Username:  payload.AuthorUsername,
						Bio:       payload.AuthorBio,
						Image:     payload.AuthorImage,
						Following: payload.AuthorFollowing,
					},
				}
			}
		}
	}()

	return out
}
