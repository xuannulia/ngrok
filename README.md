# ngrok v1 self-hosted fork

Maintained self-hosted fork of the archived ngrok v1 codebase.

This fork keeps the simple ngrok v1 workflow and updates the parts that matter
for running it on today's public internet: authentication, dependency cleanup,
multi-domain routing, certificate reload, service templates, and safer defaults.

This is for developer-operated tunnel services and private infrastructure. It is
not a commercial tunnel platform.

[Chinese README](README.zh-CN.md)

## Status

Implemented in this fork:

- Required server/client authentication token.
- TLS 1.2 minimum on server TLS listeners.
- Bounded ngrok protocol message size.
- Public connection limit with `-maxConnections`.
- Custom TCP remote ports disabled by default.
- HTTP inspection body capture capped at 1 MB.
- Local inspection UI protection when exposed outside loopback.
- Multi-domain routing.
- SNI certificate selection with repeated `-domainCert`.
- Certificate hot reload for renewed certificate/key files.
- `systemd`, `launchd`, and Nginx deployment templates.
- Reduced dependency surface; obsolete dependencies were replaced or removed.

## Documentation

- [Migration guide](MIGRATION.md)
- [Security policy](SECURITY.md)
- [Self-hosting reference](docs/SELFHOSTING.md)
- [Third-party notices](THIRD_PARTY_NOTICES.md)

## Build

This project still uses the original GOPATH layout. Use the Makefile.

```bash
make all
make release-all
```

Binaries:

```text
bin/ngrok
bin/ngrokd
```

## Minimal Deployment Flow

Follow this path for the smallest production-like deployment behind Nginx:

1. Create DNS records:

   ```text
   ngrok.example.com A/AAAA <server-ip>
   *.example.com     A/AAAA <server-ip>
   ```

2. Issue a public wildcard certificate with DNS-01:

   ```text
   example.com
   *.example.com
   ```

3. Build and install on the server:

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
   Use the Nginx example in this README or `deploy/nginx/ngrok-http.conf`.

6. Start the server:

   ```bash
   sudo systemctl enable --now ngrokd
   sudo systemctl status ngrokd
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

Expected result:

```text
http://app.example.com
https://app.example.com
```

## Server

Minimal server:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -authToken=change-me-long-random-token
```

The token may also be supplied through the environment:

```bash
NGROK_AUTH_TOKEN=change-me-long-random-token ./bin/ngrokd -domain=example.com
```

Do not run a public server with an empty, weak, or shared-by-accident token.

Useful server flags:

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

## Client

Example client config:

```yaml
server_addr: ngrok.example.com:4443
auth_token: change-me-long-random-token
trust_host_root_certs: true
inspect_addr: 127.0.0.1:4040

tunnels:
  app:
    proto:
      http: 127.0.0.1:3000
    subdomain: app
```

Start all configured tunnels:

```bash
./bin/ngrok -config=client.yml start-all
```

`subdomain: app` is scoped to the server primary `-domain` only:

```text
app.example.com
```

To use another configured domain, request an explicit hostname:

```yaml
tunnels:
  app-example-net:
    proto:
      http: 127.0.0.1:3000
    hostname: app.example.net
```

The server rejects explicit hostnames outside the configured domains.

## Multiple Domains

Run one server with several base domains:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/example.net/fullchain.pem:/etc/ngrok/tls/example.net/privkey.pem \
  -domainCert=example.org:/etc/ngrok/tls/example.org/fullchain.pem:/etc/ngrok/tls/example.org/privkey.pem \
  -authToken=change-me-long-random-token
```

DNS requirements:

```text
*.example.com   A/AAAA  <server-ip>
*.example.net   A/AAAA  <server-ip>
*.example.org   A/AAAA  <server-ip>
ngrok.example.com A/AAAA <server-ip>
```

Root-domain records are optional unless you also want tunnels on the root name.

## Certificates

Use publicly trusted certificates when possible. Then clients can use:

```yaml
trust_host_root_certs: true
```

Clients do not need to be rebuilt when a normal public CA certificate renews.

### Wildcard Certificates With DNS-01

Wildcard certificates require DNS-01 validation. The DNS provider does not need
to be special; it only needs API support that your ACME client can update.

Common ACME clients:

- `acme.sh`
- `lego`
- `certbot` with a DNS plugin

Generic renewal flow:

```bash
# acme.sh. Replace dns_provider with the DNS plugin you use.
export DNS_PROVIDER_API_KEY=...
acme.sh --issue --dns dns_provider -d example.com -d '*.example.com'
acme.sh --install-cert -d example.com \
  --fullchain-file /etc/ngrok/tls/fullchain.pem \
  --key-file /etc/ngrok/tls/privkey.pem \
  --reloadcmd 'systemctl reload nginx'

# lego. Replace provider and credentials with your DNS provider plugin.
DNS_PROVIDER_API_KEY=... lego \
  --dns provider \
  --domains example.com \
  --domains '*.example.com' \
  --path /etc/ngrok/acme \
  run

# certbot. Replace dns-provider with the installed DNS plugin name.
certbot certonly \
  --authenticator dns-provider \
  -d example.com \
  -d '*.example.com'
```

Install renewed files with owner and mode suitable for your service user. Reload
Nginx only when Nginx terminates browser HTTPS.

If `ngrokd` terminates TLS for `-tunnelAddr` or `-httpsAddr`, it does not need a
restart for normal renewals. It keeps the current certificate in memory and only
checks the certificate/key files when the loaded certificate is near expiration.
New TLS handshakes use the renewed files after reload succeeds. Existing
connections continue.

If Nginx terminates browser HTTPS, reload Nginx after certificate installation.
`ngrokd` hot reload still applies to the client control listener when that
listener uses the same certificate files.

## Nginx Coexistence

Use Nginx when the server already hosts other sites on ports 80 and 443.

### Option A: Nginx Terminates HTTPS

This is simple and works well when browser HTTPS traffic can be terminated by
Nginx and proxied to ngrokd's HTTP listener.

Run ngrokd:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -httpAddr=127.0.0.1:8080 \
  -httpsAddr= \
  -tunnelAddr=0.0.0.0:4443 \
  -authToken=change-me-long-random-token
```

Nginx:

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    "" close;
}

server {
    listen 80;
    server_name *.example.com;

    client_max_body_size 0;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_http_version 1.1;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:8080;
    }
}

server {
    listen 443 ssl;
    http2 on;
    server_name *.example.com;

    ssl_certificate /etc/ngrok/tls/fullchain.pem;
    ssl_certificate_key /etc/ngrok/tls/privkey.pem;

    client_max_body_size 0;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_http_version 1.1;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:8080;
    }
}
```

Use this model if you want Nginx to own all public web TLS and keep ngrokd away
from port 443.

### Option B: HTTPS Passthrough To ngrokd

Use this when ngrokd must terminate HTTPS tunnel traffic itself and choose
certificates by SNI.

Run ngrokd:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example.com/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example.com/privkey.pem \
  -httpsAddr=127.0.0.1:8443 \
  -tunnelAddr=0.0.0.0:4443 \
  -authToken=change-me-long-random-token
```

Nginx `stream {}`:

```nginx
map $ssl_preread_server_name $backend {
    ~(^|\.)example\.com$ 127.0.0.1:8443;
    default 127.0.0.1:9443;
}

server {
    listen 443;
    proxy_pass $backend;
    ssl_preread on;
}
```

The `default` backend should point to your normal HTTPS stack. This prevents
ngrok tunnel domains from taking over unrelated sites.

Templates:

```text
deploy/nginx/ngrok-http.conf
deploy/nginx/ngrok-stream.conf
```

## Running As A Service

Linux server:

```bash
make release-all
sudo sh deploy/install-systemd.sh
sudo editor /etc/ngrok/ngrokd.env
sudo systemctl enable --now ngrokd
sudo systemctl status ngrokd
```

Logs:

```bash
journalctl -u ngrokd -f
```

Linux client:

```bash
sudo cp deploy/systemd/client.yml.example /etc/ngrok/client.yml
sudo editor /etc/ngrok/client.yml
sudo systemctl enable --now ngrok-client@client
```

macOS examples:

```text
deploy/launchd/com.example.ngrokd.plist
deploy/launchd/com.example.ngrok-client.plist
```

## Example Deployment

Goal:

```text
Primary domain: example.com
Extra domains:  example.net, example.org
Control server: ngrok.example.com:4443
Tunnel hosts:   app.example.com, api.example.net, admin.example.org
Nginx owns:     80 and 443
ngrokd HTTP:    127.0.0.1:8080
ngrokd control: 0.0.0.0:4443
```

DNS:

```text
ngrok.example.com A     203.0.113.10
*.example.com     A     203.0.113.10
*.example.net     A     203.0.113.10
*.example.org     A     203.0.113.10
```

Server:

```bash
NGROK_AUTH_TOKEN=change-me-long-random-token ./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/fullchain.pem:/etc/ngrok/tls/privkey.pem \
  -domainCert=example.org:/etc/ngrok/tls/fullchain.pem:/etc/ngrok/tls/privkey.pem \
  -httpAddr=127.0.0.1:8080 \
  -httpsAddr= \
  -tunnelAddr=0.0.0.0:4443
```

Client:

```yaml
server_addr: ngrok.example.com:4443
auth_token: change-me-long-random-token
trust_host_root_certs: true

tunnels:
  web:
    proto:
      http: 127.0.0.1:3000
    subdomain: app

  api:
    proto:
      http: 127.0.0.1:4000
    hostname: api.example.net

  admin:
    proto:
      http: 127.0.0.1:5000
    hostname: admin.example.org
```

Results:

```text
http://app.example.com
https://app.example.com      # if Nginx terminates HTTPS for wildcard hosts
http://api.example.net
https://api.example.net
http://admin.example.org
https://admin.example.org
```

## Security Notes

- Use a long random `auth_token`.
- Keep client config files readable only by the user running the client.
- Keep server env files readable only by root and the service user.
- Keep the inspection UI on `127.0.0.1`, or set `inspect_auth`.
- Do not enable `-allowRemotePorts` for untrusted clients.
- Put normal firewall and rate-limit controls in front of public services.
- Rotate DNS API credentials if they were ever pasted into chat, logs, or issue
  trackers.

## License

This fork is licensed under the Apache License 2.0. See [LICENSE](LICENSE).

This repository includes code derived from ngrok v1 and third-party components.
See [NOTICE](NOTICE), [MODIFICATIONS.md](MODIFICATIONS.md), and
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
