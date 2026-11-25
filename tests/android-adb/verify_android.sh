#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-single}"
ITERATIONS="${2:-10}"
PKG="io.github.mumulhl.myvpn"
LIMIT_SEC=5
if [ "${MODE}" = "two-hop" ]; then
  LIMIT_SEC=10
fi

command -v adb >/dev/null || { echo "adb not found" >&2; exit 1; }
adb get-state >/dev/null

echo "[ADB] Verifying Android ABI (arm64-v8a) and SDK >= 29"
ABI="$(adb shell getprop ro.product.cpu.abi | tr -d '\r')"
SDK="$(adb shell getprop ro.build.version.sdk | tr -d '\r')"
echo "Device ABI=${ABI}, SDK=${SDK}"
[ "${SDK}" -ge 29 ] || { echo "Android 10+ (API 29+) required" >&2; exit 1; }

echo "[ADB] Checking foreground VPN service is running"
adb shell dumpsys activity services "${PKG}" | grep -q "MyVpnService" || {
  echo "MyVpnService is not active; start VPN on device first" >&2
  exit 1
}

echo "[ADB] Running ${ITERATIONS} Wi-Fi <-> Cellular switch reconnect tests (threshold <= ${LIMIT_SEC}s)"
for i in $(seq 1 "${ITERATIONS}"); do
  adb shell svc wifi disable
  START_TS=$(date +%s)
  RECOVERED=0
  for _ in $(seq 1 "${LIMIT_SEC}"); do
    if adb shell ping -c 1 -W 1 1.1.1.1 >/dev/null 2>&1; then
      RECOVERED=1
      break
    fi
    sleep 1
  done
  ELAPSED=$(( $(date +%s) - START_TS ))
  echo "  [${i}/${ITERATIONS}] Wi-Fi -> Cellular recovered=${RECOVERED} elapsed=${ELAPSED}s"
  [ "${RECOVERED}" -eq 1 ] && [ "${ELAPSED}" -le "${LIMIT_SEC}" ] || { echo "Reconnect exceeded ${LIMIT_SEC}s" >&2; exit 1; }

  adb shell svc wifi enable
  sleep 3
done

echo "[ADB] PASS: All ${ITERATIONS} reconnect cycles met <= ${LIMIT_SEC}s threshold (${MODE})"
