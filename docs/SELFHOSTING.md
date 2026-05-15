# Self-Hosting Reference

The primary deployment guide is the repository root [README.md](../README.md).
Chinese documentation is available in [README.zh-CN.md](../README.zh-CN.md).

Use this file as a short reference for operators who already understand the
deployment model.

## Minimal Path

1. Point DNS to the server:

   ```text
   ngrok.example.com A/AAAA <server-ip>
   *.example.com     A/AAAA <server-ip>
   ```

2. Issue a public wildcard certificate with DNS-01:

   ```text
   example.com
   *.example.com
   ```

3. Build and install:

   ```bash
   make release-all
   sudo sh deploy/install-systemd.sh
   ```

4. Edit `/etc/ngrok/ngrokd.env`:

   ```text
   NGROKD_DOMAIN=example.com
   NGROKD_TLS_CRT=/etc/ngrok/tls/fullchain.pem
   NGROKD_TLS_KEY=/etc/ngrok/tls/privkey.pem
   NGROKD_AUTH_TOKEN=<long-random-token>
   NGROKD_HTTP_ADDR=127.0.0.1:8080
   NGROKD_HTTPS_ADDR=
   NGROKD_TUNNEL_ADDR=0.0.0.0:4443
   ```

5. Configure Nginx to proxy `*.example.com` to `127.0.0.1:8080`.
   See `deploy/nginx/ngrok-http.conf`.

6. Start the server:

   ```bash
   sudo systemctl enable --now ngrokd
   journalctl -u ngrokd -f
   ```

7. Configure a client:

   ```yaml
   server_addr: ngrok.example.com:4443
   auth_token: <long-random-token>
   trust_host_root_certs: true

   tunnels:
     app:
       proto:
         http: 127.0.0.1:3000
       subdomain: app
   ```

8. Start the client:

   ```bash
   ./bin/ngrok -config=client.yml start-all
   ```

## Server Flags

```text
-domain              Primary base domain used by subdomain tunnels.
-domainCert          Additional domain certificate mapping. Repeatable.
-tlsCrt              Default TLS certificate file.
-tlsKey              Default TLS private key file.
-httpAddr            Public HTTP tunnel listener. Empty disables it.
-httpsAddr           Public HTTPS tunnel listener. Empty disables it.
-tunnelAddr          Client control/proxy listener.
-authToken           Required client token.
-maxConnections      Max accepted public/tunnel connections. Default: 1024.
-allowRemotePorts    Allow clients to request fixed TCP remote ports. Default: false.
```

## Nginx Models

Use one of these models when Nginx owns `:80` and `:443`.

### Nginx Terminates HTTPS

Use this for the simplest deployment:

```text
nginx :80/:443 -> ngrokd 127.0.0.1:8080
```

Set:

```text
NGROKD_HTTP_ADDR=127.0.0.1:8080
NGROKD_HTTPS_ADDR=
```

Reload Nginx after browser-facing certificate renewals.

### HTTPS Passthrough

Use this when ngrokd must terminate HTTPS tunnel traffic and choose certificates
by SNI:

```text
nginx stream :443 -> ngrokd 127.0.0.1:8443
```

Set:

```text
NGROKD_HTTPS_ADDR=127.0.0.1:8443
```

See `deploy/nginx/ngrok-stream.conf`.

## Certificates

Use public CA certificates when possible and set clients to:

```yaml
trust_host_root_certs: true
```

Wildcard domains require DNS-01 validation. The DNS provider is not fixed; use
an ACME client that supports your provider, such as `acme.sh`, `lego`, or
`certbot` with a DNS plugin.

ngrokd loads certificates at startup and can reload certificate/key files for
new TLS handshakes when the loaded certificate approaches expiration. Existing
connections continue.

## Multi-Domain Behavior

Additional domains use repeated `-domainCert`.

`subdomain` registers only under the primary `-domain`. Use explicit
`hostname` for other domains:

```yaml
tunnels:
  app-net:
    proto:
      http: 127.0.0.1:3000
    hostname: app.example.net
```

The server rejects hostnames outside configured domains.

## Related Documents

- [README.md](../README.md): main deployment guide.
- [README.zh-CN.md](../README.zh-CN.md): Chinese deployment guide.
- [MIGRATION.md](../MIGRATION.md): migration from archived ngrok v1.
- [SECURITY.md](../SECURITY.md): security policy and reporting.
