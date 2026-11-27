export type RuntimePlatform =
  "android" | "linux" | "darwin" | "windows" | "unknown";

export interface PlatformUIProfile {
  platform: RuntimePlatform;
  isAndroid: boolean;
  isLinuxDesktop: boolean;
  tunModeLabel: string;
  tunBadgeLabel: string;
  showTrayOption: boolean;
  showSystemProxyOption: boolean;
  showPerAppProxy: boolean;
  showAndroidVpnShortcut: boolean;
  showLinuxRawFake: boolean;
  killSwitchMode: "toggle" | "android_shortcut";
}

export function normalizeRuntimePlatform(raw?: string | null): RuntimePlatform {
  const v = (raw ?? "").trim().toLowerCase();
  if (v === "android") return "android";
  if (v === "linux") return "linux";
  if (v === "darwin" || v === "macos") return "darwin";
  if (v === "windows") return "windows";
  return "unknown";
}

export function detectFallbackPlatform(): RuntimePlatform {
  if (typeof navigator === "undefined") return "linux";
  const ua = navigator.userAgent || "";
  if (/android/i.test(ua)) return "android";
  if (/linux/i.test(ua)) return "linux";
  if (/mac os x|macintosh/i.test(ua)) return "darwin";
  if (/windows/i.test(ua)) return "windows";
  return "unknown";
}

export function getPlatformUIProfile(
  platform: RuntimePlatform,
): PlatformUIProfile {
  const isAndroid = platform === "android";
  const isLinuxDesktop = platform === "linux" || platform === "unknown";
  return {
    platform,
    isAndroid,
    isLinuxDesktop,
    tunModeLabel: isAndroid ? "VPN 模式" : "全局 TUN",
    tunBadgeLabel: isAndroid ? "VPN 模式" : "TUN 模式",
    showTrayOption: isLinuxDesktop,
    showSystemProxyOption: isLinuxDesktop,
    showPerAppProxy: isAndroid,
    showAndroidVpnShortcut: isAndroid,
    showLinuxRawFake: isLinuxDesktop,
    killSwitchMode: isAndroid ? "android_shortcut" : "toggle",
  };
}
