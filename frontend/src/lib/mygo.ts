import type { ChainProbeResult, TestURLPreset } from "./probe";
import type { ClassicCNApp, DetectedCNApp, InstalledApp } from "./cn_apps";

// Typed client bridge matching MyGo bound AppService and AndroidNetPlugin methods.
declare global {
  interface Window {
    mygo?: {
      call: <T = unknown>(method: string, ...args: unknown[]) => Promise<T>;
    };
  }
}

export interface TrafficStats {
  running: boolean;
  up_bytes: number;
  down_bytes: number;
  mode: string;
  chain_desc: string;
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
  GetConfig: () => callGo<Record<string, any>>("AppService.GetConfig"),
  SaveConfig: (cfg: Record<string, any>) =>
    callGo<void>("AppService.SaveConfig", cfg),
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
  GetClassicCNApps: () => callGo<ClassicCNApp[]>("AppService.GetClassicCNApps"),
  DetectCNApps: (apps: InstalledApp[]) =>
    callGo<DetectedCNApp[]>("AppService.DetectCNApps", apps),
};

export const AndroidNetPlugin = {
  prepareAndStart: (args: Record<string, unknown>) =>
    callGo<{ started: boolean }>("AndroidNetPlugin.prepareAndStart", args),
  stop: () => callGo<{ stopped: boolean }>("AndroidNetPlugin.stop", {}),
  updateStats: (args: {
    up_bps: number;
    down_bps: number;
    chain_desc: string;
  }) => callGo<void>("AndroidNetPlugin.updateStats", args),
  listInstalledApps: () =>
    callGo<{ apps: InstalledApp[] }>("AndroidNetPlugin.listInstalledApps", {}),
  openVpnSettings: () =>
    callGo<{ opened: boolean }>("AndroidNetPlugin.openVpnSettings", {}),
};
