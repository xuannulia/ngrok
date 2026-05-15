package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"ngrok/server/assets"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	certReloadBeforeExpiry = 24 * time.Hour
	certReloadRetryAfter   = time.Minute
)

type DomainTLSConfig struct {
	Domain  string
	CrtPath string
	KeyPath string
}

type certSource struct {
	sync.RWMutex
	crtPath     string
	keyPath     string
	defaultCrt  string
	defaultKey  string
	cert        *tls.Certificate
	crtModTime  time.Time
	keyModTime  time.Time
	nextCheck   time.Time
	assetBacked bool
	description string
}

func newCertSource(description string, crtPath string, keyPath string) *certSource {
	return &certSource{
		crtPath:     crtPath,
		keyPath:     keyPath,
		defaultCrt:  "assets/server/tls/snakeoil.crt",
		defaultKey:  "assets/server/tls/snakeoil.key",
		assetBacked: crtPath == "" || keyPath == "",
		description: description,
	}
}

func (s *certSource) fileOrAsset(path string, defaultPath string) ([]byte, error) {
	if path == "" {
		return assets.Asset(defaultPath)
	}
	return os.ReadFile(path)
}

func (s *certSource) statModTimes() (crtMod time.Time, keyMod time.Time, err error) {
	if s.assetBacked {
		return time.Time{}, time.Time{}, nil
	}

	crtInfo, err := os.Stat(s.crtPath)
	if err != nil {
		return
	}

	keyInfo, err := os.Stat(s.keyPath)
	if err != nil {
		return
	}

	return crtInfo.ModTime(), keyInfo.ModTime(), nil
}

func nextCertificateCheck(cert *tls.Certificate) time.Time {
	if cert == nil || len(cert.Certificate) == 0 {
		return time.Now().Add(certReloadRetryAfter)
	}

	leaf := cert.Leaf
	if leaf == nil {
		parsed, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return time.Now().Add(certReloadRetryAfter)
		}
		leaf = parsed
		cert.Leaf = parsed
	}

	next := leaf.NotAfter.Add(-certReloadBeforeExpiry)
	if next.Before(time.Now()) {
		return time.Now().Add(certReloadRetryAfter)
	}
	return next
}

func (s *certSource) Certificate() (*tls.Certificate, error) {
	now := time.Now()
	s.RLock()
	if s.cert != nil && now.Before(s.nextCheck) {
		cert := s.cert
		s.RUnlock()
		return cert, nil
	}
	s.RUnlock()

	crtMod, keyMod, err := s.statModTimes()
	if err != nil {
		s.RLock()
		cert := s.cert
		s.RUnlock()
		if cert != nil {
			return cert, nil
		}
		return nil, err
	}

	s.RLock()
	if s.cert != nil && (s.assetBacked || (crtMod.Equal(s.crtModTime) && keyMod.Equal(s.keyModTime))) {
		cert := s.cert
		s.RUnlock()
		return cert, nil
	}
	s.RUnlock()

	s.Lock()
	defer s.Unlock()

	crtMod, keyMod, err = s.statModTimes()
	if err != nil {
		if s.cert != nil {
			return s.cert, nil
		}
		return nil, err
	}
	if s.cert != nil && (s.assetBacked || (crtMod.Equal(s.crtModTime) && keyMod.Equal(s.keyModTime))) {
		s.nextCheck = nextCertificateCheck(s.cert)
		return s.cert, nil
	}

	crt, err := s.fileOrAsset(s.crtPath, s.defaultCrt)
	if err != nil {
		if s.cert != nil {
			return s.cert, nil
		}
		return nil, err
	}

	key, err := s.fileOrAsset(s.keyPath, s.defaultKey)
	if err != nil {
		if s.cert != nil {
			return s.cert, nil
		}
		return nil, err
	}

	cert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		if s.cert != nil {
			return s.cert, nil
		}
		return nil, err
	}

	s.cert = &cert
	s.crtModTime = crtMod
	s.keyModTime = keyMod
	s.nextCheck = nextCertificateCheck(s.cert)
	return s.cert, nil
}

type certReloader struct {
	defaultSource *certSource
	byName        map[string]*certSource
}

func (r *certReloader) certificateForName(name string) *certSource {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if source := r.byName[name]; source != nil {
		return source
	}
	if i := strings.IndexByte(name, '.'); i > 0 {
		if source := r.byName["*."+name[i+1:]]; source != nil {
			return source
		}
	}
	return r.defaultSource
}

func (r *certReloader) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	source := r.certificateForName(hello.ServerName)
	cert, err := source.Certificate()
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate for %s: %v", source.description, err)
	}
	return cert, nil
}

func LoadTLSConfig(defaultCrtPath string, defaultKeyPath string, domains []DomainTLSConfig) (tlsConfig *tls.Config, err error) {
	defaultSource := newCertSource("default", defaultCrtPath, defaultKeyPath)
	defaultCert, err := defaultSource.Certificate()
	if err != nil {
		return
	}

	reloader := &certReloader{
		defaultSource: defaultSource,
		byName:        make(map[string]*certSource),
	}
	for _, domain := range domains {
		source := newCertSource(domain.Domain, domain.CrtPath, domain.KeyPath)
		if _, err := source.Certificate(); err != nil {
			return nil, fmt.Errorf("failed to load certificate for %s: %v", domain.Domain, err)
		}
		domainName := strings.ToLower(domain.Domain)
		reloader.byName[domainName] = source
		reloader.byName["*."+domainName] = source
	}

	tlsConfig = &tls.Config{
		Certificates:   []tls.Certificate{*defaultCert},
		GetCertificate: reloader.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	return
}
