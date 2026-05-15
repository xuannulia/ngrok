# Migration Guide

This guide covers migration from the archived ngrok v1 codebase to this
self-hosted fork.

## Compatibility Summary

The basic v1 operating model is unchanged:

- Run `ngrokd` on a server.
- Configure clients with `server_addr`.
- Start named tunnels from a YAML config.
- Use `subdomain` or explicit `hostname` for HTTP tunnels.

This fork changes defaults and deployment behavior for safer self-hosting.

## Required Server Authentication

Original ngrok v1 allowed self-hosted deployments without a required shared
server token. This fork requires a token.

Server:

```bash
./bin/ngrokd \
  -domain=example.com \
  -authToken=<long-random-token>
```

Or:

```bash
NGROK_AUTH_TOKEN=<long-random-token> ./bin/ngrokd -domain=example.com
```

Client:

```yaml
server_addr: ngrok.example.com:4443
auth_token: <long-random-token>
trust_host_root_certs: true
```

## Public CA Certificates

For public CA certificates, clients should use:

```yaml
trust_host_root_certs: true
```

With this mode, clients do not need to be rebuilt when the server certificate
renews under the same public CA trust chain.

Self-signed CA deployments still require clients to trust the signing CA.

## Certificate Reload

This fork can reload certificate/key files for new TLS handshakes.

Behavior:

- The certificate is loaded at startup.
- The in-memory certificate is reused normally.
- When the loaded certificate is close to expiration, the server checks whether
  the certificate/key files changed.
- If reload succeeds, new TLS handshakes use the new certificate.
- Existing connections continue.
- If reload fails, the previous in-memory certificate remains in use.

Nginx still needs reload when Nginx terminates browser HTTPS.

## Multiple Domains

Original v1 deployments commonly assumed one base domain. This fork supports
additional domains with repeated `-domainCert`:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example.com/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example.com/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/example.net/fullchain.pem:/etc/ngrok/tls/example.net/privkey.pem \
  -authToken=<long-random-token>
```

`subdomain` is intentionally scoped to the primary `-domain` only:

```yaml
tunnels:
  app:
    proto:
      http: 127.0.0.1:3000
    subdomain: app
```

Result:

```text
app.example.com
```

To use another configured domain, request an explicit hostname:

```yaml
tunnels:
  app-net:
    proto:
      http: 127.0.0.1:3000
    hostname: app.example.net
```

The server rejects explicit hostnames outside the configured domains.

## TCP Remote Ports

Client-requested fixed TCP remote ports are disabled by default.

Old behavior:

```bash
ngrok -proto=tcp -remote-addr=:2222 22
```

New behavior:

- Random TCP ports still work.
- Fixed remote ports require the server flag:

```bash
-allowRemotePorts=true
```

Enable this only for trusted clients.

## Nginx Coexistence

If Nginx already owns `:80` and `:443`, do not bind ngrokd directly to those
ports.

Use loopback listeners:

```text
NGROKD_HTTP_ADDR=127.0.0.1:8080
NGROKD_HTTPS_ADDR=
NGROKD_TUNNEL_ADDR=0.0.0.0:4443
```

Then proxy wildcard HTTP/HTTPS hosts through Nginx as shown in `README.md`.

If ngrokd must terminate HTTPS tunnel traffic directly, use Nginx `stream` with
SNI passthrough and route only the tunnel domains to `127.0.0.1:8443`.

## Inspection UI

The inspection UI should remain on loopback:

```yaml
inspect_addr: 127.0.0.1:4040
```

If binding to a non-loopback address, configure basic auth:

```yaml
inspect_addr: 0.0.0.0:4040
inspect_auth: user:password
```

Without `inspect_auth`, non-loopback inspection binding is rejected.

## Build And Dependencies

The project still uses GOPATH layout. The Makefile handles the expected build
environment:

```bash
make all
make release-all
```

Notable dependency/build changes:

- Legacy dynamic dependency restoration was removed.
- Historical GOPATH dependencies are vendored under `src/github.com`.
- Static assets use Go `embed`; `go-bindata` is no longer required.
- Several obsolete libraries were replaced or removed.
- The remaining external dependencies are intentionally small.

## Deployment Files

New deployment templates:

```text
deploy/systemd/
deploy/launchd/
deploy/nginx/
```

Linux server install:

```bash
make release-all
sudo sh deploy/install-systemd.sh
sudo editor /etc/ngrok/ngrokd.env
sudo systemctl enable --now ngrokd
```

## Checklist

- Generate a long random auth token.
- Configure server `-authToken` or `NGROK_AUTH_TOKEN`.
- Add matching client `auth_token`.
- Use `trust_host_root_certs: true` with public CA certificates.
- Add wildcard DNS records for tunnel domains.
- Configure DNS-01 certificate automation.
- Decide whether Nginx terminates HTTPS or passes HTTPS through by SNI.
- Keep inspection UI on loopback or protect it with `inspect_auth`.
- Keep `-allowRemotePorts=false` unless fixed TCP ports are required.
