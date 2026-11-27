import { describe, expect, it } from "bun:test";
import { getPlatformUIProfile, normalizeRuntimePlatform } from "./platform";

describe("platform UI isolation profile", () => {
  it("normalizes runtime platform strings", () => {
    expect(normalizeRuntimePlatform("android")).toBe("android");
    expect(normalizeRuntimePlatform("Linux")).toBe("linux");
    expect(normalizeRuntimePlatform("darwin")).toBe("darwin");
    expect(normalizeRuntimePlatform("")).toBe("unknown");
  });

  it("hides desktop-only items and uses VPN terminology on Android", () => {
    const ui = getPlatformUIProfile("android");
    expect(ui.isAndroid).toBe(true);
    expect(ui.tunModeLabel).toBe("VPN 模式");
    expect(ui.tunBadgeLabel).toBe("VPN 模式");
    expect(ui.showTrayOption).toBe(false);
    expect(ui.showSystemProxyOption).toBe(false);
    expect(ui.showLinuxRawFake).toBe(false);
    expect(ui.showPerAppProxy).toBe(true);
    expect(ui.showAndroidVpnShortcut).toBe(true);
    expect(ui.killSwitchMode).toBe("android_shortcut");
  });

  it("hides Android-only items and keeps TUN/tray/RawFake on Linux desktop", () => {
    const ui = getPlatformUIProfile("linux");
    expect(ui.isLinuxDesktop).toBe(true);
    expect(ui.tunModeLabel).toBe("全局 TUN");
    expect(ui.tunBadgeLabel).toBe("TUN 模式");
    expect(ui.showTrayOption).toBe(true);
    expect(ui.showSystemProxyOption).toBe(true);
    expect(ui.showLinuxRawFake).toBe(true);
    expect(ui.showPerAppProxy).toBe(false);
    expect(ui.showAndroidVpnShortcut).toBe(false);
    expect(ui.killSwitchMode).toBe("toggle");
  });
});
