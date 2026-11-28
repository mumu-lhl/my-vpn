import { formatGephCountryName } from "./geph_exits";
import type { VPNGateNode } from "./mygo";
import { countryCodeToFlag } from "./probe";

export type VPNGateSortField = "score" | "speed" | "total_traffic" | "ping";
export type VPNGateSortDir = "asc" | "desc";

export interface VPNGateCountryOption {
  code: string;
  name: string;
  flag: string;
  count: number;
}

export function defaultVPNGateSortDir(field: VPNGateSortField): VPNGateSortDir {
  return field === "ping" ? "asc" : "desc";
}

export function formatVPNGateCountry(
  countryShort?: string,
  countryLong?: string,
): { flag: string; name: string } {
  const code = (countryShort || "").trim().toUpperCase();
  const flag = code ? countryCodeToFlag(code) : "🌐";
  const localized = code ? formatGephCountryName(code) : "";
  const name =
    localized && localized !== code
      ? localized
      : (countryLong || code || "未知地区").trim();
  return { flag, name };
}

export function extractVPNGateCountries(
  nodes: VPNGateNode[],
): VPNGateCountryOption[] {
  const map = new Map<string, VPNGateCountryOption>();
  for (const n of nodes) {
    const code = (n.country_short || "").trim().toUpperCase();
    if (!code) continue;
    const prev = map.get(code);
    if (prev) {
      prev.count += 1;
    } else {
      const { flag, name } = formatVPNGateCountry(code, n.country_long);
      map.set(code, { code, name, flag, count: 1 });
    }
  }
  return [...map.values()].sort(
    (a, b) => b.count - a.count || a.code.localeCompare(b.code),
  );
}

export function filterVPNGateNodes(
  nodes: VPNGateNode[],
  countryCode: string,
): VPNGateNode[] {
  const target = (countryCode || "ALL").trim().toUpperCase();
  if (!target || target === "ALL") return nodes;
  return nodes.filter(
    (n) => (n.country_short || "").trim().toUpperCase() === target,
  );
}

export function sortVPNGateNodes(
  nodes: VPNGateNode[],
  field: VPNGateSortField,
  dir: VPNGateSortDir,
): VPNGateNode[] {
  const mult = dir === "asc" ? 1 : -1;
  return [...nodes].sort((a, b) => {
    let va = 0;
    let vb = 0;
    switch (field) {
      case "speed":
        va = a.speed || 0;
        vb = b.speed || 0;
        break;
      case "total_traffic":
        va = a.total_traffic || 0;
        vb = b.total_traffic || 0;
        break;
      case "ping":
        va = a.ping > 0 ? a.ping : 999999;
        vb = b.ping > 0 ? b.ping : 999999;
        break;
      case "score":
      default:
        va = a.score || 0;
        vb = b.score || 0;
        break;
    }
    if (va !== vb) return (va - vb) * mult;
    return (b.score || 0) - (a.score || 0);
  });
}

export function formatVPNGateSpeed(bps: number): string {
  if (!bps || bps <= 0) return "--";
  const mbps = bps / 1_000_000;
  if (mbps >= 100) return `${mbps.toFixed(0)} Mbps`;
  return `${mbps.toFixed(1)} Mbps`;
}

export function formatVPNGateTraffic(bytes: number): string {
  if (!bytes || bytes <= 0) return "--";
  const tb = bytes / 1024 ** 4;
  if (tb >= 1) return `${tb.toFixed(2)} TB`;
  const gb = bytes / 1024 ** 3;
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / 1024 ** 2;
  return `${mb.toFixed(0)} MB`;
}

export function formatVPNGateScore(score: number): string {
  if (!score || score <= 0) return "--";
  if (score >= 1_000_000) return `${(score / 1_000_000).toFixed(1)}M`;
  if (score >= 1_000) return `${(score / 1_000).toFixed(0)}K`;
  return String(score);
}
