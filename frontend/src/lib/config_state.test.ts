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
});
