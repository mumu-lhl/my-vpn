export type HopKind =
  "geph" | "psiphon" | "warp" | "vpngate" | "external" | "direct_bypass";

export interface HopRef {
  kind: HopKind;
  node_id?: string;
}

export interface ChainConfig {
  two_hop: boolean;
  hop1: HopRef;
  hop2: HopRef;
}

export interface ChainAnalysis {
  label: string;
  autoSwitchWarpMasqueH2: boolean;
  warning: string | null;
}

export function analyzeChain(
  chain: ChainConfig,
  hop1SupportsUDP = true,
  vpngateSupportsUDP = true,
): ChainAnalysis {
  if (chain.hop1.kind === "direct_bypass") {
    return {
      label: formatHop(chain.hop1),
      autoSwitchWarpMasqueH2: false,
      warning: null,
    };
  }
  const hop1IsTcpOnly =
    chain.hop1.kind === "psiphon" ||
    (chain.hop1.kind === "vpngate" && !vpngateSupportsUDP) ||
    (chain.hop1.kind === "external" && !hop1SupportsUDP);
  if (!chain.two_hop) {
    return {
      label: formatHop(chain.hop1),
      autoSwitchWarpMasqueH2: false,
      warning: hop1IsTcpOnly
        ? "当前第一跳仅支持 TCP，非直连 UDP 流量将被拦截以防泄露。"
        : null,
    };
  }
  const autoSwitch = hop1IsTcpOnly && chain.hop2.kind === "warp";
  return {
    label: `${formatHop(chain.hop1)} → ${formatHop(chain.hop2)}`,
    autoSwitchWarpMasqueH2: autoSwitch,
    warning: autoSwitch
      ? "第一跳仅支持 TCP，系统已自动将第二跳 WARP 切换为 MASQUE (HTTP/2 over TLS/TCP) 模式。"
      : null,
  };
}

function formatHop(h: HopRef): string {
  switch (h.kind) {
    case "geph":
      return "迷雾通 (Geph5)";
    case "psiphon":
      return "赛风 (Psiphon)";
    case "warp":
      return "Cloudflare WARP";
    case "vpngate":
      return "VPNGate (OpenVPN)";
    case "external":
      return h.node_id ? `外部节点 (${h.node_id})` : "外部节点";
    case "direct_bypass":
      return "直连绕过（无服务器）";
  }
}

export function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}
