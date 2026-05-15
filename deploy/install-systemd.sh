#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PREFIX=${PREFIX:-/usr/local}
SYSCONFDIR=${SYSCONFDIR:-/etc/ngrok}
UNITDIR=${UNITDIR:-/etc/systemd/system}
USER_NAME=${NGROK_USER:-ngrok}
GROUP_NAME=${NGROK_GROUP:-ngrok}

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2
  exit 1
fi

if ! getent group "$GROUP_NAME" >/dev/null 2>&1; then
  groupadd --system "$GROUP_NAME"
fi

if ! id "$USER_NAME" >/dev/null 2>&1; then
  useradd --system --gid "$GROUP_NAME" --home-dir /var/lib/ngrok --shell /usr/sbin/nologin "$USER_NAME"
fi

install -d -o "$USER_NAME" -g "$GROUP_NAME" -m 0750 /var/lib/ngrok
install -d -o root -g "$GROUP_NAME" -m 0750 "$SYSCONFDIR"
install -d -o root -g "$GROUP_NAME" -m 0750 "$SYSCONFDIR/tls"
install -d -m 0755 "$PREFIX/bin"

install -m 0755 "$ROOT_DIR/bin/ngrokd" "$PREFIX/bin/ngrokd"
install -m 0755 "$ROOT_DIR/bin/ngrok" "$PREFIX/bin/ngrok"

if [ ! -f "$SYSCONFDIR/ngrokd.env" ]; then
  install -o root -g "$GROUP_NAME" -m 0640 "$ROOT_DIR/deploy/systemd/ngrokd.env.example" "$SYSCONFDIR/ngrokd.env"
fi

if [ ! -f "$SYSCONFDIR/client.yml" ]; then
  install -o root -g "$GROUP_NAME" -m 0640 "$ROOT_DIR/deploy/systemd/client.yml.example" "$SYSCONFDIR/client.yml"
fi

install -m 0644 "$ROOT_DIR/deploy/systemd/ngrokd.service" "$UNITDIR/ngrokd.service"
install -m 0644 "$ROOT_DIR/deploy/systemd/ngrokd-privileged-ports.service" "$UNITDIR/ngrokd-privileged-ports.service"
install -m 0644 "$ROOT_DIR/deploy/systemd/ngrok-client@.service" "$UNITDIR/ngrok-client@.service"

systemctl daemon-reload

cat <<EOF
Installed ngrok services.

Edit:
  $SYSCONFDIR/ngrokd.env
  $SYSCONFDIR/client.yml

Start server:
  systemctl enable --now ngrokd

Start client tunnels from $SYSCONFDIR/client.yml:
  systemctl enable --now ngrok-client@client
EOF
