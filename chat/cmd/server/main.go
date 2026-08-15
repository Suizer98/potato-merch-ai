package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	chatv1 "github.com/Suizer98/potato-merch-ai/gen/go/chat/v1"
	storeagent "github.com/Suizer98/potato-merch-ai/internal/agent"
	"github.com/Suizer98/potato-merch-ai/internal/config"
	"github.com/Suizer98/potato-merch-ai/internal/server"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	llm, err := storeagent.NewLLM(ctx, cfg)
	if err != nil {
		log.Fatalf("llm: %v", err)
	}
	root, err := storeagent.NewRootAgent(ctx, cfg, llm)
	if err != nil {
		log.Fatalf("agent: %v", err)
	}
	chatServer, err := server.NewChatServer(root)
	if err != nil {
		log.Fatalf("chat server: %v", err)
	}

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcServer, chatServer)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(chatv1.ChatService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	go func() {
		log.Printf("gRPC listening on %s (provider=%s)", cfg.GRPCAddr, cfg.LLMProvider)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()
	go chatServer.ServeHTTP(cfg.HTTPAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Printf("shutting down")
	grpcServer.GracefulStop()
}
