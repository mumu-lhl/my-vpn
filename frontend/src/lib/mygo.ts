import type { ChainProbeResult, TestURLPreset } from "./probe";
import type { ClassicCNApp, DetectedCNApp, InstalledApp } from "./cn_apps";

// Typed client bridge matching MyGo bound AppService methods.
declare global {
  interface Window {
    mygo?: {
      call: <T = unknown>(method: string, ...args: unknown[]) => Promise<T>;
    };
  }
}

export interface BypassCapabilityStatus {
  os: string;
  raw_cap_available: boolean;
  degraded: boolean;
  notice?: string;
}

export interface BypassStrategy {
  id: string;
  name: string;
  description: string;
  methods: string[];
  split_at_sni: boolean;
  mix_sni_case: boolean;
  split_count: number;
  fragment_size: string;
  fragment_sleep: string;
  ttl: number;
  http_mix_host_case: boolean;
  quic_mode: string;
  requires_raw: boolean;
}

export interface StrategyProbeResult {
  strategy_id: string;
  strategy_name: string;
  success: boolean;
  latency_ms: number;
  used_ech: boolean;
  error?: string;
}

export interface AutoProbeReport {
  domain: string;
  resolved_ips?: string[];
  ech_available: boolean;
  ech_handshake_ok: boolean;
  recommended_id?: string;
  recommended_name?: string;
  best_latency_ms?: number;
  capability: BypassCapabilityStatus;
  results: StrategyProbeResult[];
  error?: string;
}

export interface DomainCacheEntry {
  domain: string;
  strategy_id: string;
  latency_ms: number;
  ech_supported: boolean;
  failure_count: number;
  locked: boolean;
  updated_at_unix: number;
}

export interface TrafficStats {
  running: boolean;
  up_bytes: number;
  down_bytes: number;
  mode: string;
  chain_desc: string;
  bypass_capability?: BypassCapabilityStatus;
}

export interface LANInfo {
  ip: string;
  socks_port: number;
  http_port: number;
  allow_lan: boolean;
}

export interface ExternalNode {
  id: string;
  name: string;
  protocol: string;
  server: string;
  port: number;
  latency_ms?: number;
}

export interface SpeedTestResult {
  node_id: string;
  latency_ms: number;
  err?: string;
}

export interface GephExitLocation {
  country: string;
  city: string;
  load?: number;
}

export function hasMyGoBridge(): boolean {
  return (
    typeof window !== "undefined" && typeof window.mygo?.call === "function"
  );
}

async function callGo<T>(method: string, ...args: unknown[]): Promise<T> {
  if (hasMyGoBridge()) {
    return window.mygo!.call<T>(method, ...args);
  }
  throw new Error(`MyGo runtime bridge unavailable for ${method}`);
}

export const AppService = {
  GetRuntimePlatform: () => callGo<string>("AppService.GetRuntimePlatform"),
  GetConfig: () => callGo<Record<string, any>>("AppService.GetConfig"),
  SaveConfig: (cfg: Record<string, any>) =>
    callGo<void>("AppService.SaveConfig", cfg),
  SwitchInboundMode: (mode: "proxy" | "tun") =>
    callGo<void>("AppService.SwitchInboundMode", mode),
  Connect: () =>
    callGo<{ warnings?: Array<{ code: string; message: string }> }>(
      "AppService.Connect",
    ),
  Disconnect: () => callGo<void>("AppService.Disconnect"),
  SetKillSwitch: (enabled: boolean) =>
    callGo<boolean>("AppService.SetKillSwitch", enabled),
  GetStats: () => callGo<TrafficStats>("AppService.GetStats"),
  GetLANInfo: () => callGo<LANInfo>("AppService.GetLANInfo"),
  GetBuiltinProbeURLs: () =>
    callGo<TestURLPreset[]>("AppService.GetBuiltinProbeURLs"),
  ProbeChain: () => callGo<ChainProbeResult>("AppService.ProbeChain"),
  ImportSubscription: (name: string, url: string, intervalHours: number) =>
    callGo<ExternalNode[]>(
      "AppService.ImportSubscription",
      name,
      url,
      intervalHours,
    ),
  TestAllNodes: () => callGo<SpeedTestResult[]>("AppService.TestAllNodes"),
  FetchGephExits: () => callGo<GephExitLocation[]>("AppService.FetchGephExits"),
  SelectGephExit: (country: string, city: string) =>
    callGo<void>("AppService.SelectGephExit", country, city),
  GetClassicCNApps: () => callGo<ClassicCNApp[]>("AppService.GetClassicCNApps"),
  DetectCNApps: (apps: InstalledApp[]) =>
    callGo<DetectedCNApp[]>("AppService.DetectCNApps", apps),
  GetBypassPresets: () =>
    callGo<BypassStrategy[]>("AppService.GetBypassPresets"),
  GetBypassCapabilityStatus: () =>
    callGo<BypassCapabilityStatus>("AppService.GetBypassCapabilityStatus"),
  AutoProbeBypass: (domain: string) =>
    callGo<AutoProbeReport>("AppService.AutoProbeBypass", domain),
  GetDomainBypassCache: () =>
    callGo<DomainCacheEntry[]>("AppService.GetDomainBypassCache"),
  LockDomainBypassStrategy: (domain: string, strategyId: string) =>
    callGo<void>("AppService.LockDomainBypassStrategy", domain, strategyId),
  ClearDomainBypassCache: (domain: string) =>
    callGo<void>("AppService.ClearDomainBypassCache", domain),
  ListInstalledApps: () =>
    callGo<InstalledApp[]>("AppService.ListInstalledApps"),
  OpenVpnSettings: () => callGo<boolean>("AppService.OpenVpnSettings"),
};
