package server

import (
	"crypto/subtle"
	"crypto/tls"
	"fmt"
	"math/rand"
	"ngrok/conn"
	log "ngrok/log"
	"ngrok/msg"
	"ngrok/util"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

const (
	registryCacheSize uint64        = 1024 * 1024 // 1 MB
	connReadTimeout   time.Duration = 10 * time.Second
)

// GLOBALS
var (
	tunnelRegistry  *TunnelRegistry
	controlRegistry *ControlRegistry

	// XXX: kill these global variables - they're only used in tunnel.go for constructing forwarding URLs
	opts      *Options
	listeners map[string]*conn.Listener
	connSlots chan struct{}
	domains   []string
)

func acquireConnSlot(c conn.Conn) bool {
	if connSlots == nil {
		return true
	}

	select {
	case connSlots <- struct{}{}:
		return true
	default:
		c.Warn("Connection limit reached, closing %s", c.RemoteAddr())
		c.Close()
		return false
	}
}

func parseDomainTLSConfigs(values []string) ([]DomainTLSConfig, error) {
	configs := make([]DomainTLSConfig, 0, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid domainCert %q, expected domain:crt:key", value)
		}
		domain := strings.ToLower(strings.TrimSpace(parts[0]))
		if !validHostname(domain) {
			return nil, fmt.Errorf("invalid domainCert domain %q", domain)
		}
		configs = append(configs, DomainTLSConfig{
			Domain:  domain,
			CrtPath: parts[1],
			KeyPath: parts[2],
		})
	}
	return configs, nil
}

func configuredDomains(defaultDomain string, domainCerts []DomainTLSConfig) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(domainCerts)+1)
	add := func(domain string) {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" || seen[domain] {
			return
		}
		seen[domain] = true
		result = append(result, domain)
	}
	add(defaultDomain)
	for _, cert := range domainCerts {
		add(cert.Domain)
	}
	return result
}

func releaseConnSlot() {
	if connSlots != nil {
		<-connSlots
	}
}

func validateAuth(authMsg *msg.Auth) error {
	if opts.authToken == "" {
		return fmt.Errorf("server authentication token is not configured")
	}
	if subtle.ConstantTimeCompare([]byte(authMsg.User), []byte(opts.authToken)) != 1 {
		return fmt.Errorf("invalid authentication token")
	}
	return nil
}

func NewProxy(pxyConn conn.Conn, regPxy *msg.RegProxy) {
	// fail gracefully if the proxy connection fails to register
	defer func() {
		if r := recover(); r != nil {
			pxyConn.Warn("Failed with error: %v", r)
			pxyConn.Close()
		}
	}()

	// set logging prefix
	pxyConn.SetType("pxy")

	// look up the control connection for this proxy
	pxyConn.Info("Registering new proxy for %s", regPxy.ClientId)
	ctl := controlRegistry.Get(regPxy.ClientId)

	if ctl == nil {
		panic("No client found for identifier: " + regPxy.ClientId)
	}

	ctl.RegisterProxy(pxyConn)
}

// Listen for incoming control and proxy connections
// We listen for incoming control and proxy connections on the same port
// for ease of deployment. The hope is that by running on port 443, using
// TLS and running all connections over the same port, we can bust through
// restrictive firewalls.
func tunnelListener(addr string, tlsConfig *tls.Config) {
	// listen for incoming connections
	listener, err := conn.Listen(addr, "tun", tlsConfig)
	if err != nil {
		panic(err)
	}

	log.Info("Listening for control and proxy connections on %s", listener.Addr.String())
	for c := range listener.Conns {
		if !acquireConnSlot(c) {
			continue
		}
		go func(tunnelConn conn.Conn) {
			defer releaseConnSlot()
			// don't crash on panics
			defer func() {
				if r := recover(); r != nil {
					tunnelConn.Info("tunnelListener failed with error %v: %s", r, debug.Stack())
				}
			}()

			tunnelConn.SetReadDeadline(time.Now().Add(connReadTimeout))
			var rawMsg msg.Message
			if rawMsg, err = msg.ReadMsg(tunnelConn); err != nil {
				tunnelConn.Warn("Failed to read message: %v", err)
				tunnelConn.Close()
				return
			}

			// don't timeout after the initial read, tunnel heartbeating will kill
			// dead connections
			tunnelConn.SetReadDeadline(time.Time{})

			switch m := rawMsg.(type) {
			case *msg.Auth:
				NewControl(tunnelConn, m)

			case *msg.RegProxy:
				NewProxy(tunnelConn, m)

			default:
				tunnelConn.Close()
			}
		}(c)
	}
}

func Main() {
	// parse options
	opts = parseArgs()

	// init logging
	log.LogTo(opts.logto, opts.loglevel)
	if opts.maxConnections <= 0 {
		panic("maxConnections must be positive")
	}
	connSlots = make(chan struct{}, opts.maxConnections)

	// seed random number generator
	seed, err := util.RandomSeed()
	if err != nil {
		panic(err)
	}
	rand.Seed(seed)

	// init tunnel/control registry
	registryCacheFile := os.Getenv("REGISTRY_CACHE_FILE")
	tunnelRegistry = NewTunnelRegistry(registryCacheSize, registryCacheFile)
	controlRegistry = NewControlRegistry()

	// start listeners
	listeners = make(map[string]*conn.Listener)

	// load tls configuration
	domainTLSConfigs, err := parseDomainTLSConfigs(opts.domainCerts)
	if err != nil {
		panic(err)
	}
	domains = configuredDomains(opts.domain, domainTLSConfigs)
	tlsConfig, err := LoadTLSConfig(opts.tlsCrt, opts.tlsKey, domainTLSConfigs)
	if err != nil {
		panic(err)
	}

	// listen for http
	if opts.httpAddr != "" {
		listeners["http"] = startHttpListener(opts.httpAddr, nil)
	}

	// listen for https
	if opts.httpsAddr != "" {
		listeners["https"] = startHttpListener(opts.httpsAddr, tlsConfig)
	}

	// ngrok clients
	tunnelListener(opts.tunnelAddr, tlsConfig)
}
