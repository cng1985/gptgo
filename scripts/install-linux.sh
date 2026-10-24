#!/bin/sh
set -eu

APP_NAME="${APP_NAME:-chatbi}"
SRC_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
BIN="$SRC_DIR/chatbi"
PREFIX="/opt/$APP_NAME"
DATA_DIR="/var/lib/$APP_NAME"
UNIT="/etc/systemd/system/$APP_NAME.service"

if [ "$(id -u)" -ne 0 ]; then
  echo "需要管理员权限" >&2
  exit 1
fi
if [ ! -f "$BIN" ]; then
  echo "未找到 $BIN" >&2
  exit 1
fi
arch=$(uname -m)
case "$arch" in
  x86_64|aarch64|arm64) ;;
  *) echo "未验证的架构 $arch" >&2; exit 1 ;;
esac

install -d -m 0755 "$PREFIX/configs" "$PREFIX/migrations" "$DATA_DIR"
if ! id "$APP_NAME" >/dev/null 2>&1; then
  useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$APP_NAME"
fi
install -m 0755 "$BIN" "$PREFIX/chatbi.new"
if [ -d "$SRC_DIR/configs" ]; then
  cp -an "$SRC_DIR/configs/." "$PREFIX/configs/" 2>/dev/null || cp -a "$SRC_DIR/configs/." "$PREFIX/configs/"
fi
if [ -d "$SRC_DIR/migrations" ]; then
  cp -a "$SRC_DIR/migrations/." "$PREFIX/migrations/"
fi
mv -f "$PREFIX/chatbi.new" "$PREFIX/chatbi"
chown -R "$APP_NAME:$APP_NAME" "$DATA_DIR"
chown -R root:root "$PREFIX"

cat > "$UNIT" <<EOF
[Unit]
Description=ChatBI AI
After=network.target

[Service]
Type=simple
User=$APP_NAME
Group=$APP_NAME
WorkingDirectory=$PREFIX
Environment=APP_PROFILE=pro
Environment=APP_DATA_DIR=$DATA_DIR
Environment=APP_BASE_DIR=$PREFIX
ExecStart=$PREFIX/chatbi --base-dir $PREFIX --data-dir $DATA_DIR --profile pro
Restart=on-failure
RestartSec=3
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
if systemctl is-active --quiet "$APP_NAME"; then
  systemctl restart "$APP_NAME"
else
  systemctl enable --now "$APP_NAME"
fi
systemctl --no-pager --full status "$APP_NAME" || true
echo "查看日志: journalctl -u $APP_NAME"
