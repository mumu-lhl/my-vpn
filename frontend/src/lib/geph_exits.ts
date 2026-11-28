import type { GephExitLocation } from "./mygo";
import { countryCodeToFlag } from "./probe";

const COUNTRY_NAMES_ZH: Record<string, string> = {
  AU: "澳大利亚",
  BG: "保加利亚",
  BR: "巴西",
  CA: "加拿大",
  CH: "瑞士",
  DE: "德国",
  ES: "西班牙",
  FI: "芬兰",
  FR: "法国",
  GB: "英国",
  HK: "中国香港",
  ID: "印度尼西亚",
  IN: "印度",
  IT: "意大利",
  JP: "日本",
  KR: "韩国",
  MY: "马来西亚",
  NL: "荷兰",
  NO: "挪威",
  PL: "波兰",
  RO: "罗马尼亚",
  SE: "瑞典",
  SG: "新加坡",
  TR: "土耳其",
  TW: "中国台湾",
  UA: "乌克兰",
  UK: "英国",
  US: "美国",
};

export function formatGephCountryName(countryCode: string): string {
  const code = (countryCode || "").trim().toUpperCase();
  if (!code || code === "AUTO") return "自动选择";
  return COUNTRY_NAMES_ZH[code] ?? code;
}

export function formatGephExitSummary(
  country?: string,
  city?: string,
): { flag: string; label: string; isAuto: boolean } {
  const code = (country || "").trim().toUpperCase();
  const cleanCity = (city || "").trim();
  if (!code || code === "AUTO") {
    return { flag: "🌐", label: "自动选择（智能路由）", isAuto: true };
  }
  const flag = countryCodeToFlag(code);
  const name = formatGephCountryName(code);
  const label = cleanCity ? `${name} · ${cleanCity}` : `${name} (${code})`;
  return { flag, label, isAuto: false };
}

export function formatGephExitLoad(load?: number): {
  percentText: string;
  level: "low" | "medium" | "high" | "unknown";
} {
  if (typeof load !== "number" || Number.isNaN(load) || load < 0) {
    return { percentText: "--", level: "unknown" };
  }
  const pct = Math.min(100, Math.max(0, Math.round(load * 100)));
  if (pct < 50) return { percentText: `${pct}%`, level: "low" };
  if (pct < 80) return { percentText: `${pct}%`, level: "medium" };
  return { percentText: `${pct}%`, level: "high" };
}

export function isGephExitSelected(
  selectedCountry: string,
  selectedCity: string,
  exitCountry: string,
  exitCity: string,
): boolean {
  const selC = (selectedCountry || "auto").trim().toUpperCase();
  const exC = (exitCountry || "auto").trim().toUpperCase();
  if (selC === "AUTO" && exC === "AUTO") return true;
  if (selC !== exC) return false;
  const selCity = (selectedCity || "").trim().toLowerCase();
  const exCity = (exitCity || "").trim().toLowerCase();
  if (!selCity) return exCity === "";
  return selCity === exCity;
}

export function sortGephExits(exits: GephExitLocation[]): GephExitLocation[] {
  return [...exits].sort((a, b) => {
    const la = typeof a.load === "number" ? a.load : 999;
    const lb = typeof b.load === "number" ? b.load : 999;
    const ca = (a.country || "").toUpperCase();
    const cb = (b.country || "").toUpperCase();
    if (ca !== cb) return ca.localeCompare(cb);
    if (la !== lb) return la - lb;
    return (a.city || "").localeCompare(b.city || "");
  });
}
