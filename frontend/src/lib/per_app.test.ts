import { describe, expect, it } from "bun:test";
import {
  buildPerAppCatalog,
  filterAndSortPerApps,
  selectClassicCNPackages,
  selectDetectedCNPackages,
  togglePackageSelection,
} from "./per_app";
import type { InstalledApp } from "./cn_apps";

const SAMPLE_APPS: InstalledApp[] = [
  { package_name: "com.tencent.mm", label: "微信", is_system: false },
  {
    package_name: "com.android.settings",
    label: "设置",
    is_system: true,
  },
  {
    package_name: "org.telegram.messenger",
    label: "Telegram",
    is_system: false,
  },
  {
    package_name: "com.example.local",
    label: "本地阅读器",
    is_system: false,
    installer_package: "com.xiaomi.market",
  },
  {
    package_name: "io.github.mumulhl.looma",
    label: "Looma",
    is_system: false,
  },
];

describe("per_app catalog, filtering, and CN integration (UI-AC-03)", () => {
  it("excludes Looma itself and keeps manually selected unknown packages visible", () => {
    const catalog = buildPerAppCatalog(SAMPLE_APPS, ["com.custom.app"]);
    expect(
      catalog.some((a) => a.package_name === "io.github.mumulhl.looma"),
    ).toBe(false);
    const custom = catalog.find((a) => a.package_name === "com.custom.app");
    expect(custom).toBeDefined();
    expect(custom?.selected).toBe(true);
  });

  it("hides unselected system apps by default, supports search, and pins selected apps first", () => {
    const catalog = buildPerAppCatalog(SAMPLE_APPS, ["org.telegram.messenger"]);
    const defaultList = filterAndSortPerApps(catalog, {
      query: "",
      showSystem: false,
      onlySelected: false,
      selectedFirst: true,
    });
    expect(
      defaultList.some((a) => a.package_name === "com.android.settings"),
    ).toBe(false);
    expect(defaultList[0].package_name).toBe("org.telegram.messenger");

    const withSystem = filterAndSortPerApps(catalog, {
      query: "settings",
      showSystem: true,
      onlySelected: false,
      selectedFirst: true,
    });
    expect(withSystem).toHaveLength(1);
    expect(withSystem[0].package_name).toBe("com.android.settings");

    const onlySelected = filterAndSortPerApps(catalog, {
      query: "",
      showSystem: true,
      onlySelected: true,
      selectedFirst: true,
    });
    expect(onlySelected).toHaveLength(1);
    expect(onlySelected[0].package_name).toBe("org.telegram.messenger");
  });

  it("toggles individual packages cleanly", () => {
    const added = togglePackageSelection([], "com.tencent.mm");
    expect(added).toEqual(["com.tencent.mm"]);
    const removed = togglePackageSelection(added, "com.tencent.mm");
    expect(removed).toEqual([]);
  });

  it("integrates classic CN app quick-select and multi-signal CN detection", () => {
    const classicRes = selectClassicCNPackages(SAMPLE_APPS, []);
    expect(classicRes.packages).toEqual(["com.tencent.mm"]);
    expect(classicRes.addedCount).toBe(1);

    const detectedRes = selectDetectedCNPackages(
      SAMPLE_APPS,
      classicRes.packages,
    );
    expect(detectedRes.packages).toContain("com.tencent.mm");
    expect(detectedRes.packages).toContain("com.example.local");
    expect(detectedRes.packages).not.toContain("org.telegram.messenger");
    expect(detectedRes.addedCount).toBe(1);
    expect(detectedRes.detectedCount).toBe(2);
  });
});
