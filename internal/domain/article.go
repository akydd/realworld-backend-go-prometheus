package domain

import (
	"context"
	"regexp"
	"strings"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSlug converts a title into a URL-safe slug by lowercasing it and
// replacing runs of non-alphanumeric characters with hyphens.
func GenerateSlug(title string) string {
	lower := strings.ToLower(title)
	slug := nonAlphanumRe.ReplaceAllString(lower, "-")
	return strings.Trim(slug, "-")
}

func validateCreateArticle(a *CreateArticle) error {
	if a.Title == "" {
		return NewValidationError("title", blankFieldErrMsg)
	}
	if a.Description == "" {
		return NewValidationError("description", blankFieldErrMsg)
	}
	if a.Body == "" {
		return NewValidationError("body", blankFieldErrMsg)
	}
	return nil
}

func deduplicateTags(tags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

func validateUpdateArticle(u *UpdateArticle) error {
	if u.Title == nil && u.Description == nil && u.Body == nil && u.TagList == nil {
		return NewValidationError("article", blankFieldErrMsg)
	}
	if u.Title != nil && *u.Title == "" {
		return NewValidationError("title", blankFieldErrMsg)
	}
	return nil
}

type articleRepo interface {
	InsertArticle(ctx context.Context, authorID int, slug string, a *CreateArticle) (*Article, error)
	GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error)
	UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error)
	FavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error)
	UnfavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error)
	DeleteArticle(ctx context.Context, callerID int, slug string) error
	ListArticles(ctx context.Context, filter ListArticlesFilter, viewerID int) (*ArticleList, error)
	FeedArticles(ctx context.Context, filter ArticleFeedFilter, viewerID int) (*ArticleList, error)
	ViewerFollowsUser(ctx context.Context, viewerID int, username string) bool
}

type articlePublisher interface {
	PublishArticle(ctx context.Context, article *Article) error
}

type articleSubscriber interface {
	ArticleSubscribe(ctx context.Context) (<-chan Article, error)
}

// ArticleController implements the article management use-cases of the domain.
type ArticleController struct {
	repo articleRepo
	pub  articlePublisher
	sub  articleSubscriber
}

// NewArticleController creates an ArticleController backed by the given repository.
func NewArticleController(r articleRepo, p articlePublisher, s articleSubscriber) *ArticleController {
	return &ArticleController{
		repo: r,
		pub:  p,
		sub:  s,
	}
}

// CreateArticle validates the request, deduplicates tags, generates a slug from the title,
// and persists the new article.
func (c *ArticleController) CreateArticle(ctx context.Context, authorID int, a *CreateArticle) (*Article, error) {
	if err := validateCreateArticle(a); err != nil {
		return nil, err
	}

	a.TagList = deduplicateTags(a.TagList)

	slug := GenerateSlug(a.Title)

	article, err := c.repo.InsertArticle(ctx, authorID, slug, a)
	if err != nil {
		return nil, err
	}

	_ = c.pub.PublishArticle(ctx, article)

	return article, nil
}

// GetArticleBySlug retrieves a single article by its slug, enriching it with viewer-specific data.
func (c *ArticleController) GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error) {
	return c.repo.GetArticleBySlug(ctx, slug, viewerID)
}

// UpdateArticle validates and applies the supplied changes to the article identified by slug.
func (c *ArticleController) UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error) {
	if err := validateUpdateArticle(u); err != nil {
		return nil, err
	}
	return c.repo.UpdateArticle(ctx, callerID, slug, u)
}

// FavoriteArticle marks the article identified by slug as favorited by the given user.
func (c *ArticleController) FavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error) {
	return c.repo.FavoriteArticle(ctx, userID, slug)
}

// UnfavoriteArticle removes the favorite mark from the article identified by slug for the given user.
func (c *ArticleController) UnfavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error) {
	return c.repo.UnfavoriteArticle(ctx, userID, slug)
}

// DeleteArticle removes the article identified by slug if the caller is its author.
func (c *ArticleController) DeleteArticle(ctx context.Context, callerID int, slug string) error {
	return c.repo.DeleteArticle(ctx, callerID, slug)
}

// ListArticles returns a paginated, optionally filtered list of articles from the global feed.
func (c *ArticleController) ListArticles(ctx context.Context, filter ListArticlesFilter, viewerID int) (*ArticleList, error) {
	return c.repo.ListArticles(ctx, filter, viewerID)
}

// FeedArticles returns a paginated list of articles from authors the viewer follows.
func (c *ArticleController) FeedArticles(ctx context.Context, filter ArticleFeedFilter, viewerID int) (*ArticleList, error) {
	return c.repo.FeedArticles(ctx, filter, viewerID)
}

func (c *ArticleController) ArticleSubscribe(ctx context.Context, viewerID int) (<-chan Article, error) {
	in, err := c.sub.ArticleSubscribe(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan Article)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case a, ok := <-in:
				if !ok {
					return
				}
				if c.repo.ViewerFollowsUser(ctx, viewerID, a.Author.Username) {
					a.Author.Following = true
					out <- a
				}
			}
		}
	}()

	return out, nil
}
