package cmd

import (
    "log"
    "go.uber.org/zap"
    "github.com/joho/godotenv"

)


func Execute() {
	err := godotenv.Load()
    if err != nil {
        log.Fatalf("Error loading .env file")
    }
    logger, err := zap.NewProduction()
    if err != nil {
            log.Fatalf("Failed to initialize logger: %v", err)
    }
    defer logger.Sync()
	go startGRPC(logger)
	startGateway(logger)
}