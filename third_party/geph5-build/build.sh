#!/usr/bin/env bash
set -euo pipefail

# Cross-compile geph5-client for Linux (musl/gnu) and Android (arm64-v8a as libgeph5.so).
# Requires: rustup, cargo, and cargo-ndk (for Android) + Android NDK.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

GEPH5_REPO="${GEPH5_REPO:-https://github.com/geph-official/geph5.git}"
GEPH5_REF="${GEPH5_REF:-master}"
OUT_DIR="${OUT_DIR:-${SCRIPT_DIR}/out}"
SRC_DIR="${OUT_DIR}/src"

mkdir -p "${OUT_DIR}"

ensure_source() {
  if [ ! -d "${SRC_DIR}/.git" ]; then
    rm -rf "${SRC_DIR}"
    git clone --depth 1 --branch "${GEPH5_REF}" "${GEPH5_REPO}" "${SRC_DIR}"
  fi
}

ensure_rust_target() {
  local target="$1"
  if command -v rustup >/dev/null 2>&1; then
    if ! rustup target list --installed | grep -qx "${target}"; then
      echo "[geph5] Installing Rust target: ${target}"
      rustup target add "${target}"
    fi
  fi
}

resolve_android_ndk() {
  if [ -n "${ANDROID_NDK_HOME:-}" ] && [ -d "${ANDROID_NDK_HOME}" ]; then
    echo "${ANDROID_NDK_HOME}"
    return 0
  fi
  if [ -n "${ANDROID_NDK_LATEST_HOME:-}" ] && [ -d "${ANDROID_NDK_LATEST_HOME}" ]; then
    echo "${ANDROID_NDK_LATEST_HOME}"
    return 0
  fi
  local sdk_root="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
  if [ -n "${sdk_root}" ] && [ -d "${sdk_root}/ndk" ]; then
    local latest
    latest="$(find "${sdk_root}/ndk" -mindepth 1 -maxdepth 1 -type d | sort -V | tail -n 1)"
    if [ -n "${latest}" ]; then
      echo "${latest}"
      return 0
    fi
  fi
  return 1
}

build_linux() {
  local target="${1:-x86_64-unknown-linux-musl}"
  ensure_source
  ensure_rust_target "${target}"

  echo "[geph5] Building Linux target: ${target}"
  (
    cd "${SRC_DIR}"
    cargo build --locked --release \
      --manifest-path binaries/geph5-client/Cargo.toml \
      --bin geph5-client \
      --target "${target}"
  )

  local out_bin="${OUT_DIR}/linux-${target}/geph5-client"
  mkdir -p "$(dirname "${out_bin}")"
  cp "${SRC_DIR}/target/${target}/release/geph5-client" "${out_bin}"
  chmod 0755 "${out_bin}"

  mkdir -p "${ROOT_DIR}/bin"
  cp "${out_bin}" "${ROOT_DIR}/bin/geph5-client"
  chmod 0755 "${ROOT_DIR}/bin/geph5-client"
  echo "[geph5] Staged Linux binary at ${ROOT_DIR}/bin/geph5-client"
}

build_android_arm64() {
  ensure_source
  ensure_rust_target "aarch64-linux-android"

  local ndk_home
  if ! ndk_home="$(resolve_android_ndk)"; then
    echo "ANDROID_NDK_HOME (or ANDROID_NDK_LATEST_HOME / ANDROID_HOME/ndk) must be set for Android cross-compilation" >&2
    exit 1
  fi
  export ANDROID_NDK_HOME="${ndk_home}"

  if ! command -v cargo-ndk >/dev/null 2>&1; then
    echo "cargo-ndk is required for Android cross-compilation (install with: cargo install cargo-ndk --locked)" >&2
    exit 1
  fi

  echo "[geph5] Building Android arm64-v8a (libgeph5.so, 16 KB page-aligned) using NDK ${ANDROID_NDK_HOME}"
  # Android 15+ devices require 16 KB ELF page alignment for native executables.
  local rustflags="${RUSTFLAGS:-}"
  if [[ "${rustflags}" != *"max-page-size=16384"* ]]; then
    rustflags="${rustflags:+${rustflags} }-C link-arg=-Wl,-z,max-page-size=16384"
  fi
  (
    cd "${SRC_DIR}"
    RUSTFLAGS="${rustflags}" cargo ndk -t arm64-v8a --platform 29 \
      build --locked --release \
      --manifest-path binaries/geph5-client/Cargo.toml \
      --bin geph5-client
  )

  local out_so="${OUT_DIR}/android-arm64-v8a/libgeph5.so"
  mkdir -p "$(dirname "${out_so}")"
  cp "${SRC_DIR}/target/aarch64-linux-android/release/geph5-client" "${out_so}"
  chmod 0755 "${out_so}"

  local jni_dir="${ROOT_DIR}/.mygo/android/app/src/main/jniLibs/arm64-v8a"
  mkdir -p "${jni_dir}"
  cp "${out_so}" "${jni_dir}/libgeph5.so"
  chmod 0755 "${jni_dir}/libgeph5.so"
  echo "[geph5] Staged Android binary at ${jni_dir}/libgeph5.so"
}

case "${1:-all}" in
  linux) build_linux "${2:-x86_64-unknown-linux-musl}" ;;
  android) build_android_arm64 ;;
  all)
    build_linux "${2:-x86_64-unknown-linux-musl}"
    build_android_arm64
    ;;
  *)
    echo "Usage: $0 [linux [target] | android | all [linux-target]]" >&2
    exit 1
    ;;
esac
