#!/bin/bash
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

if [ -e /dev/dri/renderD128 ]; then
  PGID=$(stat -c '%g' /dev/dri/renderD128)
fi

if [ "$(id -g mediahub)" != "$PGID" ]; then
  groupmod -o -g "$PGID" mediahub
fi
if [ "$(id -u mediahub)" != "$PUID" ]; then
  usermod -o -u "$PUID" mediahub
fi

mkdir -p /config /recordings /run/user/$PUID
chown "$PUID:$PGID" /config /recordings /run/user/$PUID
export XDG_RUNTIME_DIR=/run/user/$PUID

for f in /defaults/*.json; do
  base=$(basename "$f")
  [ -f "/config/$base" ] || cp "$f" "/config/$base"
done

exec gosu mediahub mediahub "$@"
