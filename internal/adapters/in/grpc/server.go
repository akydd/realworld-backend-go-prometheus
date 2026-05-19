package grpc

import (
	"realworld-backend-go/api/proto/gen/pb"

	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type GrpcServer struct {
	Server *ggrpc.Server
}

func NewGrpcServer(server *ggrpc.Server, healthServer grpc_health_v1.HealthServer, userServer pb.UserServiceServer, tagServer pb.TagServiceServer, profileServer pb.ProfileServiceServer, commentServer pb.CommentServiceServer, articleServer pb.ArticleServiceServer) *GrpcServer {
	pb.RegisterUserServiceServer(server, userServer)
	pb.RegisterTagServiceServer(server, tagServer)
	pb.RegisterArticleServiceServer(server, articleServer)
	pb.RegisterProfileServiceServer(server, profileServer)
	pb.RegisterCommentServiceServer(server, commentServer)
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	reflection.Register(server)

	return &GrpcServer{
		Server: server,
	}
}
