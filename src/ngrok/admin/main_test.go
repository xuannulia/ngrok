package admin

import (
	"strings"
	"testing"
)

func TestNginxConfigIncludesAllDomains(t *testing.T) {
	cfg := defaultServerConfig()
	cfg.Domain = "example.com"
	cfg.HTTPAddr = "127.0.0.1:18080"

	got := nginxConfig(cfg, []nginxDomain{
		{Domain: "example.com", Cert: "/certs/example.com/fullchain.pem", Key: "/certs/example.com/privkey.pem"},
		{Domain: "example.net", Cert: "/certs/example.net/fullchain.pem", Key: "/certs/example.net/privkey.pem"},
	})

	for _, want := range []string{
		"server_name example.com *.example.com;",
		"server_name example.net *.example.net;",
		"ssl_certificate /certs/example.com/fullchain.pem;",
		"ssl_certificate /certs/example.net/fullchain.pem;",
		"proxy_pass http://127.0.0.1:18080;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("nginx config missing %q:\n%s", want, got)
		}
	}
	if count := strings.Count(got, "server_name "); count != 4 {
		t.Fatalf("expected 4 server_name lines, got %d:\n%s", count, got)
	}
}

func TestRemovePrimaryDomainPromotesRemainingDomain(t *testing.T) {
	cfg := defaultServerConfig()
	cfg.Domain = "one.example"
	cfg.ControlHost = "ngrok.one.example"
	cfg.TLSCrt = "/certs/one.example/fullchain.pem"
	cfg.TLSKey = "/certs/one.example/privkey.pem"
	cfg.AuthToken = "keep-token"
	cfg.HTTPAddr = "127.0.0.1:18080"
	cfg.TunnelAddr = "0.0.0.0:4443"
	cfg.VHost = "one.example,two.example"
	cfg.ExtraArgs = "-domainCert=two.example:/certs/two.example/fullchain.pem:/certs/two.example/privkey.pem"

	if err := removeDomainFromConfig(&cfg, "", "/certs", "one.example"); err != nil {
		t.Fatal(err)
	}

	if cfg.Domain != "two.example" {
		t.Fatalf("domain = %q, want two.example", cfg.Domain)
	}
	if cfg.ControlHost != "ngrok.two.example" {
		t.Fatalf("control host = %q, want ngrok.two.example", cfg.ControlHost)
	}
	if cfg.TLSCrt != "/certs/two.example/fullchain.pem" || cfg.TLSKey != "/certs/two.example/privkey.pem" {
		t.Fatalf("promoted cert paths = %q / %q", cfg.TLSCrt, cfg.TLSKey)
	}
	if cfg.AuthToken != "keep-token" || cfg.HTTPAddr != "127.0.0.1:18080" || cfg.TunnelAddr != "0.0.0.0:4443" {
		t.Fatalf("runtime settings were not preserved: %+v", cfg)
	}
	if strings.Contains(cfg.ExtraArgs, "two.example") {
		t.Fatalf("promoted domain cert arg was not removed: %q", cfg.ExtraArgs)
	}
}
