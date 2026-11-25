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
