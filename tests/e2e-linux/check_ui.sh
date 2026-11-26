#!/usr/bin/env bash
set -euo pipefail

# Run the packaged app from outside the source tree without a Vite server.
binary=$(realpath "${1:-bin/looma}")
workdir=$(mktemp -d)
trap 'rm -rf -- "$workdir"' EXIT

xvfb-run -a -s '-screen 0 1280x960x24' bash -euo pipefail -c '
  cd "$2"
  env -u WAYLAND_DISPLAY -u MYGO_DEV -u MYGO_DEV_URL -u MYGO_ENV \
    XDG_CONFIG_HOME="$PWD/config" GDK_BACKEND=x11 "$1" > looma.log 2>&1 &
  pid=$!
  trap '\''kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true'\'' EXIT

  if ! win=$(timeout 10 xdotool search --sync --onlyvisible --name "^Looma$"); then
    echo "Looma window did not open" >&2
    exit 1
  fi

  # Exclude the frame and wait for WebKit to paint the UI, not just a blank window.
  colors=0
  for ((i = 0; i < 30; i++)); do
    if colors=$(import -window "$win" -gravity center -crop 90%x90%+0+0 -format "%k" info:) && ((colors > 30)); then
      echo "Looma rendered its UI ($colors colors in the window interior)"
      exit 0
    fi
    sleep 0.3
  done
  echo "Looma opened but did not render its UI ($colors colors in the window interior)" >&2
  exit 1
' bash "$binary" "$workdir"
