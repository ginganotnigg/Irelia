package cmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	api "irelia/api"
	feat "irelia/internal/features"
	repo "irelia/internal/repo"
	rb "irelia/pkg/rabbit/pkg"
	"irelia/pkg/database/client"
	"irelia/pkg/ent"
	"irelia/pkg/ent/migrate"
)

func customMetadataAnnotator(ctx context.Context, req *http.Request) metadata.MD {
	md := metadata.MD{}

	for name, values := range req.Header {
		lowerName := strings.ToLower(name)
		if strings.HasPrefix(lowerName, "x-") {
			md.Append(lowerName, values...)
		}
	}

	return md
}

func startGRPC(logger *zap.Logger) {
	dbconfig := client.ReadConfig()
	rbconfig := rb.ReadConfig()

    drv, err := client.Open("mysql_irelia", dbconfig)
    if err != nil {
        logger.Fatal("Failed to initialize Ent driver", zap.Error(err))
    }
	entClient := ent.NewClient(ent.Driver(drv))
	defer func() {
		if err := entClient.Close(); err != nil {
			logger.Fatal("can not close ent client", zap.Error(err))
		}
	}()

	if err = entClient.Schema.Create(context.Background(), migrate.WithDropIndex(true)); err != nil {
		logger.Fatal("can not init my database", zap.Error(err))
	}

	rabbitMQ := rb.New(rbconfig)
    repository := repo.New(entClient)

	// Start consuming messages from RabbitMQ
	// go rabbitMQ.Consume(context.Background(), repository.Interview.ReceiveScore)
	irelia := feat.New(repository, rabbitMQ, logger)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	api.RegisterIreliaServer(grpcServer, irelia)
	reflection.Register(grpcServer)

	grpcListener, err := net.Listen("tcp", viper.GetString("server.host")+":"+viper.GetString("server.port"))
	if err != nil {
		logger.Fatal("Failed to listen for gRPC server", zap.Error(err))
	}

	go func() {
		logger.Info("Starting gRPC server", zap.String("port", viper.GetString("server.port")))
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal("Failed to serve gRPC server", zap.Error(err))
		}
	}()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down gRPC server...")
	grpcServer.GracefulStop()
	logger.Info("gRPC server stopped")
}
