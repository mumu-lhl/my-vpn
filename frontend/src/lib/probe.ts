import type { ChainConfig, HopKind } from "./chain";

export interface TestURLPreset {
  id: string;
  label: string;
  url: string;
}

export interface ConnectivityResult {
  url: string;
  reachable: boolean;
  latency_ms: number;
  status_code?: number;
  error?: string;
}

export interface ExitIPInfo {
  ip: string;
  country_code: string;
  flag: string;
  city?: string;
  org?: string;
  provider?: string;
  error?: string;
}

export interface HopProbeResult {
  hop_index: number;
  label: string;
  kind: string;
  proxy_addr?: string;
  connectivity: ConnectivityResult;
  exit_ip: ExitIPInfo;
}

export interface ChainProbeResult {
  test_url: string;
  hops: HopProbeResult[];
}

export const BUILTIN_PROBE_URLS: TestURLPreset[] = [
  {
    id: "gstatic-204",
    label: "Google 204（默认）",
    url: "http://connectivitycheck.gstatic.com/generate_204",
  },
  {
    id: "cloudflare-204",
    label: "Cloudflare 204",
    url: "http://cp.cloudflare.com/generate_204",
  },
  {
    id: "gstatic-https-204",
    label: "Google HTTPS 204",
    url: "https://www.gstatic.com/generate_204",
  },
  {
    id: "apple-captive",
    label: "Apple Captive",
    url: "http://captive.apple.com/hotspot-detect.html",
  },
  {
    id: "google-doh",
    label: "Google DoH",
    url: "https://dns.google/dns-query",
  },
  {
    id: "cloudflare-ip",
    label: "Cloudflare 1.1.1.1",
    url: "http://1.1.1.1",
  },
];

/** Converts an ISO 3166-1 alpha-2 country code (e.g. "JP") into a flag emoji ("🇯🇵"). */
export function countryCodeToFlag(code: string): string {
  const c = code.trim().toUpperCase();
  if (!/^[A-Z]{2}$/.test(c)) {
    return "🌐";
  }
  const base = 0x1f1e6;
  return String.fromCodePoint(
    base + (c.charCodeAt(0) - 65),
    base + (c.charCodeAt(1) - 65),
  );
}

export function validateProbeUrl(raw: string): boolean {
  const trimmed = raw.trim();
  if (!trimmed) {
    return false;
  }
  try {
    const u = new URL(trimmed);
    return (
      (u.protocol === "http:" || u.protocol === "https:") && u.host.length > 0
    );
  } catch {
    return false;
  }
}

export function formatLatency(res: ConnectivityResult): string {
  if (!res.reachable) {
    return res.error ? `失败 (${res.error})` : "不可达";
  }
  return `${res.latency_ms} ms`;
}

export function buildPreviewProbeResult(
  testURL: string,
  chain: ChainConfig,
): ChainProbeResult {
  const mkHop = (
    index: number,
    kind: HopKind,
    label: string,
  ): HopProbeResult => ({
    hop_index: index,
    label,
    kind,
    connectivity: {
      url: testURL,
      reachable: false,
      latency_ms: 0,
      error: "未连接本地服务",
    },
    exit_ip: {
      ip: "--",
      country_code: "",
      flag: countryCodeToFlag(""),
      error: "连接后可获取出口信息",
    },
  });
  return {
    test_url: testURL,
    hops: chain.two_hop
      ? [
          mkHop(1, chain.hop1.kind, `第一跳 · ${chain.hop1.kind}`),
          mkHop(2, chain.hop2.kind, `第二跳 · ${chain.hop2.kind}`),
        ]
      : [mkHop(1, chain.hop1.kind, `当前出口 · ${chain.hop1.kind}`)],
  };
}
