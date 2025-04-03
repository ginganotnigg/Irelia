package dbconfig

import (
    "database/sql"
    "fmt"

    "github.com/spf13/viper"
    "go.uber.org/zap"
    _ "github.com/go-sql-driver/mysql"
)

// DB is the global database connection instance
var DB *sql.DB

// InitDB initializes the database connection
func InitDB(logger *zap.Logger) {
    // Read the config.yaml
    viper.SetConfigFile("./config/config.yaml")
    if err := viper.ReadInConfig(); err != nil {
        logger.Fatal("Error reading config file", zap.Error(err))
    }

    // Enable environment variable usage
    viper.BindEnv("db.type", "DB_TYPE")
    viper.BindEnv("db.user", "DB_USER")
    viper.BindEnv("db.password", "DB_PASSWORD")
    viper.BindEnv("db.host", "DB_HOST")
    viper.BindEnv("db.port", "DB_PORT")
    viper.BindEnv("db.name", "DB_NAME")

    dbType := viper.GetString("db.type")
    dbUser := viper.GetString("db.user")
    dbPassword := viper.GetString("db.password")
    dbHost := viper.GetString("db.host")
    dbPort := viper.GetString("db.port")
    dbName := viper.GetString("db.name")

    // Create DSN (Data Source Name)
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
        dbUser, dbPassword, dbHost, dbPort, dbName)

    var err error
    DB, err = sql.Open(dbType, dsn)
    if err != nil {
        logger.Fatal("Error opening database", zap.Error(err))
    }

    // Test the database connection
    err = DB.Ping()
    if err != nil {
        logger.Fatal("Error connecting to database", zap.Error(err))
    }

    logger.Info("Database connected successfully")
}