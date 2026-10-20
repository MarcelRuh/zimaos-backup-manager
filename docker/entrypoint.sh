#!/bin/sh
set -eu

DATA_DIR="${DATA_DIR:-/data}"

if [ "$(id -u)" = "0" ]; then
  if ! mkdir -p "$DATA_DIR" 2>/dev/null; then
    echo "error: cannot create data directory ${DATA_DIR} (check volume mount)" >&2
    exit 1
  fi
  # Named/bind volumes are often root-owned; reclaim for the non-root app user.
  chown -R zbm:zbm "$DATA_DIR" || {
    echo "error: cannot chown ${DATA_DIR} to zbm:zbm (check volume permissions)" >&2
    exit 1
  }
  exec su-exec zbm /usr/local/bin/zimaos-backup-manager "$@"
fi

exec /usr/local/bin/zimaos-backup-manager "$@"
