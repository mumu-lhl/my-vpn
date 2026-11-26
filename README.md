# my-vpn

Cross-platform censorship-circumvention proxy & VPN client for **Linux (`niri` / Wayland)** and **Android (10+ / `arm64-v8a`)**, built with [MyGo](https://mygo.run/docs) + Svelte 5 (Bun + Tailwind CSS + Lucide) + [hiddify-sing-box](https://github.com/hiddify/hiddify-sing-box) v5 + [geph5-client](https://github.com/geph-official/geph5).

See [SPEC.md](./SPEC.md) for the complete technical architecture, milestones, and acceptance criteria.

## Quick Start

```bash
# Run Go unit & integration tests
make test-go

# Build Linux GUI & core-helper
make build

# Frontend (Bun 1.4+ native test/check/fmt + oxlint)
bun --cwd frontend install
bun --cwd frontend fmt --check src
bun --cwd frontend check
bun --cwd frontend run lint
bun --cwd frontend test
bun --cwd frontend run build
```

## Android release signing

The [Release workflow](.github/workflows/release.yml) runs on `v*` tags and
uploads a signed arm64 APK and AAB as the `my-vpn-android-arm64` Actions
artifact. Reuse the existing release key if you have already distributed a
release APK. Otherwise, generate a key once and configure the repository's
Actions secrets before pushing a tag:

```bash
keytool -genkeypair -keystore looma-release.p12 -storetype PKCS12 \
  -alias looma -keyalg RSA -keysize 3072 -validity 10000
base64 -w0 looma-release.p12 | gh secret set ANDROID_KEYSTORE_BASE64
gh secret set MYGO_ANDROID_KEYSTORE_PASSWORD
```

The workflow expects the alias `looma`, as set in `mygo.config.ts`. Set
`MYGO_ANDROID_KEY_PASSWORD` as a separate Actions secret only if the key
password differs from the keystore password. Keep a secure backup of the
keystore and passwords outside GitHub; never commit them.

Before distributing the APK, run `apksigner verify --print-certs <apk>` and
compare the SHA-256 certificate fingerprint with your release key. For
in-place upgrades, keep the package ID and signing key unchanged and bump
the app version in `mygo.config.ts` (MyGo derives `versionCode` from it).
If a debug-signed build with the same package ID is installed, Android
requires uninstalling it first, which removes its local app data.
