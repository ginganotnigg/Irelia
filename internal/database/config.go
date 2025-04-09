package dbconfig

import (
	"database/sql"
	"fmt"
	"io/ioutil"
    "time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

    // Create DSN (Data Source Name) without specifying the database
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true", dbUser, dbPassword, dbHost, dbPort)

    var err error
    DB, err = sql.Open(dbType, dsn)
    if err != nil {
        logger.Fatal("Error opening database", zap.Error(err))
    }

    // Retry connecting to the database
    for i := 0; i < 10; i++ {
        err = DB.Ping()
        if err == nil {
            break
        }
        logger.Warn("Database not ready, retrying...", zap.Int("attempt", i+1))
        time.Sleep(5 * time.Second)
    }

    if err != nil {
        logger.Fatal("Error connecting to database", zap.Error(err))
    }

    logger.Info("Connected to MySQL server successfully")

    // Ensure the database exists
    ensureDatabaseExists(logger, dbName)

    // Update DSN to include the database name
    dsnWithDB := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbPort, dbName)
    DB, err = sql.Open(dbType, dsnWithDB)
    if err != nil {
        logger.Fatal("Error opening database with name", zap.Error(err))
    }

    // Test the database connection again
    err = DB.Ping()
    if err != nil {
        logger.Fatal("Error connecting to database with name", zap.Error(err))
    }

    logger.Info("Database connected successfully")

    // Run schema.sql files to create tables
    runSchemaSQL(logger)
}

// runSchemaSQL reads and executes the SQL files in the correct order
func runSchemaSQL(logger *zap.Logger) {
    schemaFiles := []string{
        "./internal/database/interviews.sql",
        "./internal/database/questions.sql",
    }

    for _, file := range schemaFiles {
        logger.Info("Processing schema file", zap.String("file", file))

        // Extract table name from filename
        tableName := extractTableName(file)
        logger.Info("Extracted table name", zap.String("table", tableName))

        // Check if table exists
        exists, err := tableExists(DB, tableName)
        if err != nil {
            logger.Fatal("Error checking if table exists", zap.String("table", tableName), zap.Error(err))
        }

        if exists {
            logger.Info("Table already exists, skipping", zap.String("table", tableName))
            continue // Skip to the next file
        }

        logger.Info("Executing schema file", zap.String("file", file))

        // Read the SQL file
        schema, err := ioutil.ReadFile(file)
        if err != nil {
            logger.Fatal("Error reading schema file", zap.String("file", file), zap.Error(err))
        }

        // Log the SQL content for debugging
        logger.Debug("SQL content", zap.String("file", file), zap.String("content", string(schema)))

        // Execute the SQL commands
        _, err = DB.Exec(string(schema))
        if err != nil {
            logger.Fatal("Error executing schema file", zap.String("file", file), zap.Error(err))
        }

        logger.Info("Executed schema file successfully", zap.String("file", file))
    }

    logger.Info("Database schema created successfully")
}

// ensureDatabaseExists checks if the database exists and creates it if it does not
func ensureDatabaseExists(logger *zap.Logger, dbName string) {
    query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)
    _, err := DB.Exec(query)
    if err != nil {
        logger.Fatal("Error creating database", zap.Error(err))
    }

    logger.Info("Database ensured successfully", zap.String("database", dbName))
}

// tableExists checks if a table exists in the database
func tableExists(db *sql.DB, tableName string) (bool, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s'", tableName)
	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// extractTableName extracts the table name from the file path
func extractTableName(filePath string) string {
	// Simple extraction based on the assumption that the filename is the table name
	// You might need a more robust approach if your naming conventions are different
	tableName := filePath[len("./internal/database/") : len(filePath)-len(".sql")]
	return tableName
}
