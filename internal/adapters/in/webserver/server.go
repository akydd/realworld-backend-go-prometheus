package webserver

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Server wraps the HTTP router and manages the lifecycle of the web server.
type Server struct {
	port   string
	router http.Handler
}

// ServerHandlers is the port (in the hexagonal-architecture sense) that the web
// server uses to dispatch every HTTP request to an application handler.
type ServerHandlers interface {
	RegisterUser(http.ResponseWriter, *http.Request)
	LoginUser(http.ResponseWriter, *http.Request)
	GetUser(http.ResponseWriter, *http.Request)
	UpdateUser(http.ResponseWriter, *http.Request)
	GetProfile(http.ResponseWriter, *http.Request)
	FollowUser(http.ResponseWriter, *http.Request)
	UnfollowUser(http.ResponseWriter, *http.Request)
	CreateArticle(http.ResponseWriter, *http.Request)
	GetArticle(http.ResponseWriter, *http.Request)
	UpdateArticle(http.ResponseWriter, *http.Request)
	DeleteArticle(http.ResponseWriter, *http.Request)
	ListArticles(http.ResponseWriter, *http.Request)
	GetArticleFeed(http.ResponseWriter, *http.Request)
	FavoriteArticle(http.ResponseWriter, *http.Request)
	UnfavoriteArticle(http.ResponseWriter, *http.Request)
	CreateArticleComment(http.ResponseWriter, *http.Request)
	GetArticleComments(http.ResponseWriter, *http.Request)
	DeleteArticleComment(http.ResponseWriter, *http.Request)
	GetTags(http.ResponseWriter, *http.Request)
	HealthCheck(http.ResponseWriter, *http.Request)
}

// NewServer constructs a Server, wires all API routes to the supplied handlers,
// and applies authentication middleware to protected and optional-auth routes.
func NewServer(port string, h ServerHandlers, jwtSecret string) (*Server, error) {
	r := mux.NewRouter()
	r.HandleFunc("/api/users", h.RegisterUser).Methods("POST")
	r.HandleFunc("/api/users/login", h.LoginUser).Methods("POST")
	r.HandleFunc("/api/tags", h.GetTags).Methods("GET")
	r.HandleFunc("/api/healthcheck", h.HealthCheck).Methods("GET")

	protected := r.NewRoute().Subrouter()
	protected.Use(authMiddleware(jwtSecret))
	protected.HandleFunc("/api/user", h.GetUser).Methods("GET")
	protected.HandleFunc("/api/user", h.UpdateUser).Methods("PUT")
	protected.HandleFunc("/api/profiles/{username}/follow", h.FollowUser).Methods("POST")
	protected.HandleFunc("/api/profiles/{username}/follow", h.UnfollowUser).Methods("DELETE")
	protected.HandleFunc("/api/articles", h.CreateArticle).Methods("POST")
	protected.HandleFunc("/api/articles/feed", h.GetArticleFeed).Methods("GET")
	protected.HandleFunc("/api/articles/{slug}", h.UpdateArticle).Methods("PUT")
	protected.HandleFunc("/api/articles/{slug}", h.DeleteArticle).Methods("DELETE")
	protected.HandleFunc("/api/articles/{slug}/favorite", h.FavoriteArticle).Methods("POST")
	protected.HandleFunc("/api/articles/{slug}/favorite", h.UnfavoriteArticle).Methods("DELETE")
	protected.HandleFunc("/api/articles/{slug}/comments", h.CreateArticleComment).Methods("POST")
	protected.HandleFunc("/api/articles/{slug}/comments/{id}", h.DeleteArticleComment).Methods("DELETE")

	optionalAuth := r.NewRoute().Subrouter()
	optionalAuth.Use(optionalAuthMiddleware(jwtSecret))
	optionalAuth.HandleFunc("/api/profiles/{username}", h.GetProfile).Methods("GET")
	optionalAuth.HandleFunc("/api/articles", h.ListArticles).Methods("GET")
	optionalAuth.HandleFunc("/api/articles/{slug}", h.GetArticle).Methods("GET")
	optionalAuth.HandleFunc("/api/articles/{slug}/comments", h.GetArticleComments).Methods("GET")

	// logging!
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	return &Server{
		router: loggedRouter,
		port:   port,
	}, nil
}

// Start begins listening for HTTP connections on the configured port and blocks until the server exits.
func (s *Server) Start() {
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", s.port), s.router))
}

// Stop is a placeholder for graceful shutdown logic.
func (s *Server) Stop() {

}
