#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
HELPER_BIN="${ROOT_DIR}/bin/core-helper"
SOCK="/tmp/myvpn-e2e-$$.sock"
WORKDIR="$(mktemp -d)"

cleanup() {
  if [ -n "${HELPER_PID:-}" ]; then
    kill "${HELPER_PID}" 2>/dev/null || true
    wait "${HELPER_PID}" 2>/dev/null || true
  fi
  rm -rf "${SOCK}" "${WORKDIR}"
}
trap cleanup EXIT

[ -x "${HELPER_BIN}" ] || { echo "Missing ${HELPER_BIN}; run make build-helper first" >&2; exit 1; }

"${HELPER_BIN}" --socket "${SOCK}" --workdir "${WORKDIR}" &
HELPER_PID=$!

for _ in $(seq 1 30); do
  [ -S "${SOCK}" ] && break
  sleep 0.1
done
[ -S "${SOCK}" ] || { echo "core-helper socket did not appear" >&2; exit 1; }

echo "[E2E] Checking /health"
curl -fsS --unix-socket "${SOCK}" http://unix/health | grep -q '"ok":true'

echo "[E2E] Starting proxy mode via IPC"
curl -fsS --unix-socket "${SOCK}" -X POST http://unix/start \
  -H 'Content-Type: application/json' \
  -d '{
    "config": {
      "inbound": {"mode": "proxy", "socks_port": 18080, "http_port": 18081, "allow_lan": false, "tun_name": "myvpn0", "tun_address": "172.19.0.1/30", "auto_route": true, "strict_route": true},
      "chain": {"two_hop": true, "hop1": {"kind": "psiphon"}, "hop2": {"kind": "warp"}},
      "geph": {"exit_country": "auto", "local_socks_port": 19909, "control_port": 19910},
      "psiphon": {"egress_region": "auto", "establish_tunnel_timeout_sec": 30},
      "warp": {"mode": "wg", "masque": {"vhttp": "auto", "tls_fragment": true, "ech": true}},
      "routing": {"bypass_cn": true, "block_ads": true, "bypass_private": true},
      "dns": {"direct_dns": "https://dns.alidns.com/dns-query", "proxy_dns": "https://1.1.1.1/dns-query", "fake_ip": true, "hijack_dns": true, "block_non_direct_ipv6_when_unsupported": true, "reject_non_direct_udp_when_tcp_only": true}
    }
  }' | grep -q '"WARP_AUTO_MASQUE_H2"'

echo "[E2E] Checking /stats running=true"
curl -fsS --unix-socket "${SOCK}" http://unix/stats | grep -q '"running":true'

echo "[E2E] Stopping engine via IPC"
curl -fsS --unix-socket "${SOCK}" -X POST http://unix/stop | grep -q '"ok":true'
curl -fsS --unix-socket "${SOCK}" http://unix/stats | grep -q '"running":false'

echo "[E2E] PASS: core-helper IPC lifecycle + WARP_AUTO_MASQUE_H2 verified"
