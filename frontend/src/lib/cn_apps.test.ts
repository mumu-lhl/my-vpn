import { describe, expect, it } from "bun:test";
import {
  CLASSIC_CN_APPS,
  detectChinaApps,
  mergeUniquePackages,
} from "./cn_apps";

describe("detectChinaApps & classic presets", () => {
  it("contains WeChat, QQ, Xiaohongshu in classic preset", () => {
    const pkgs = CLASSIC_CN_APPS.map((a) => a.package_name);
    expect(pkgs).toContain("com.tencent.mm");
    expect(pkgs).toContain("com.tencent.mobileqq");
    expect(pkgs).toContain("com.xingin.xhs");
  });

  it("auto-detects CN apps by classic list, store, vendor prefix, and CJK label while keeping global apps", () => {
    const detected = detectChinaApps([
      { package_name: "com.tencent.mm", label: "微信" },
      {
        package_name: "com.example.shop",
        label: "Shop",
        installer_package: "com.xiaomi.market",
      },
      { package_name: "com.bytedance.internal", label: "Internal" },
      { package_name: "com.local.transit", label: "公交卡" },
      { package_name: "org.telegram.messenger", label: "Telegram" },
      { package_name: "com.zhiliaoapp.musically", label: "TikTok" },
    ]);
    expect(detected.map((d) => d.package_name)).toEqual([
      "com.tencent.mm",
      "com.example.shop",
      "com.bytedance.internal",
      "com.local.transit",
    ]);
  });

  it("deduplicates merged packages", () => {
    expect(
      mergeUniquePackages(
        ["com.tencent.mm"],
        ["com.tencent.mm", "com.xingin.xhs"],
      ),
    ).toEqual(["com.tencent.mm", "com.xingin.xhs"]);
  });
});
