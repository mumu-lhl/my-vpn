import { describe, expect, it } from "bun:test";
import {
  formatGephCountryName,
  formatGephExitLoad,
  formatGephExitSummary,
  isGephExitSelected,
  sortGephExits,
} from "./geph_exits";

describe("geph_exits helpers (AC-GEPH-02, AC-GEPH-04)", () => {
  it("formats exit summary with flag and localized country + city", () => {
    const auto = formatGephExitSummary("auto", "");
    expect(auto.isAuto).toBe(true);
    expect(auto.flag).toBe("🌐");

    const jpTokyo = formatGephExitSummary("jp", "Tokyo");
    expect(jpTokyo.isAuto).toBe(false);
    expect(jpTokyo.flag).toBe("🇯🇵");
    expect(jpTokyo.label).toBe("日本 · Tokyo");

    expect(formatGephCountryName("SG")).toBe("新加坡");
  });

  it("formats exit load percentage and severity level", () => {
    expect(formatGephExitLoad(0.24)).toEqual({
      percentText: "24%",
      level: "low",
    });
    expect(formatGephExitLoad(0.65)).toEqual({
      percentText: "65%",
      level: "medium",
    });
    expect(formatGephExitLoad(0.92)).toEqual({
      percentText: "92%",
      level: "high",
    });
    expect(formatGephExitLoad(undefined)).toEqual({
      percentText: "--",
      level: "unknown",
    });
  });

  it("matches selected country/city and sorts exits consistently", () => {
    expect(isGephExitSelected("JP", "Tokyo", "jp", "tokyo")).toBe(true);
    expect(isGephExitSelected("JP", "Tokyo", "jp", "Osaka")).toBe(false);
    expect(isGephExitSelected("auto", "", "auto", "")).toBe(true);

    const sorted = sortGephExits([
      { country: "US", city: "Newark", load: 0.5 },
      { country: "JP", city: "Osaka", load: 0.7 },
      { country: "JP", city: "Tokyo", load: 0.2 },
    ]);
    expect(sorted.map((x) => `${x.country}:${x.city}`)).toEqual([
      "JP:Tokyo",
      "JP:Osaka",
      "US:Newark",
    ]);
  });
});
