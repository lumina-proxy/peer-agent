#!/bin/sh
set -e
case "${1:-}" in
  remove|purge|0)
    if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
      systemctl disable --now peer-agent >/dev/null 2>&1 || true
    fi
    ;;
esac
