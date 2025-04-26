package client

import (
	"database/sql/driver"
	"fmt"
	"time"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"

	db "irelia/pkg/database/api"
)

func ReadConfig() *db.Database {
	// Enable environment variable usage
	viper.BindEnv("db.user", "DB_USER")
	viper.BindEnv("db.password", "DB_PASSWORD")
	viper.BindEnv("db.host", "DB_HOST")
	viper.BindEnv("db.port", "DB_PORT")
	viper.BindEnv("db.name", "DB_NAME")
	viper.BindEnv("db.aws_region", "DB_AWS_REGION")
	viper.BindEnv("db.auth_method", "DB_AUTH_METHOD")

	// Map the auth_method string to the enum
	authMethod := mapAuthMethod(viper.GetString("db.auth_method"))

	return &db.Database{
		Username:       viper.GetString("db.user"),
		Password:       viper.GetString("db.password"),
		Host:           viper.GetString("db.host"),
		Port:           viper.GetUint32("db.port"),
		Name:           viper.GetString("db.name"),
		AwsRegion:      viper.GetString("db.aws_region"),
		AuthMethod:     authMethod,
		TracingEnabled: viper.GetBool("db.tracing_enabled"),
		MaxOpenConns:   viper.GetUint32("db.max_open_conns"),
		MaxIdleConns:   viper.GetUint32("db.max_idle_conns"),
	}
}

// mapAuthMethod maps a string to the db.Database_AuthMethod enum
func mapAuthMethod(authMethod string) db.Database_AuthMethod {
	switch authMethod {
	case "none":
		return db.Database_AUTH_METHOD_NONE
	case "username_password":
		return db.Database_AUTH_METHOD_USERNAME_PASSWORD
	case "aws_iam":
		return db.Database_AUTH_METHOD_AWS_IAM
	default:
		return db.Database_AUTH_METHOD_UNSPECIFIED
	}
}

// NewDriver creates a new custom driver with optional AWS IAM support
func NewDriver(config *db.Database) driver.Driver {
	drv := &Driver{config: config}
	if config.GetAuthMethod() == db.Database_AUTH_METHOD_AWS_IAM {
		drv.startRotation()
	}
	return drv
}

type Driver struct {
	drv    mysql.MySQLDriver
	config *db.Database
	token  string
}

func (d *Driver) Open(_ string) (driver.Conn, error) {
	dbEndpoint := fmt.Sprintf("%s:%d", d.config.GetHost(), d.config.GetPort())

	mysqlConfig := &mysql.Config{
		Addr:                    dbEndpoint,
		DBName:                  d.config.GetName(),
		AllowCleartextPasswords: true,
		AllowNativePasswords:    true,
		ParseTime:               true,
		User:                    d.config.GetUsername(),
	}

	if d.config.GetAuthMethod() == db.Database_AUTH_METHOD_AWS_IAM {
		mysqlConfig.Passwd = d.token
		mysqlConfig.TLSConfig = "rds"
	} else if d.config.GetAuthMethod() == db.Database_AUTH_METHOD_USERNAME_PASSWORD {
		mysqlConfig.Passwd = d.config.GetPassword()
	}

	return d.drv.Open(mysqlConfig.FormatDSN())
}

func formatDSN(config *db.Database, token string, withDB bool) string {
	dbEndpoint := fmt.Sprintf("%s:%d", config.GetHost(), config.GetPort())
    if withDB {
        dbEndpoint = fmt.Sprintf("%s/%s", dbEndpoint, config.GetName())
    }
	mysqlConfig := &mysql.Config{
		Addr:                    dbEndpoint,
		DBName:                  "",
		Net:                     "tcp",
		AllowCleartextPasswords: true,
		AllowNativePasswords:    true,
		ParseTime:               true,
		User:                    config.GetUsername(),
	}

	if config.GetAuthMethod() == db.Database_AUTH_METHOD_AWS_IAM {
		mysqlConfig.Passwd = token
		mysqlConfig.TLSConfig = "rds"
	} else if config.GetAuthMethod() == db.Database_AUTH_METHOD_USERNAME_PASSWORD {
		mysqlConfig.Passwd = config.GetPassword()
	}
	return mysqlConfig.FormatDSN()
}

func (d *Driver) startRotation() error {
	// Build the initial token
	if err := d.buildTokenWithRetry(5); err != nil {
		return fmt.Errorf("failed to build AWS IAM auth token: %w", err)
	}

	// Start token rotation
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			if err := d.buildTokenWithRetry(5); err != nil {
			}
		}
	}()
	return nil
}

func (d *Driver) buildTokenWithRetry(retries int) error {
	for retries > 0 {
		if err := d.buildToken(); err != nil {
			retries--
			time.Sleep(10 * time.Second)
			continue
		}
		return nil
	}
	return fmt.Errorf("exceeded retries for AWS IAM auth token build")
}

func (d *Driver) buildToken() error {
	// cfg, err := config.LoadDefaultConfig(context.TODO())
	// if err != nil {
	//     d.logger.Error("Failed to load AWS config", zap.Error(err))
	//     return err
	// }

	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()

	// token, err := auth.BuildAuthToken(
	//     ctx,
	//     fmt.Sprintf("%s:%d", d.config.GetHost(), d.config.GetPort()),
	//     d.config.GetAwsRegion(),
	//     d.config.GetUsername(),
	//     cfg.Credentials,
	// )
	// if err != nil {
	//     d.logger.Error("Failed to build AWS IAM auth token", zap.Error(err))
	//     return err
	// }

	// d.token = token
	return fmt.Errorf("AWS IAM auth token generation is disabled")
}
