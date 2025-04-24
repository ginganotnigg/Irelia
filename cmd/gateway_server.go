package cmd

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
    "github.com/spf13/viper"
    "go.uber.org/zap"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    api "irelia/api"
)

// Middleware to enable CORS
func enableCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        // Handle preflight requests
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func startGateway(logger *zap.Logger) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    mux := runtime.NewServeMux(
        runtime.WithMetadata(customMetadataAnnotator),
    )
    opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
    err := api.RegisterIreliaHandlerFromEndpoint(
        ctx,
        mux,
        fmt.Sprintf("localhost:%s", viper.GetString("server.port")),
        opts,
    )
    if err != nil {
        logger.Fatal("Failed to register gateway handler", zap.Error(err))
    }

    // Wrap the mux with the CORS middleware
    corsHandler := enableCORS(mux)

    httpServer := &http.Server{
        Addr:    fmt.Sprintf(":%s", viper.GetString("server.gwport")),
        Handler: corsHandler, // Use the CORS-enabled handler
    }

    go func() {
        logger.Info("Starting HTTP gateway server", zap.String("port", viper.GetString("server.gwport")))
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("Failed to serve HTTP gateway", zap.Error(err))
        }
    }()

    // Handle graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    logger.Info("Shutting down HTTP gateway server...")

    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer shutdownCancel()

    if err := httpServer.Shutdown(shutdownCtx); err != nil {
        logger.Error("HTTP server shutdown error", zap.Error(err))
    }

    logger.Info("HTTP gateway server stopped")
}