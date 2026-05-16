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
	articleController := domain.NewArticleController(database)
	tagController := domain.NewTagController(database)
	commentController := domain.NewCommentController(database)
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
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(igrpc.AuthInterceptor(jwtSecret, authMethods)))
	userGrpcServer := igrpc.NewUserServer(userController)
	tagGrpcServer := igrpc.NewTagServer(tagController)
	profileGrpcServer := igrpc.NewProfileServer(profileController)
	commentGrpcServer := igrpc.NewCommentServer(commentController)
	articleGrpcServer := igrpc.NewArticleServer(articleController)
	igrpcServer := igrpc.NewGrpcServer(grpcServer, userGrpcServer, tagGrpcServer, profileGrpcServer, commentGrpcServer, articleGrpcServer)

	lis, err := net.Listen("tcp", ":8099")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go func() {
		log.Printf("starting gRPC server on port 8099...")
		if err := igrpcServer.Server.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	s.Start()
}
