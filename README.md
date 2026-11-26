# my-vpn

Cross-platform censorship-circumvention proxy & VPN client for **Linux (`niri` / Wayland)** and **Android (10+ / `arm64-v8a`)**, built with [MyGo](https://mygo.run/docs) + Svelte 5 (Bun + Tailwind CSS + Lucide) + [hiddify-sing-box](https://github.com/hiddify/hiddify-sing-box) v5 + [geph5-client](https://github.com/geph-official/geph5).

See [SPEC.md](./SPEC.md) for the complete technical architecture, milestones, and acceptance criteria.

## Quick Start

```bash
# Run Go unit & integration tests
make test-go

# Build Linux GUI (with embedded frontend) & core-helper
make build
./bin/looma

# Frontend (Bun 1.4+ native test/check/fmt + oxlint)
bun --cwd frontend install
bun --cwd frontend fmt --check src
bun --cwd frontend check
bun run --cwd frontend lint
bun --cwd frontend test
bun run --cwd frontend build
```

Linux requires GTK 3 and WebKitGTK 4.1 at runtime. The GUI does not need a
Vite server or the `frontend/dist` directory after building. To smoke-test
the built GUI without a desktop session, run `bash tests/e2e-linux/check_ui.sh`
(requires Xvfb, xauth, xdotool, and ImageMagick).

## Nightly releases

The [Release workflow](.github/workflows/release.yml) checks the default
branch daily at 19:23 UTC (03:23 UTC+8). If the `nightly` tag already
points at its tip, both builds are skipped. Otherwise the workflow builds
Linux amd64 and signed Android arm64, replaces the assets on the single
`nightly` prerelease, and moves the tag to the built commit. The release
stays at `/releases/tag/nightly`. This requires a mutable release and
permission to update the `nightly` tag. The existing `v*` tag builds still
upload Actions artifacts separately.

## Android release signing

Both `v*` tag builds and nightly releases use the same Android signing key.
The signed APK and AAB are uploaded as the `my-vpn-android-arm64` Actions
artifact and are also attached to the nightly prerelease. Reuse the
existing release key if you have already distributed a release APK.
Otherwise, generate a key once and configure the repository's Actions
secrets before running a release build:

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
