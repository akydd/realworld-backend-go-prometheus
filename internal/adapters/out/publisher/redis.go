package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"realworld-backend-go/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	redisClient        *redis.Client
	articleChannelName string
	commentChannelName string
}

type RedisConfig struct {
	Password           string
	Host               string
	Port               string
	ArticleChannelName string
	CommentChannelName string
}

func validateConfig(c *RedisConfig) error {
	if c.Password == "" {
		return errors.New("redis config must contain a password")
	}

	if c.Host == "" {
		return errors.New("redis config must contain a host")
	}

	if c.Port == "" {
		return errors.New("redis config must contain a port")
	}

	if c.ArticleChannelName == "" {
		return errors.New("redis config must contain a name for the article channel")
	}

	if c.CommentChannelName == "" {
		return errors.New("redis config must contain a name for the comment channel")
	}

	return nil
}

func New(c *RedisConfig) (*Client, error) {
	if err := validateConfig(c); err != nil {
		return nil, err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", c.Host, c.Port),
		Password: c.Password,
		DB:       0,
	})

	return &Client{
		redisClient:        redisClient,
		articleChannelName: c.ArticleChannelName,
		commentChannelName: c.CommentChannelName,
	}, nil
}

type article struct {
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	TagList        []string  `json:"tagList"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Favorited      bool      `json:"favorited"`
	FavoritesCount int       `json:"favoritesCount"`
	Author         author    `json:"author"`
}

func marshalArticle(a *domain.Article) (string, error) {
	author := author(a.Author)

	art := &article{
		Slug:           a.Slug,
		Title:          a.Title,
		Description:    a.Description,
		TagList:        a.TagList,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
		Favorited:      a.Favorited,
		FavoritesCount: a.FavoritesCount,
		Author:         author,
	}

	jsonData, err := json.Marshal(art)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func unmarshalArticle(msg *redis.Message) (*domain.Article, error) {
	var a article

	err := json.Unmarshal([]byte(msg.Payload), &a)
	if err != nil {
		return nil, err
	}

	return &domain.Article{
		Slug:           a.Slug,
		Title:          a.Title,
		Description:    a.Description,
		TagList:        a.TagList,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
		Favorited:      a.Favorited,
		FavoritesCount: a.FavoritesCount,
		Author: domain.Profile{
			Username:  a.Author.Username,
			Bio:       a.Author.Bio,
			Image:     a.Author.Image,
			Following: a.Author.Following,
		},
	}, nil
}

func (c *Client) PublishArticle(ctx context.Context, a *domain.Article) error {
	jsonString, err := marshalArticle(a)
	if err != nil {
		return err
	}

	return c.redisClient.Publish(ctx, c.articleChannelName, jsonString).Err()
}

type author struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

type comment struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Body      string    `json:"body"`
	Author    author    `json:"author"`
}

func marshalComment(c *domain.Comment) (string, error) {
	cmt := &comment{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Body:      c.Body,
		Author:    author(c.Author),
	}
	jsonData, err := json.Marshal(cmt)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func unmarshalComment(msg *redis.Message) (*domain.Comment, error) {
	var c comment
	if err := json.Unmarshal([]byte(msg.Payload), &c); err != nil {
		return nil, err
	}
	return &domain.Comment{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Body:      c.Body,
		Author: domain.Profile{
			Username:  c.Author.Username,
			Bio:       c.Author.Bio,
			Image:     c.Author.Image,
			Following: c.Author.Following,
		},
	}, nil
}

func (c *Client) PublishComment(ctx context.Context, slug string, cmt *domain.Comment) error {
	jsonString, err := marshalComment(cmt)
	if err != nil {
		return err
	}
	return c.redisClient.Publish(ctx, c.commentChannelName+":"+slug, jsonString).Err()
}

func (c *Client) CommentSubscribe(ctx context.Context, slug string) <-chan domain.Comment {
	j := c.redisClient.Subscribe(ctx, c.commentChannelName+":"+slug)
	in := j.Channel()
	out := make(chan domain.Comment)
	go func() {
		defer close(out)
		for msg := range in {
			cmt, err := unmarshalComment(msg)
			if err != nil {
				continue
			}
			out <- *cmt
		}
	}()
	return out
}

func (c *Client) ArticleSubscribe(ctx context.Context) <-chan domain.Article {
	j := c.redisClient.Subscribe(ctx, c.articleChannelName)
	in := j.Channel()

	out := make(chan domain.Article)

	go func() {
		defer close(out)

		for msg := range in {
			article, err := unmarshalArticle(msg)
			if err != nil {
				continue
			}

			out <- *article
		}
	}()

	return out
}
