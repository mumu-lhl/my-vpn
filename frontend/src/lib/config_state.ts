import type { ChainConfig, HopKind } from "./chain";
import type { ExternalNode } from "./mygo";
import { BUILTIN_PROBE_URLS } from "./probe";

export interface AppFormState {
  inboundMode: "proxy" | "tun";
  allowLAN: boolean;
  killSwitch: boolean;
  socksPort: number;
  httpPort: number;
  chain: ChainConfig;
  gephSecret: string;
  gephCountry: string;
  psiphonRegion: string;
  warpMode: "auto" | "wg" | "awg" | "masque";
  warpLicense: string;
  warpNoiseMode: string;
  warpMasqueVhttp: "auto" | "h3" | "h2";
  bypassCN: boolean;
  bypassPrivate: boolean;
  blockAds: boolean;
  directBypass: boolean;
  fakeIP: boolean;
  directDNS: string;
  proxyDNS: string;
  probeMode: "builtin" | "custom";
  probeBuiltinURL: string;
  probeCustomURL: string;
  minimizeToTray: boolean;
  setSystemProxy: boolean;
  perAppMode: "all" | "whitelist" | "blacklist";
  excludedPackages: string[];
  externalNodes: ExternalNode[];
}

export function createDefaultFormState(): AppFormState {
  return {
    inboundMode: "proxy",
    allowLAN: false,
    killSwitch: false,
    socksPort: 10808,
    httpPort: 10809,
    chain: {
      two_hop: true,
      hop1: { kind: "geph" },
      hop2: { kind: "warp" },
    },
    gephSecret: "",
    gephCountry: "auto",
    psiphonRegion: "auto",
    warpMode: "auto",
    warpLicense: "",
    warpNoiseMode: "quic",
    warpMasqueVhttp: "auto",
    bypassCN: true,
    bypassPrivate: true,
    blockAds: true,
    directBypass: true,
    fakeIP: true,
    directDNS: "https://dns.alidns.com/dns-query",
    proxyDNS: "https://1.1.1.1/dns-query",
    probeMode: "builtin",
    probeBuiltinURL: BUILTIN_PROBE_URLS[0].url,
    probeCustomURL: "",
    minimizeToTray: true,
    setSystemProxy: true,
    perAppMode: "blacklist",
    excludedPackages: [],
    externalNodes: [],
  };
}

export function hydrateFormState(
  cfg: Record<string, any>,
  current: AppFormState = createDefaultFormState(),
): AppFormState {
  const next: AppFormState = {
    ...current,
    chain: {
      two_hop: current.chain.two_hop,
      hop1: { ...current.chain.hop1 },
      hop2: { ...current.chain.hop2 },
    },
    excludedPackages: [...current.excludedPackages],
    externalNodes: [...current.externalNodes],
  };

  if (cfg.inbound) {
    next.inboundMode = cfg.inbound.mode === "tun" ? "tun" : "proxy";
    next.socksPort = cfg.inbound.socks_port ?? 10808;
    next.httpPort = cfg.inbound.http_port ?? 10809;
    next.allowLAN = Boolean(cfg.inbound.allow_lan);
    next.killSwitch = Boolean(cfg.inbound.kill_switch);
  }
  if (cfg.chain) {
    next.chain = {
      two_hop: Boolean(cfg.chain.two_hop),
      hop1: {
        kind: (cfg.chain.hop1?.kind ?? "geph") as HopKind,
        node_id: cfg.chain.hop1?.node_id,
      },
      hop2: {
        kind: (cfg.chain.hop2?.kind ?? "warp") as HopKind,
        node_id: cfg.chain.hop2?.node_id,
      },
    };
  }
  if (cfg.geph) {
    next.gephSecret = cfg.geph.secret ?? "";
    next.gephCountry = cfg.geph.exit_country ?? "auto";
  }
  if (cfg.psiphon) {
    next.psiphonRegion = cfg.psiphon.egress_region ?? "auto";
  }
  if (cfg.warp) {
    next.warpMode = cfg.warp.mode ?? "auto";
    next.warpLicense = cfg.warp.license_key ?? "";
    next.warpNoiseMode = cfg.warp.noise_mode ?? "quic";
    next.warpMasqueVhttp = cfg.warp.masque?.vhttp ?? "auto";
  }
  if (cfg.routing) {
    next.bypassCN = Boolean(cfg.routing.bypass_cn);
    next.bypassPrivate = Boolean(cfg.routing.bypass_private ?? true);
    next.blockAds = Boolean(cfg.routing.block_ads);
    next.directBypass = Boolean(
      cfg.routing.direct_bypass?.enable_default_preset ?? true,
    );
  }
  if (cfg.dns) {
    next.directDNS = cfg.dns.direct_dns ?? next.directDNS;
    next.proxyDNS = cfg.dns.proxy_dns ?? next.proxyDNS;
    next.fakeIP = Boolean(cfg.dns.fake_ip ?? true);
  }
  if (cfg.probe) {
    next.probeMode = cfg.probe.mode === "custom" ? "custom" : "builtin";
    next.probeBuiltinURL = cfg.probe.test_url || BUILTIN_PROBE_URLS[0].url;
    next.probeCustomURL = cfg.probe.custom_url ?? "";
  }
  if (cfg.linux) {
    next.minimizeToTray = cfg.linux.minimize_to_tray ?? true;
    next.setSystemProxy = cfg.linux.set_system_proxy ?? true;
  }
  if (cfg.android) {
    next.perAppMode = cfg.android.per_app_mode ?? "blacklist";
    next.excludedPackages = Array.isArray(cfg.android.per_app_packages)
      ? cfg.android.per_app_packages
      : [];
  }
  if (Array.isArray(cfg.external_nodes)) {
    next.externalNodes = cfg.external_nodes;
  }
  return next;
}

export function buildConfigPayload(
  rawConfig: Record<string, any>,
  form: AppFormState,
): Record<string, any> {
  const base = structuredClone(rawConfig);
  return {
    ...base,
    inbound: {
      ...base.inbound,
      mode: form.inboundMode,
      socks_port: form.socksPort,
      http_port: form.httpPort,
      allow_lan: form.allowLAN,
      kill_switch: form.killSwitch,
    },
    chain: {
      two_hop: form.chain.two_hop,
      hop1: { ...form.chain.hop1 },
      hop2: { ...form.chain.hop2 },
    },
    geph: {
      ...base.geph,
      secret: form.gephSecret,
      exit_country: form.gephCountry,
    },
    psiphon: {
      ...base.psiphon,
      egress_region: form.psiphonRegion,
    },
    warp: {
      ...base.warp,
      mode: form.warpMode,
      license_key: form.warpLicense,
      noise_mode: form.warpNoiseMode,
      masque: {
        ...base.warp?.masque,
        vhttp: form.warpMasqueVhttp,
      },
    },
    routing: {
      ...base.routing,
      bypass_cn: form.bypassCN,
      bypass_private: form.bypassPrivate,
      block_ads: form.blockAds,
      direct_bypass: {
        ...base.routing?.direct_bypass,
        enable_default_preset: form.directBypass,
      },
    },
    dns: {
      ...base.dns,
      direct_dns: form.directDNS,
      proxy_dns: form.proxyDNS,
      fake_ip: form.fakeIP,
    },
    probe: {
      ...base.probe,
      mode: form.probeMode,
      test_url: form.probeBuiltinURL,
      custom_url: form.probeCustomURL.trim(),
    },
    linux: {
      ...base.linux,
      minimize_to_tray: form.minimizeToTray,
      set_system_proxy: form.setSystemProxy,
    },
    android: {
      ...base.android,
      per_app_mode: form.perAppMode,
      per_app_packages: form.excludedPackages,
    },
  };
}
