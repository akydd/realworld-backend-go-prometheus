package grpc

import (
	"realworld-backend-go/api/proto/gen/pb"

	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GrpcServer struct {
	Server *ggrpc.Server
}

func NewGrpcServer(server *ggrpc.Server, userServer pb.UserServiceServer, tagServer pb.TagServiceServer, profileServer pb.ProfileServiceServer, commentServer pb.CommentServiceServer, articleServer pb.ArticleServiceServer) *GrpcServer {
	pb.RegisterUserServiceServer(server, userServer)
	pb.RegisterTagServiceServer(server, tagServer)
	pb.RegisterArticleServiceServer(server, articleServer)
	pb.RegisterProfileServiceServer(server, profileServer)
	pb.RegisterCommentServiceServer(server, commentServer)

	reflection.Register(server)

	return &GrpcServer{
		Server: server,
	}
}
