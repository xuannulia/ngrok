# Security Policy

## Supported Scope

This fork is maintained for self-hosted developer tunnel services and private
infrastructure.

Supported security scope:

- The current `master` branch.
- Server and client authentication behavior.
- TLS configuration and certificate reload behavior.
- Public HTTP/HTTPS/TCP tunnel routing.
- Local inspection UI access control.
- Deployment templates under `deploy/`.

Out of scope:

- The archived upstream ngrok v1 project.
- The commercial ngrok cloud service.
- Unsupported local patches not present in this repository.
- Compromised hosts, leaked DNS provider keys, or leaked `auth_token` values
  after they have left this repository.

## Reporting a Vulnerability

Do not open a public issue for a vulnerability that includes exploit details,
tokens, private keys, DNS API credentials, or live service endpoints.

Report privately to the repository maintainer using the private contact method
configured on the hosting platform. Include:

- Affected commit or release.
- Whether the issue affects server, client, deployment templates, or docs.
- Reproduction steps.
- Impact and preconditions.
- Logs or packet captures with secrets removed.
- Suggested fix, if available.

Expected handling:

- Triage the report.
- Confirm impact and supported scope.
- Prepare a fix or mitigation.
- Publish a security note after users have a reasonable path to update.

## Secret Handling

Treat these values as secrets:

- `NGROKD_AUTH_TOKEN`
- Client `auth_token`
- TLS private keys
- DNS provider API credentials used for DNS-01
- SSH credentials and deployment keys

If any secret is pasted into chat, logs, issues, pull requests, or terminal
history shared with others, rotate it. Removing the text later is not enough.

Recommended file permissions:

```text
/etc/ngrok/ngrokd.env       root:ngrok 0640
/etc/ngrok/client.yml       root:ngrok 0640
/etc/ngrok/tls/privkey.pem  root:ngrok 0640
```

Client config files should be readable only by the local user running the
client.

## Public Internet Deployment Baseline

Minimum recommended controls:

- Use a long random `auth_token`.
- Keep `-allowRemotePorts=false` unless every client is trusted.
- Keep `-maxConnections` set to a finite value.
- Bind the inspection UI to `127.0.0.1`, or set `inspect_auth`.
- Use publicly trusted TLS certificates where possible.
- Use DNS-01 automation credentials with the minimum permissions your provider
  supports.
- Put Nginx, firewall, and host-level rate limits in front of public services
  where appropriate.
- Keep server logs free of request bodies, tokens, and DNS provider secrets.

## Known Limitations

- Authentication is a shared-token model. Anyone with the token can register
  tunnels allowed by the server configuration.
- This fork does not implement per-user quotas, per-token domain policies, or
  a management API.
- Nginx-terminated HTTPS means browser TLS terminates at Nginx before traffic is
  proxied to the local ngrokd HTTP listener.
- The local inspection UI is a developer tool. Do not expose it publicly without
  authentication and network controls.
