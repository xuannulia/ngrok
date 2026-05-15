# Modifications

This repository is a derivative work of ngrok v1. It modifies the original
codebase for self-hosted operation and public-network hardening.

Major changes include:

- Required server/client authentication token.
- TLS 1.2 minimum on server-side TLS listeners.
- Bounded ngrok protocol message size.
- Public connection limiting.
- Default denial of client-requested fixed TCP remote ports.
- HTTP inspection body capture limit.
- Protected local inspection UI when bound outside loopback.
- Multi-domain routing and SNI certificate selection.
- Certificate hot reload for renewed certificate/key files.
- `systemd`, `launchd`, and Nginx deployment templates.
- Replacement or removal of obsolete dependencies.
- Updated English and Chinese deployment documentation.

Most modified source files are under:

- `src/ngrok/client/`
- `src/ngrok/server/`
- `src/ngrok/conn/`
- `src/ngrok/msg/`
- `src/ngrok/proto/`
- `src/ngrok/util/`
- `deploy/`
- `docs/`

Detailed changes are available in the Git history.
