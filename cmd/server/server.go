package main

import (
	"flag"
	"log"
	"net"
	"os"

	pb "realworld-backend-go/api/proto/gen/pb"
	igrpc "realworld-backend-go/internal/adapters/in/grpc"
	"realworld-backend-go/internal/adapters/in/webserver"
	"realworld-backend-go/internal/adapters/out/db"
	"realworld-backend-go/internal/domain"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
)

func main() {
	// load configs
	envFile := flag.String("env", ".env", "path to env file")
	flag.Parse()
	if _, err := os.Stat(*envFile); err == nil {
		if err := godotenv.Load(*envFile); err != nil {
			log.Fatal(err)
		}
	}

	// Setup all dependencies
	database, err := db.New(&db.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
	})
	if err != nil {
		log.Fatal(err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	userController := domain.New(database, jwtSecret)
	profileController := domain.NewProfileController(database)
	articleController := domain.NewArticleController(database, database, database)
	tagController := domain.NewTagController(database)
	commentController := domain.NewCommentController(database, database)
	handlers := webserver.NewHandler(userController, profileController, articleController, tagController, commentController)

	port := os.Getenv("SERVER_PORT")

	log.Printf("starting server on port %s...\n", port)

	s, err := webserver.NewServer(port, handlers, jwtSecret)
	if err != nil {
		log.Fatal(err)
	}

	// gRPC server
	authMethods := map[string]igrpc.AuthRequirement{
		pb.UserService_GetUser_FullMethodName:              igrpc.MandatoryAuth,
		pb.UserService_UpdateUser_FullMethodName:           igrpc.MandatoryAuth,
		pb.ProfileService_GetProfile_FullMethodName:        igrpc.OptionalAuth,
		pb.ProfileService_FollowUser_FullMethodName:        igrpc.MandatoryAuth,
		pb.ProfileService_UnfollowUser_FullMethodName:      igrpc.MandatoryAuth,
		pb.ArticleService_CreateArticle_FullMethodName:     igrpc.MandatoryAuth,
		pb.ArticleService_GetArticleBySlug_FullMethodName:  igrpc.OptionalAuth,
		pb.ArticleService_UpdateArticle_FullMethodName:     igrpc.MandatoryAuth,
		pb.ArticleService_FavoriteArticle_FullMethodName:   igrpc.MandatoryAuth,
		pb.ArticleService_UnfavoriteArticle_FullMethodName: igrpc.MandatoryAuth,
		pb.ArticleService_DeleteArticle_FullMethodName:     igrpc.MandatoryAuth,
		pb.ArticleService_ListArticles_FullMethodName:      igrpc.OptionalAuth,
		pb.ArticleService_FeedArticles_FullMethodName:      igrpc.MandatoryAuth,
		pb.CommentService_CreateComment_FullMethodName:     igrpc.MandatoryAuth,
		pb.CommentService_GetComments_FullMethodName:       igrpc.OptionalAuth,
		pb.CommentService_DeleteComment_FullMethodName:     igrpc.MandatoryAuth,
	}
	streamAuthMethods := map[string]igrpc.AuthRequirement{
		pb.ArticleService_LiveArticleFeed_FullMethodName: igrpc.MandatoryAuth,
		pb.CommentService_LiveCommentFeed_FullMethodName: igrpc.OptionalAuth,
	}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(igrpc.AuthInterceptor(jwtSecret, authMethods)),
		grpc.StreamInterceptor(igrpc.StreamAuthInterceptor(jwtSecret, streamAuthMethods)),
	)
	userGrpcServer := igrpc.NewUserServer(userController)
	tagGrpcServer := igrpc.NewTagServer(tagController)
	profileGrpcServer := igrpc.NewProfileServer(profileController)
	commentGrpcServer := igrpc.NewCommentServer(commentController, database)
	articleGrpcServer := igrpc.NewArticleServer(articleController)
	healthServer := health.NewServer()
	igrpcServer := igrpc.NewGrpcServer(grpcServer, healthServer, userGrpcServer, tagGrpcServer, profileGrpcServer, commentGrpcServer, articleGrpcServer)

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		log.Fatal("GRPC_PORT environment variable is required")
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go func() {
		log.Printf("starting gRPC server on port %s...", grpcPort)
		if err := igrpcServer.Server.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	s.Start()
}
