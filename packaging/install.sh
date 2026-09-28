#!/bin/sh
set -eu

BASE="${LUMINA_DOWNLOADS:-https://luminaproxy.com/downloads}"
TOKEN="${DEVICE_TOKEN:-}"
VERSION="${LUMINA_VERSION:-}"
ACTION=install
SERVICE=1
LABEL=com.luminaproxy.peer-agent

say() { printf '%s\n' "$*"; }
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --token) [ $# -ge 2 ] || fail "--token needs a value"; TOKEN="$2"; shift 2 ;;
    --token=*) TOKEN="${1#*=}"; shift ;;
    --version) [ $# -ge 2 ] || fail "--version needs a value"; VERSION="$2"; shift 2 ;;
    --version=*) VERSION="${1#*=}"; shift ;;
    --uninstall) ACTION=uninstall; shift ;;
    --no-service) SERVICE=0; shift ;;
    *) fail "unknown option: $1" ;;
  esac
done

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) fail "unsupported system $(uname -s); on Windows use install.ps1, or run the Docker image" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) fail "unsupported CPU $(uname -m)" ;;
esac

SUDO=""
if [ "$OS" = linux ] && [ "$(id -u)" -ne 0 ]; then
  command -v sudo >/dev/null 2>&1 || fail "run this as root, or install sudo"
  SUDO=sudo
fi

if [ "$OS" = linux ]; then
  BIN=/usr/local/bin/peer-agent
  CONF_DIR=/etc/luminaproxy
  ENV_FILE=$CONF_DIR/peer-agent.env
  UNIT=/etc/systemd/system/peer-agent.service
else
  APP_DIR="$HOME/.luminaproxy"
  BIN="$APP_DIR/bin/peer-agent"
  LOG="$APP_DIR/agent.log"
  PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
fi

has_systemd() { command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; }

uninstall() {
  if [ "$OS" = linux ]; then
    if has_systemd; then
      $SUDO systemctl disable --now peer-agent >/dev/null 2>&1 || true
      $SUDO rm -f "$UNIT"
      $SUDO systemctl daemon-reload >/dev/null 2>&1 || true
    fi
    $SUDO rm -f "$BIN" "$ENV_FILE"
  else
    launchctl bootout "gui/$(id -u)/$LABEL" >/dev/null 2>&1 || launchctl unload "$PLIST" >/dev/null 2>&1 || true
    rm -f "$PLIST"
    rm -rf "$APP_DIR"
  fi
  say "LuminaProxy peer agent removed."
}

if [ "$ACTION" = uninstall ]; then
  uninstall
  exit 0
fi

[ -n "$TOKEN" ] || fail "missing device token; get one in Dashboard -> Earn -> Add device, then run with --token lpk_..."
case "$TOKEN" in
  lpk_*) ;;
  *) fail "that does not look like a device token (tokens start with lpk_)" ;;
esac
case "$TOKEN" in
  *[!A-Za-z0-9_]*) fail "the device token contains unexpected characters" ;;
esac

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  else shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "$BASE/VERSION" | tr -d '[:space:]')" || fail "could not reach $BASE"
fi
VERSION="${VERSION#v}"
case "$VERSION" in
  ''|*[!0-9A-Za-z.-]*) fail "could not determine the agent version" ;;
esac

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
FILE="peer-agent_${VERSION}_${OS}_${ARCH}.tar.gz"

say "Downloading peer agent $VERSION for $OS/$ARCH ..."
curl -fsSL "$BASE/v$VERSION/$FILE" -o "$TMP/$FILE" || fail "download failed from $BASE/v$VERSION/$FILE"
curl -fsSL "$BASE/v$VERSION/checksums.txt" -o "$TMP/checksums.txt" || fail "could not fetch checksums"
EXPECTED="$(awk -v f="$FILE" '$2 == f { print $1 }' "$TMP/checksums.txt")"
[ -n "$EXPECTED" ] || fail "no checksum published for $FILE"
[ "$EXPECTED" = "$(sha256 "$TMP/$FILE")" ] || fail "checksum mismatch for $FILE; aborting"
tar -xzf "$TMP/$FILE" -C "$TMP" peer-agent

if [ "$OS" = linux ]; then
  $SUDO install -d -m 0755 "$(dirname "$BIN")" "$CONF_DIR"
  $SUDO install -m 0755 "$TMP/peer-agent" "$BIN"
  printf 'DEVICE_TOKEN=%s\nLUMINA_PEER_CONSENT=yes\nLOG_LEVEL=info\n' "$TOKEN" > "$TMP/peer-agent.env"
  $SUDO install -m 0600 "$TMP/peer-agent.env" "$ENV_FILE"

  if [ "$SERVICE" -eq 1 ] && has_systemd; then
    cat > "$TMP/peer-agent.service" <<UNIT
[Unit]
Description=LuminaProxy peer agent (opt-in bandwidth sharing)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$ENV_FILE
ExecStart=$BIN
Restart=always
RestartSec=5
DynamicUser=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
MemoryMax=128M

[Install]
WantedBy=multi-user.target
UNIT
    $SUDO install -m 0644 "$TMP/peer-agent.service" "$UNIT"
    $SUDO systemctl daemon-reload
    $SUDO systemctl enable peer-agent >/dev/null 2>&1
    $SUDO systemctl restart peer-agent
    sleep 2
    if $SUDO systemctl is-active --quiet peer-agent; then
      say "LuminaProxy peer agent is installed and running (systemd service peer-agent)."
      say "Status: sudo systemctl status peer-agent   Logs: sudo journalctl -u peer-agent -f"
    else
      fail "the service did not start; check: sudo journalctl -u peer-agent -n 50"
    fi
  else
    say "Installed $BIN. This system has no systemd, so start it yourself:"
    say "  set -a; . $ENV_FILE; set +a; $BIN"
  fi
else
  mkdir -p "$(dirname "$BIN")" "$(dirname "$PLIST")"
  install -m 0755 "$TMP/peer-agent" "$BIN"
  xattr -d com.apple.quarantine "$BIN" >/dev/null 2>&1 || true

  if [ "$SERVICE" -eq 1 ]; then
    umask 077
    cat > "$PLIST" <<PLISTXML
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>$BIN</string>
    <string>--i-consent</string>
    <string>--log-file</string>
    <string>$LOG</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>DEVICE_TOKEN</key>
    <string>$TOKEN</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ProcessType</key>
  <string>Background</string>
</dict>
</plist>
PLISTXML
    launchctl bootout "gui/$(id -u)/$LABEL" >/dev/null 2>&1 || true
    launchctl bootstrap "gui/$(id -u)" "$PLIST" >/dev/null 2>&1 || launchctl load -w "$PLIST"
    say "LuminaProxy peer agent is installed and running. It starts automatically when you log in."
    say "Logs: $LOG"
  else
    say "Installed $BIN. Start it with: DEVICE_TOKEN=$TOKEN $BIN --i-consent"
  fi
fi

say "Your device will show as online in Dashboard -> Earn within a minute."
say "To remove it later: curl -fsSL $BASE/install.sh | sh -s -- --uninstall"
