# ngrok v1 self-hosted fork

This repository is a self-hosted maintenance fork of ngrok v1. The original v1
codebase is archived. This fork keeps the small, direct ngrok v1 workflow and
adds the pieces that matter when you run it on the public internet today:
client authentication, safer TLS defaults, connection limits, multi-domain
support, certificate reloads, and service templates.

It is meant for personal use, team tunnel services, and private infrastructure.
It is not trying to be a commercial tunnel platform.

[Chinese README](README.zh-CN.md)

## What Changed

- The server and client must both use an authentication token.
- Server TLS listeners require TLS 1.2 or newer.
- ngrok protocol messages have a size limit.
- `-maxConnections` limits accepted public and tunnel connections.
- Clients cannot request fixed public TCP ports by default.
- HTTP inspection stores at most 1 MB of request/response body data.
- The inspection UI requires authentication when exposed outside loopback.
- One server can handle multiple base domains.
- Repeated `-domainCert` flags can provide SNI certificates for extra domains.
- Renewed certificate files can be picked up by new TLS handshakes.
- `systemd`, `launchd`, and Nginx templates are included.
- Several obsolete dependencies were removed or replaced.

## Documentation

- [Migration guide](MIGRATION.md)
- [Security policy](SECURITY.md)
- [Self-hosting reference](docs/SELFHOSTING.md)
- [Third-party notices](THIRD_PARTY_NOTICES.md)

## Build

The project still uses the original GOPATH layout. Build it with the Makefile:

```bash
make all
make release-all
```

The binaries are written to:

```text
bin/ngrok
bin/ngrokd
```

## Web Admin

`ngrok-admin` provides the first-run setup panel:

```bash
sudo ./bin/ngrok-admin
```

It listens on `127.0.0.1:9090` by default and prints a setup key in the
terminal. Use that key to create the admin account.

The panel can write `ngrokd.env`, generate a client config, issue DNS-01
certificates through `acme.sh`, write an Nginx config, and start or restart the
`ngrokd` service.

## Minimal Deployment Flow

This is the smallest useful deployment behind Nginx: Nginx owns ports 80 and
443, while `ngrokd` handles the client control connection and the local HTTP
tunnel listener.

1. Create DNS records:

   ```text
   ngrok.example.com A/AAAA <server-ip>
   *.example.com     A/AAAA <server-ip>
   ```

2. Issue a public wildcard certificate with DNS-01. It should cover:

   ```text
   example.com
   *.example.com
   ```

3. Build and install the server:

   ```bash
   make release-all
   sudo sh deploy/install-systemd.sh
   ```

4. Edit `/etc/ngrok/ngrokd.env`:

   ```text
   NGROKD_DOMAIN=example.com
   NGROKD_CONTROL_HOST=ngrok.example.com
   NGROKD_TLS_CRT=/etc/ngrok/tls/fullchain.pem
   NGROKD_TLS_KEY=/etc/ngrok/tls/privkey.pem
   NGROKD_AUTH_TOKEN=<long-random-token>
   NGROKD_HTTP_ADDR=127.0.0.1:8080
   NGROKD_HTTPS_ADDR=
   NGROKD_TUNNEL_ADDR=0.0.0.0:4443
   ```

5. Configure Nginx to proxy `*.example.com` to `127.0.0.1:8080`.
   Use the example later in this README or start from
   `deploy/nginx/ngrok-http.conf`.

6. Start the server:

   ```bash
   sudo systemctl enable --now ngrokd
   sudo systemctl status ngrokd
   ```

7. Create a client config:

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

The tunnel should be reachable at:

```text
http://app.example.com
https://app.example.com
```

## Server

Minimal server command:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -authToken=change-me-long-random-token
```

You can also pass the token through the environment:

```bash
NGROK_AUTH_TOKEN=change-me-long-random-token ./bin/ngrokd -domain=example.com
```

Do not put a public server online with an empty token, a weak token, or a token
that has already been shared around.

Useful server flags:

```text
-domain              Primary base domain. Subdomain tunnels are created under it.
-domainCert          Certificate mapping for an extra domain. Repeatable.
-tlsCrt              Default TLS certificate file.
-tlsKey              Default TLS private key file.
-httpAddr            Public HTTP tunnel listener. Empty disables it.
-httpsAddr           Public HTTPS tunnel listener. Empty disables it.
-tunnelAddr          Client control and proxy listener.
-authToken           Token required from clients.
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

`subdomain: app` only uses the server's primary `-domain`:

```text
app.example.com
```

To use another domain that is configured on the server, request the full
`hostname`:

```yaml
tunnels:
  app-example-net:
    proto:
      http: 127.0.0.1:3000
    hostname: app.example.net
```

The server rejects hostnames outside the configured domain suffixes.

## Multiple Domains

One `ngrokd` process can serve several base domains:

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/example.net/fullchain.pem:/etc/ngrok/tls/example.net/privkey.pem \
  -domainCert=example.org:/etc/ngrok/tls/example.org/fullchain.pem:/etc/ngrok/tls/example.org/privkey.pem \
  -authToken=change-me-long-random-token
```

DNS should include at least:

```text
*.example.com     A/AAAA <server-ip>
*.example.net     A/AAAA <server-ip>
*.example.org     A/AAAA <server-ip>
ngrok.example.com A/AAAA <server-ip>
```

Root-domain records are only needed if you also want to use the root names as
tunnel hosts.

## Certificates

Use publicly trusted certificates when you can. Then clients can use:

```yaml
trust_host_root_certs: true
```

With public CA certificates, clients do not need to be rebuilt after normal
certificate renewal.

### Wildcard Certificates With DNS-01

Wildcard certificates require DNS-01 validation. Your DNS provider only needs an
API that your ACME client can update.

Common ACME clients:

- `acme.sh`
- `lego`
- `certbot` with a DNS plugin

Generic flow:

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

After renewal, install the certificate and key where the service user can read
them, with suitable file permissions. Reload Nginx only when Nginx terminates
browser HTTPS.

If `ngrokd` terminates TLS for `-tunnelAddr` or `-httpsAddr`, normal renewal
does not require a restart. `ngrokd` keeps the current certificate in memory and
checks the certificate/key files when the loaded certificate is near expiration.
After a successful reload, new TLS handshakes use the renewed files. Existing
connections continue.

If Nginx terminates browser HTTPS, reload Nginx after installing the renewed
files. `ngrokd` certificate reload still applies to the client control listener.

## Using Nginx

If Nginx already owns ports 80 and 443 on the server, keep `ngrokd` off those
ports. There are two common setups.

### Option A: Nginx Terminates HTTPS

This is the usual setup. Nginx handles browser HTTPS and proxies requests to the
HTTP listener exposed by `ngrokd`.

`ngrokd`:

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

Use this option when you want Nginx to manage all public web TLS and keep
`ngrokd` away from port 443.

### Option B: HTTPS Passthrough To ngrokd

Use this when `ngrokd` should terminate HTTPS tunnel traffic itself and choose
certificates by SNI. Nginx only passes the TCP connection through.

`ngrokd`:

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

The `default` backend should point to your normal HTTPS service, so tunnel
domains do not interfere with unrelated sites.

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

macOS templates:

```text
deploy/launchd/com.example.ngrokd.plist
deploy/launchd/com.example.ngrok-client.plist
```

## Example Deployment

Assume this target setup:

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

The expected hosts are:

```text
http://app.example.com
https://app.example.com
http://api.example.net
https://api.example.net
http://admin.example.org
https://admin.example.org
```

## Security Notes

- Use a long random `auth_token`.
- Keep client config files readable only by the user running the client.
- Keep server environment files readable only by root and the service user.
- Keep the inspection UI on `127.0.0.1`; if it must be exposed elsewhere, set
  `inspect_auth`.
- Do not enable `-allowRemotePorts` for untrusted clients.
- Put normal firewall and rate-limit controls in front of public entry points.
- Rotate DNS API credentials if they ever appear in chat, logs, or issue
  trackers.

## License

This fork is licensed under the Apache License 2.0. See [LICENSE](LICENSE).

This repository includes code derived from ngrok v1 and third-party components.
See [NOTICE](NOTICE), [MODIFICATIONS.md](MODIFICATIONS.md), and
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
