//go:build integration

package grpc_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"realworld-backend-go/api/proto/gen/pb"
)

func grpcAddr() string {
	if h := os.Getenv("GRPC_HOST"); h != "" {
		return h
	}
	return "localhost:8098"
}

func dial(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(grpcAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial %s: %v", grpcAddr(), err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func withToken(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Token "+token))
}

func genUID() string {
	return fmt.Sprintf("%d%04d", time.Now().Unix(), time.Now().Nanosecond()%10000)
}

func nullableStr(s string) *pb.NullableString {
	return &pb.NullableString{Value: s}
}

func clearNullable() *pb.NullableString {
	return &pb.NullableString{}
}

func strPtr(s string) *string { return &s }
