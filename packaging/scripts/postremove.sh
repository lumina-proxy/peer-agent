#!/bin/sh
set -e
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
  systemctl daemon-reload >/dev/null 2>&1 || true
fi
if [ "${1:-}" = "purge" ]; then
  rm -f /etc/luminaproxy/peer-agent.env
  rmdir /etc/luminaproxy 2>/dev/null || true
fi
