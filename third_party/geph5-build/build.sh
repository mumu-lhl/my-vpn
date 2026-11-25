#!/usr/bin/env bash
set -euo pipefail

# Cross-compile geph5-client for Linux (x86_64/aarch64) and Android (arm64-v8a as libgeph5.so).
# Requires: rustup, cargo, and cargo-ndk (for Android) + ANDROID_NDK_HOME.

GEPH5_REPO="${GEPH5_REPO:-https://github.com/geph-official/geph5.git}"
GEPH5_REF="${GEPH5_REF:-master}"
OUT_DIR="$(cd "$(dirname "$0")" && pwd)/out"
SRC_DIR="${OUT_DIR}/src"

mkdir -p "${OUT_DIR}"

if [ ! -d "${SRC_DIR}" ]; then
  git clone --depth 1 --branch "${GEPH5_REF}" "${GEPH5_REPO}" "${SRC_DIR}"
fi

build_linux() {
  local target="${1:-x86_64-unknown-linux-gnu}"
  echo "[geph5] Building Linux target: ${target}"
  (cd "${SRC_DIR}" && cargo build --release --bin geph5-client --target "${target}")
  mkdir -p "${OUT_DIR}/linux-${target}"
  cp "${SRC_DIR}/target/${target}/release/geph5-client" "${OUT_DIR}/linux-${target}/geph5-client"
}

build_android_arm64() {
  echo "[geph5] Building Android arm64-v8a (libgeph5.so)"
  : "${ANDROID_NDK_HOME:?ANDROID_NDK_HOME must be set for Android cross-compilation}"
  (cd "${SRC_DIR}" && cargo ndk -t arm64-v8a --platform 29 build --release --bin geph5-client)
  mkdir -p "${OUT_DIR}/android-arm64-v8a"
  cp "${SRC_DIR}/target/aarch64-linux-android/release/geph5-client" "${OUT_DIR}/android-arm64-v8a/libgeph5.so"
}

case "${1:-all}" in
  linux) build_linux "${2:-x86_64-unknown-linux-gnu}" ;;
  android) build_android_arm64 ;;
  all)
    build_linux x86_64-unknown-linux-gnu
    build_android_arm64
    ;;
  *)
    echo "Usage: $0 [linux [target] | android | all]" >&2
    exit 1
    ;;
esac
