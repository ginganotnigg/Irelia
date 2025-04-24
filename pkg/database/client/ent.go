package client

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/go-sql-driver/mysql"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"

	dbe "irelia/pkg/database/api"
)

// Open initializes a new Ent SQL driver from config.
func Open(name string, cfg *dbe.Database) (*entsql.Driver, error) {
	if cfg.AuthMethod == dbe.Database_AUTH_METHOD_AWS_IAM {
		pem, err := loadAwsRDSCAPem()
		if err != nil {
			return nil, err
		}
		rootCertPool := x509.NewCertPool()
		if !rootCertPool.AppendCertsFromPEM(pem) {
			return nil, errors.New("failed to append AWS RDS CA")
		}
		err = mysql.RegisterTLSConfig("rds", &tls.Config{
			RootCAs: rootCertPool,
		})
		if err != nil {
			return nil, err
		}
	}

	var (
		db *sql.DB
		err error
	)

	driver := NewDriver(cfg)
	sql.Register(name, driver)
	if cfg.TracingEnabled {
		sqltrace.Register(name, driver, sqltrace.WithServiceName(os.Getenv("DD_SERVICE")))
		db, err = sqltrace.Open(name, "", sqltrace.WithServiceName(os.Getenv("DD_SERVICE")))
		if err != nil {
			return nil, err
		}
	} else {
		db, err = sql.Open(name, "")
		if err != nil {
			return nil, err
		}
	}
	drv := entsql.OpenDB(dialect.MySQL, db)
	if cfg.GetMaxIdleConns() > 0 {
		drv.DB().SetMaxIdleConns(int(cfg.GetMaxIdleConns()))
	}
	if cfg.GetMaxOpenConns() > 0 {
		drv.DB().SetMaxOpenConns(int(cfg.GetMaxOpenConns()))
	}
	if cfg.GetConnMaxIdleTime() > 0 {
		drv.DB().SetConnMaxIdleTime(time.Duration(cfg.GetConnMaxIdleTime()) * time.Minute)
	}
	if cfg.GetConnMaxLifeTime() > 0 {
		drv.DB().SetConnMaxLifetime(time.Duration(cfg.GetConnMaxLifeTime()) * time.Minute)
	}
	return drv, nil
}

// loadAwsRDSCAPem fetches the AWS RDS global CA cert.
func loadAwsRDSCAPem() ([]byte, error) {
	resp, err := http.Get("https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}