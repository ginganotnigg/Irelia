package redis

import (
	"crypto/tls"
	"crypto/x509"

	pb "irelia/pkg/redis/api"
)

func NewTLS(config *pb.TLS) (*tls.Config, error) {
	if config == nil || !config.GetEnabled() {
		return nil, nil
	}

	var (
		certificate tls.Certificate
		caCertPool  *x509.CertPool
		err         error
	)

	// Define TLS configuration
	if len(config.GetCert()) > 0 && len(config.GetKey()) > 0 {
		certificate, err = tls.X509KeyPair([]byte(config.GetCert()), []byte(config.GetKey()))
		if err != nil {
			return nil, err
		}
	}

	if len(config.GetCa()) > 0 {
		caCertPool = x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM([]byte(config.GetCa())); !ok {
			return nil, err
		}
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		RootCAs:            caCertPool,
		InsecureSkipVerify: config.GetInsecureSkipVerify(),
	}, nil
}