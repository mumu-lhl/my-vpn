import type { ChainConfig, HopKind } from "./chain";
import type { ExternalNode } from "./mygo";
import { BUILTIN_PROBE_URLS } from "./probe";

export type DirectBypassScope = "global" | "list";
export type QUICBypassMode = "fake" | "block" | "pass";

export interface AppFormState {
  inboundMode: "proxy" | "tun";
  allowLAN: boolean;
  killSwitch: boolean;
  socksPort: number;
  httpPort: number;
  chain: ChainConfig;
  gephSecret: string;
  gephCountry: string;
  gephCity: string;
  psiphonRegion: string;
  warpMode: "auto" | "wg" | "awg" | "masque";
  warpLicense: string;
  warpNoiseMode: string;
  warpMasqueVhttp: "auto" | "h3" | "h2";
  bypassCN: boolean;
  bypassPrivate: boolean;
  blockAds: boolean;
  directBypass: boolean;
  directBypassScope: DirectBypassScope;
  directBypassPresetID: string;
  directBypassSplitAtSNI: boolean;
  directBypassMixSNICase: boolean;
  directBypassSplitCount: number;
  directBypassFragmentSize: string;
  directBypassFragmentSleep: string;
  directBypassRecordFragment: boolean;
  directBypassEnableDisorder: boolean;
  directBypassEnableOOB: boolean;
  directBypassEnableFakeTTL: boolean;
  directBypassFakeTTL: number;
  directBypassEnableRawFake: boolean;
  directBypassRawBadSum: boolean;
  directBypassRawBadSeq: boolean;
  directBypassHttpMixHostCase: boolean;
  directBypassQUICMode: QUICBypassMode;
  directBypassEnableECH: boolean;
  directBypassAutoLearn: boolean;
  directBypassDoHURL: string;
  directBypassDoHStaticIPs: string;
  directBypassDoHDisableSNI: boolean;
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

/**
 * Safely deep-clones a plain or Svelte 5 `$state` Proxy record without calling
 * `structuredClone` directly (which throws DataCloneError on Proxy objects in WebView/Chromium).
 */
export function clonePlainRecord(
  input: Record<string, any> | null | undefined,
): Record<string, any> {
  if (!input || typeof input !== "object") {
    return {};
  }
  try {
    return JSON.parse(JSON.stringify(input)) as Record<string, any>;
  } catch {
    return {};
  }
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
    gephCity: "",
    psiphonRegion: "auto",
    warpMode: "auto",
    warpLicense: "",
    warpNoiseMode: "quic",
    warpMasqueVhttp: "auto",
    bypassCN: true,
    bypassPrivate: true,
    blockAds: true,
    directBypass: true,
    directBypassScope: "global",
    directBypassPresetID: "disorder_oob",
    directBypassSplitAtSNI: true,
    directBypassMixSNICase: true,
    directBypassSplitCount: 3,
    directBypassFragmentSize: "10-30",
    directBypassFragmentSleep: "2-8ms",
    directBypassRecordFragment: true,
    directBypassEnableDisorder: true,
    directBypassEnableOOB: true,
    directBypassEnableFakeTTL: false,
    directBypassFakeTTL: 4,
    directBypassEnableRawFake: false,
    directBypassRawBadSum: true,
    directBypassRawBadSeq: true,
    directBypassHttpMixHostCase: true,
    directBypassQUICMode: "fake",
    directBypassEnableECH: true,
    directBypassAutoLearn: true,
    directBypassDoHURL: "",
    directBypassDoHStaticIPs: "",
    directBypassDoHDisableSNI: true,
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
    const hop1Kind = (cfg.chain.hop1?.kind ?? "geph") as HopKind;
    next.chain = {
      two_hop:
        hop1Kind === "direct_bypass" ? false : Boolean(cfg.chain.two_hop),
      hop1: {
        kind: hop1Kind,
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
    next.gephCity = cfg.geph.exit_city ?? "";
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
    const db = cfg.routing.direct_bypass;
    next.directBypass = Boolean(db?.enable_default_preset ?? true);
    next.directBypassScope = db?.scope === "list" ? "list" : "global";
    next.directBypassPresetID = db?.preset_id || "disorder_oob";
    next.directBypassSplitAtSNI = Boolean(db?.split_at_sni ?? true);
    next.directBypassMixSNICase = Boolean(db?.mix_sni_case ?? true);
    next.directBypassSplitCount = Number(db?.split_count ?? 3) || 3;
    next.directBypassFragmentSize = db?.tls_fragment_size || "10-30";
    next.directBypassFragmentSleep = db?.tls_fragment_sleep || "2-8ms";
    next.directBypassRecordFragment = Boolean(db?.tls_record_fragment ?? true);
    next.directBypassEnableDisorder = Boolean(db?.enable_disorder ?? true);
    next.directBypassEnableOOB = Boolean(db?.enable_oob ?? true);
    next.directBypassEnableFakeTTL = Boolean(db?.enable_fake_ttl ?? false);
    next.directBypassFakeTTL = Number(db?.fake_ttl ?? 4) || 4;
    next.directBypassEnableRawFake = Boolean(db?.enable_raw_fake ?? false);
    next.directBypassRawBadSum = Boolean(db?.raw_bad_sum ?? true);
    next.directBypassRawBadSeq = Boolean(db?.raw_bad_seq ?? true);
    next.directBypassHttpMixHostCase = Boolean(db?.http_mix_host_case ?? true);
    next.directBypassQUICMode =
      db?.quic_mode === "block" || db?.quic_mode === "pass"
        ? db.quic_mode
        : "fake";
    next.directBypassEnableECH = Boolean(db?.enable_ech ?? true);
    next.directBypassAutoLearn = Boolean(db?.auto_learn_per_domain ?? true);
    next.directBypassDoHURL = db?.doh_url ?? "";
    next.directBypassDoHStaticIPs = Array.isArray(db?.doh_static_ips)
      ? db.doh_static_ips.join(", ")
      : "";
    next.directBypassDoHDisableSNI = Boolean(db?.doh_disable_sni ?? true);
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
  const base = clonePlainRecord(rawConfig);
  const dohStaticIPs = form.directBypassDoHStaticIPs
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
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
      two_hop:
        form.chain.hop1.kind === "direct_bypass" ? false : form.chain.two_hop,
      hop1: { ...form.chain.hop1 },
      hop2: { ...form.chain.hop2 },
    },
    geph: {
      ...base.geph,
      secret: form.gephSecret,
      exit_country: form.gephCountry,
      exit_city: form.gephCity,
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
        scope: form.directBypassScope,
        preset_id: form.directBypassPresetID,
        split_at_sni: form.directBypassSplitAtSNI,
        mix_sni_case: form.directBypassMixSNICase,
        split_count: Number(form.directBypassSplitCount) || 3,
        tls_fragment_size: form.directBypassFragmentSize.trim() || "10-30",
        tls_fragment_sleep: form.directBypassFragmentSleep.trim() || "2-8ms",
        tls_record_fragment: form.directBypassRecordFragment,
        enable_disorder: form.directBypassEnableDisorder,
        enable_oob: form.directBypassEnableOOB,
        enable_fake_ttl: form.directBypassEnableFakeTTL,
        fake_ttl: Number(form.directBypassFakeTTL) || 4,
        enable_raw_fake: form.directBypassEnableRawFake,
        raw_bad_sum: form.directBypassRawBadSum,
        raw_bad_seq: form.directBypassRawBadSeq,
        http_mix_host_case: form.directBypassHttpMixHostCase,
        quic_mode: form.directBypassQUICMode,
        enable_ech: form.directBypassEnableECH,
        auto_learn_per_domain: form.directBypassAutoLearn,
        doh_url: form.directBypassDoHURL.trim(),
        doh_static_ips: dohStaticIPs,
        doh_disable_sni: form.directBypassDoHDisableSNI,
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
      per_app_packages: [...form.excludedPackages],
    },
  };
}
