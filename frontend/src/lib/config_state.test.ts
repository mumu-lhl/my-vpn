import { describe, expect, it } from "bun:test";
import {
  buildConfigPayload,
  createDefaultFormState,
  hydrateFormState,
} from "./config_state";

describe("config_state round-trip serialization (UI-AC-06)", () => {
  it("hydrates backend config and preserves unrelated fields on save", () => {
    const backendCfg = {
      inbound: {
        mode: "tun",
        socks_port: 20808,
        http_port: 20809,
        allow_lan: true,
        kill_switch: true,
        tun_name: "looma0",
      },
      routing: {
        bypass_cn: false,
        bypass_private: false,
        block_ads: true,
        direct_bypass: { enable_default_preset: false },
      },
      linux: {
        minimize_to_tray: false,
        set_system_proxy: false,
        core_helper_path: "/usr/local/bin/looma-helper",
      },
      android: {
        per_app_mode: "whitelist",
        per_app_packages: ["com.tencent.mm"],
      },
    };

    const form = hydrateFormState(backendCfg, createDefaultFormState());
    expect(form.inboundMode).toBe("tun");
    expect(form.bypassPrivate).toBe(false);
    expect(form.minimizeToTray).toBe(false);
    expect(form.perAppMode).toBe("whitelist");
    expect(form.excludedPackages).toEqual(["com.tencent.mm"]);

    form.minimizeToTray = true;
    form.bypassPrivate = true;
    const payload = buildConfigPayload(backendCfg, form);
    expect(payload.inbound.tun_name).toBe("looma0");
    expect(payload.linux.core_helper_path).toBe("/usr/local/bin/looma-helper");
    expect(payload.linux.minimize_to_tray).toBe(true);
    expect(payload.routing.bypass_private).toBe(true);
  });

  it("hydrates and serializes serverless direct_bypass scope, fragmentation, and DoH fields", () => {
    const hydrated = hydrateFormState({
      chain: {
        two_hop: true,
        hop1: { kind: "direct_bypass" },
        hop2: { kind: "warp" },
      },
      routing: {
        bypass_cn: true,
        bypass_private: true,
        block_ads: false,
        direct_bypass: {
          enable_default_preset: true,
          scope: "list",
          tls_fragment_size: "12-24",
          tls_fragment_sleep: "3-6ms",
          tls_record_fragment: false,
          http_mix_host_case: true,
          doh_url: "https://1.0.0.1/dns-query",
          doh_static_ips: ["1.0.0.1", "1.1.1.1"],
          doh_disable_sni: true,
        },
      },
    });
    expect(hydrated.chain.two_hop).toBe(false);
    expect(hydrated.chain.hop1.kind).toBe("direct_bypass");
    expect(hydrated.directBypassScope).toBe("list");
    expect(hydrated.directBypassDoHStaticIPs).toBe("1.0.0.1, 1.1.1.1");

    const payload = buildConfigPayload({}, hydrated);
    expect(payload.chain.two_hop).toBe(false);
    expect(payload.routing.direct_bypass.scope).toBe("list");
    expect(payload.routing.direct_bypass.tls_fragment_size).toBe("12-24");
    expect(payload.routing.direct_bypass.tls_record_fragment).toBe(false);
    expect(payload.routing.direct_bypass.http_mix_host_case).toBe(true);
    expect(payload.routing.direct_bypass.doh_url).toBe(
      "https://1.0.0.1/dns-query",
    );
    expect(payload.routing.direct_bypass.doh_static_ips).toEqual([
      "1.0.0.1",
      "1.1.1.1",
    ]);
  });
});

describe("buildConfigPayload Proxy safety", () => {
  it("clones Proxy-wrapped rawConfig without throwing DataCloneError", () => {
    const backing = {
      inbound: { mode: "proxy", socks_port: 10808, http_port: 10809 },
      routing: { direct_bypass: { preset_id: "disorder_oob" } },
      custom_unknown_field: { nested: [1, 2, 3] },
    };
    const proxied = new Proxy(backing, {});
    const form = createDefaultFormState();
    form.inboundMode = "tun";
    const payload = buildConfigPayload(proxied, form);
    expect(payload.inbound.mode).toBe("tun");
    expect(payload.custom_unknown_field).toEqual({ nested: [1, 2, 3] });
  });
});
