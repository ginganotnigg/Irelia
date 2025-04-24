package cmd

import (
    "log"
    api "irelia/pkg/logger/api"
    "irelia/pkg/logger/pkg/logging"
    "context"

    "github.com/joho/godotenv"
    "github.com/spf13/viper"
)


func Execute() {
	err := godotenv.Load()
    if err != nil {
        log.Fatalf("Error loading .env file")
    }
    viper.SetConfigFile("./config/config.yaml")
    if err := viper.ReadInConfig(); err != nil {
        log.Fatalf("Error reading config file")
    }
    // Initialize the customized logger
    loggerConfig := &api.Logger{
        Pretty: viper.GetBool("logger.pretty"),
        Level:  api.Logger_Level(viper.GetInt("logger.level")),
    }
    if err := logging.InitLogger(loggerConfig); err != nil {
        log.Fatalf("Failed to initialize logger: %v", err)
    }
    logger := logging.Logger(context.TODO())

	go startGRPC(logger)
	startGateway(logger)
}