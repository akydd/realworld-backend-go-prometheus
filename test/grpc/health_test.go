//go:build integration

package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealth(t *testing.T) {
	conn := dial(t)
	health := grpc_health_v1.NewHealthClient(conn)

	resp, err := health.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("status: got %v, want SERVING", resp.GetStatus())
	}
}
