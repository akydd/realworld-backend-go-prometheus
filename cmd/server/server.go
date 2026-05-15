package main

import (
	"flag"
	"log"
	"net"
	"os"

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
	userController := domain.New(database, os.Getenv("JWT_SECRET"))
	profileController := domain.NewProfileController(database)
	articleController := domain.NewArticleController(database)
	tagController := domain.NewTagController(database)
	commentController := domain.NewCommentController(database)
	handlers := webserver.NewHandler(userController, profileController, articleController, tagController, commentController)

	port := os.Getenv("SERVER_PORT")

	log.Printf("starting server on port %s...\n", port)

	s, err := webserver.NewServer(port, handlers, os.Getenv("JWT_SECRET"))
	if err != nil {
		log.Fatal(err)
	}

	// gRPC server
	grpcServer := grpc.NewServer()
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
