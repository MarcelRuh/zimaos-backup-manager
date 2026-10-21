#!/bin/sh
set -eu

DATA_DIR="${DATA_DIR:-/data}"
DOCKER_SOCK="${DOCKER_SOCK:-/var/run/docker.sock}"

# Match the host docker.sock GID so a non-root user can talk to the engine
# (socket is typically root:docker mode 660). Used only when we drop privileges.
ensure_dockersock_access() {
  sock="$1"
  if [ ! -S "$sock" ]; then
    echo "warn: docker socket not found at ${sock} (compose discovery needs the bind mount)" >&2
    return 0
  fi
  gid="$(stat -c '%g' "$sock" 2>/dev/null || true)"
  if [ -z "$gid" ] || [ "$gid" = "0" ]; then
    return 0
  fi
  group_name="$(getent group "$gid" 2>/dev/null | cut -d: -f1 || true)"
  if [ -z "$group_name" ]; then
    group_name="dockersock"
    if ! addgroup -g "$gid" -S "$group_name" 2>/dev/null; then
      addgroup -g "$gid" "$group_name" 2>/dev/null || {
        echo "warn: could not create group gid=${gid} for docker socket" >&2
        return 0
      }
    fi
  fi
  if id -nG zbm 2>/dev/null | tr ' ' '\n' | grep -qx "$group_name"; then
    return 0
  fi
  if ! addgroup zbm "$group_name" 2>/dev/null; then
    echo "warn: could not add zbm to group ${group_name} (gid=${gid})" >&2
    return 0
  fi
  echo "info: granted zbm access to docker socket via group ${group_name} (gid=${gid})" >&2
}

if [ "$(id -u)" = "0" ]; then
  if ! mkdir -p "$DATA_DIR" 2>/dev/null; then
    echo "error: cannot create data directory ${DATA_DIR} (check volume mount)" >&2
    exit 1
  fi
  # Named/bind volumes are often root-owned; reclaim for the non-root app user
  # when we drop privileges. Always fix ownership so /data stays usable.
  chown -R zbm:zbm "$DATA_DIR" || {
    echo "error: cannot chown ${DATA_DIR} to zbm:zbm (check volume permissions)" >&2
    exit 1
  }

  # Docker socket ⇒ host-equivalent power; stay root so Compose→AppData can
  # write into other apps' typically root-owned AppData folders.
  if [ -S "$DOCKER_SOCK" ]; then
    echo "info: docker.sock present — running as root for AppData compose sync" >&2
    exec /usr/local/bin/zimaos-backup-manager "$@"
  fi

  ensure_dockersock_access "$DOCKER_SOCK"
  exec su-exec zbm /usr/local/bin/zimaos-backup-manager "$@"
fi

exec /usr/local/bin/zimaos-backup-manager "$@"
