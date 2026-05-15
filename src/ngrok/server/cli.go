package server

import (
	"flag"
	"os"
)

type domainCertFlags []string

func (f *domainCertFlags) String() string {
	return ""
}

func (f *domainCertFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type Options struct {
	httpAddr         string
	httpsAddr        string
	tunnelAddr       string
	domain           string
	domainCerts      []string
	tlsCrt           string
	tlsKey           string
	logto            string
	loglevel         string
	authToken        string
	allowRemotePorts bool
	maxConnections   int
}

func parseArgs() *Options {
	var domainCerts domainCertFlags
	httpAddr := flag.String("httpAddr", ":80", "Public address for HTTP connections, empty string to disable")
	httpsAddr := flag.String("httpsAddr", ":443", "Public address listening for HTTPS connections, emptry string to disable")
	tunnelAddr := flag.String("tunnelAddr", ":4443", "Public address listening for ngrok client")
	domain := flag.String("domain", "ngrok.com", "Domain where the tunnels are hosted")
	flag.Var(&domainCerts, "domainCert", "Additional domain certificate mapping, repeatable, format: domain:/path/to/tls.crt:/path/to/tls.key")
	tlsCrt := flag.String("tlsCrt", "", "Path to a TLS certificate file")
	tlsKey := flag.String("tlsKey", "", "Path to a TLS key file")
	logto := flag.String("log", "stdout", "Write log messages to this file. 'stdout' and 'none' have special meanings")
	loglevel := flag.String("log-level", "INFO", "The level of messages to log. One of: DEBUG, INFO, WARNING, ERROR")
	authToken := flag.String("authToken", os.Getenv("NGROK_AUTH_TOKEN"), "Required client authentication token. Can also be set with NGROK_AUTH_TOKEN")
	allowRemotePorts := flag.Bool("allowRemotePorts", false, "Allow clients to request specific TCP remote ports")
	maxConnections := flag.Int("maxConnections", 1024, "Maximum concurrent accepted public/tunnel connections")
	flag.Parse()

	return &Options{
		httpAddr:         *httpAddr,
		httpsAddr:        *httpsAddr,
		tunnelAddr:       *tunnelAddr,
		domain:           *domain,
		domainCerts:      domainCerts,
		tlsCrt:           *tlsCrt,
		tlsKey:           *tlsKey,
		logto:            *logto,
		loglevel:         *loglevel,
		authToken:        *authToken,
		allowRemotePorts: *allowRemotePorts,
		maxConnections:   *maxConnections,
	}
}
