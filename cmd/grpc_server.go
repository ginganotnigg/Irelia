package cmd

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	//"google.golang.org/grpc/credentials/insecure"
	api "irelia/api"
	"irelia/internal/handler"
	repo "irelia/internal/repo"
	dbcf "irelia/internal/database"
	"irelia/internal/service"
)

func startGRPC(logger *zap.Logger) {
	dbcf.InitDB(logger)
    db := dbcf.DB
    defer db.Close()

	// Create repository
	repo := repo.NewSQLInterviewRepository(db)

	// Set up gRPC connections to Darius and Karma
	// dariusConn, err := grpc.Dial(viper.GetString("darius.url"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	// if err != nil {
	//     logger.Fatal("Failed to connect to Darius service", zap.Error(err))
	// }
	// defer dariusConn.Close()

	// Initialize clients
	dariusClient := service.NewDariusHTTPClient()
	karmaClient := service.NewKarmaHTTPClient()

	// Create a combined service implementation that delegates to appropriate implementations
	ireliaService := handler.NewIreliaService(dariusClient, karmaClient, *repo, logger)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	api.RegisterInterviewServiceServer(grpcServer, ireliaService)
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
