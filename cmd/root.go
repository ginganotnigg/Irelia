package cmd

import (
    "log"
    "context"
    "flag"
    "github.com/joho/godotenv"
    "github.com/spf13/viper"
    
    api "irelia/pkg/logger/api"
    "irelia/pkg/logger/pkg"
)


func Execute() {
    configPath := flag.String("c", "config.yaml", "Path to config file")
    flag.Parse()

    _ = godotenv.Load()
    viper.AutomaticEnv()
    
	viper.BindEnv("db.host", "DB_HOST")
    viper.BindEnv("db.port", "DB_PORT")
    viper.BindEnv("db.user", "DB_USER")
    viper.BindEnv("db.password", "DB_PASSWORD")
    viper.BindEnv("db.name", "DB_NAME")
    viper.BindEnv("db.aws_region", "DB_AWS_REGION")
    viper.BindEnv("db.auth_method", "DB_AUTH_METHOD")
    viper.BindEnv("rabbitmq.username", "RABBITMQ_USERNAME")
    viper.BindEnv("rabbitmq.password", "RABBITMQ_PASSWORD")

    viper.SetConfigFile(*configPath)
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