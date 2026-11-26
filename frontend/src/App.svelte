<script lang="ts">
  import { onMount } from "svelte";
  import {
    Activity,
    ArrowRightLeft,
    ExternalLink,
    Globe,
    Laptop,
    Moon,
    Power,
    RefreshCw,
    Route,
    Save,
    Settings,
    Shield,
    ShieldAlert,
    Sparkles,
    Sun,
    Trash2,
    Wifi,
    Zap,
  } from "@lucide/svelte";
  import {
    analyzeChain,
    formatBytes,
    type ChainConfig,
    type HopKind,
  } from "./lib/chain";
  import { applyTheme, type ThemeMode } from "./lib/theme";
  import {
    CLASSIC_CN_APPS,
    detectChinaApps,
    mergeUniquePackages,
    type DetectedCNApp,
  } from "./lib/cn_apps";
  import {
    BUILTIN_PROBE_URLS,
    countryCodeToFlag,
    formatLatency,
    validateProbeUrl,
    type ChainProbeResult,
    type TestURLPreset,
  } from "./lib/probe";
  import {
    AndroidNetPlugin,
    AppService,
    hasMyGoBridge,
    type ExternalNode,
    type GephExitLocation,
  } from "./lib/mygo";

  type Tab = "dashboard" | "chain" | "protocols" | "routing" | "settings";
  let activeTab = $state<Tab>("dashboard");
  let themeMode = $state<ThemeMode>("system");

  let bridgeReady = $state(false);
  let rawConfig = $state<Record<string, any>>({});
  let busyConnect = $state(false);
  let statusError = $state("");
  let statusNotice = $state("");
  let backendWarnings = $state<Array<{ code: string; message: string }>>([]);

  let connected = $state(false);
  let inboundMode = $state<"proxy" | "tun">("proxy");
  let allowLAN = $state(false);
  let killSwitch = $state(false);
  let socksPort = $state(10808);
  let httpPort = $state(10809);
  let lanIP = $state("127.0.0.1");
  let upBytes = $state(0);
  let downBytes = $state(0);

  let chain = $state<ChainConfig>({
    two_hop: true,
    hop1: { kind: "geph" },
    hop2: { kind: "warp" },
  });

  let gephSecret = $state("");
  let gephCountry = $state("auto");
  let gephExits = $state<GephExitLocation[]>([]);
  let psiphonRegion = $state("auto");
  let warpMode = $state<"auto" | "wg" | "awg" | "masque">("auto");
  let warpLicense = $state("");
  let warpNoiseMode = $state("quic");
  let warpMasqueVhttp = $state<"auto" | "h3" | "h2">("auto");

  let subName = $state("");
  let subURL = $state("");
  let externalNodes = $state<ExternalNode[]>([]);
  let testingNodes = $state(false);

  let bypassCN = $state(true);
  let blockAds = $state(true);
  let directBypass = $state(true);
  let fakeIP = $state(true);
  let directDNS = $state("https://dns.alidns.com/dns-query");
  let proxyDNS = $state("https://1.1.1.1/dns-query");

  let probeMode = $state<"builtin" | "custom">("builtin");
  let probeBuiltinURL = $state(BUILTIN_PROBE_URLS[0].url);
  let probeCustomURL = $state("");
  let builtinProbePresets = $state<TestURLPreset[]>(BUILTIN_PROBE_URLS);
  let probing = $state(false);
  let probeResult = $state<ChainProbeResult | null>(null);

  let minimizeToTray = $state(true);
  let setSystemProxy = $state(true);
  let perAppMode = $state<"all" | "whitelist" | "blacklist">("blacklist");
  let excludedPackages = $state<string[]>([]);
  let detectedPreview = $state<DetectedCNApp[]>([]);
  let detectNotice = $state("");

  let chainInfo = $derived(analyzeChain(chain));
  let activeProbeURL = $derived(
    probeMode === "custom" && probeCustomURL.trim()
      ? probeCustomURL.trim()
      : probeBuiltinURL,
  );
  let customProbeValid = $derived(
    probeMode !== "custom" || validateProbeUrl(probeCustomURL),
  );

  function buildConfigPayload(): Record<string, any> {
    const base = structuredClone(rawConfig);
    return {
      ...base,
      inbound: {
        ...base.inbound,
        mode: inboundMode,
        socks_port: socksPort,
        http_port: httpPort,
        allow_lan: allowLAN,
        kill_switch: killSwitch,
      },
      chain: {
        two_hop: chain.two_hop,
        hop1: { ...chain.hop1 },
        hop2: { ...chain.hop2 },
      },
      geph: {
        ...base.geph,
        secret: gephSecret,
        exit_country: gephCountry,
      },
      psiphon: {
        ...base.psiphon,
        egress_region: psiphonRegion,
      },
      warp: {
        ...base.warp,
        mode: warpMode,
        license_key: warpLicense,
        noise_mode: warpNoiseMode,
        masque: {
          ...base.warp?.masque,
          vhttp: warpMasqueVhttp,
        },
      },
      routing: {
        ...base.routing,
        bypass_cn: bypassCN,
        block_ads: blockAds,
        direct_bypass: {
          ...base.routing?.direct_bypass,
          enable_default_preset: directBypass,
        },
      },
      dns: {
        ...base.dns,
        direct_dns: directDNS,
        proxy_dns: proxyDNS,
        fake_ip: fakeIP,
      },
      probe: {
        ...base.probe,
        mode: probeMode,
        test_url: probeBuiltinURL,
        custom_url: probeCustomURL.trim(),
      },
      linux: {
        ...base.linux,
        minimize_to_tray: minimizeToTray,
        set_system_proxy: setSystemProxy,
      },
      android: {
        ...base.android,
        per_app_mode: perAppMode,
        per_app_packages: excludedPackages,
      },
    };
  }

  function hydrateFromConfig(cfg: Record<string, any>) {
    rawConfig = cfg;
    if (cfg.inbound) {
      inboundMode = cfg.inbound.mode === "tun" ? "tun" : "proxy";
      socksPort = cfg.inbound.socks_port ?? 10808;
      httpPort = cfg.inbound.http_port ?? 10809;
      allowLAN = Boolean(cfg.inbound.allow_lan);
      killSwitch = Boolean(cfg.inbound.kill_switch);
    }
    if (cfg.chain) {
      chain = {
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
      gephSecret = cfg.geph.secret ?? "";
      gephCountry = cfg.geph.exit_country ?? "auto";
    }
    if (cfg.psiphon) {
      psiphonRegion = cfg.psiphon.egress_region ?? "auto";
    }
    if (cfg.warp) {
      warpMode = cfg.warp.mode ?? "auto";
      warpLicense = cfg.warp.license_key ?? "";
      warpNoiseMode = cfg.warp.noise_mode ?? "quic";
      warpMasqueVhttp = cfg.warp.masque?.vhttp ?? "auto";
    }
    if (cfg.routing) {
      bypassCN = Boolean(cfg.routing.bypass_cn);
      blockAds = Boolean(cfg.routing.block_ads);
      directBypass = Boolean(
        cfg.routing.direct_bypass?.enable_default_preset ?? true,
      );
    }
    if (cfg.dns) {
      directDNS = cfg.dns.direct_dns ?? directDNS;
      proxyDNS = cfg.dns.proxy_dns ?? proxyDNS;
      fakeIP = Boolean(cfg.dns.fake_ip ?? true);
    }
    if (cfg.probe) {
      probeMode = cfg.probe.mode === "custom" ? "custom" : "builtin";
      probeBuiltinURL = cfg.probe.test_url || BUILTIN_PROBE_URLS[0].url;
      probeCustomURL = cfg.probe.custom_url ?? "";
    }
    if (cfg.linux) {
      minimizeToTray = cfg.linux.minimize_to_tray ?? true;
      setSystemProxy = cfg.linux.set_system_proxy ?? true;
    }
    if (cfg.android) {
      perAppMode = cfg.android.per_app_mode ?? "blacklist";
      excludedPackages = Array.isArray(cfg.android.per_app_packages)
        ? cfg.android.per_app_packages
        : [];
    }
    if (Array.isArray(cfg.external_nodes)) {
      externalNodes = cfg.external_nodes;
    }
  }

  async function persistConfig(showNotice = false): Promise<boolean> {
    if (!customProbeValid) {
      statusError = "自定义连通性检测链接必须以 http:// 或 https:// 开头";
      return false;
    }
    statusError = "";
    const payload = buildConfigPayload();
    rawConfig = payload;
    if (!bridgeReady) {
      if (showNotice) {
        statusNotice = "配置已在本地预览状态更新";
      }
      return true;
    }
    try {
      await AppService.SaveConfig(payload);
      if (showNotice) {
        statusNotice = "配置已保存";
      }
      return true;
    } catch (err) {
      statusError = `保存配置失败：${err instanceof Error ? err.message : String(err)}`;
      return false;
    }
  }

  async function runProbe() {
    if (!(await persistConfig(false))) {
      return;
    }
    probing = true;
    statusError = "";
    try {
      if (bridgeReady) {
        probeResult = await AppService.ProbeChain();
      } else {
        probeResult = {
          test_url: activeProbeURL,
          hops: chain.two_hop
            ? [
                {
                  hop_index: 1,
                  label: `第一跳 (${chain.hop1.kind})`,
                  kind: chain.hop1.kind,
                  connectivity: {
                    url: activeProbeURL,
                    reachable: false,
                    latency_ms: 0,
                    error: "浏览器预览模式未连接后端",
                  },
                  exit_ip: {
                    ip: "--",
                    country_code: "",
                    flag: countryCodeToFlag(""),
                    error: "需在桌面/Android 运行时检测",
                  },
                },
                {
                  hop_index: 2,
                  label: `第二跳 (${chain.hop2.kind})`,
                  kind: chain.hop2.kind,
                  connectivity: {
                    url: activeProbeURL,
                    reachable: false,
                    latency_ms: 0,
                    error: "浏览器预览模式未连接后端",
                  },
                  exit_ip: {
                    ip: "--",
                    country_code: "",
                    flag: countryCodeToFlag(""),
                    error: "需在桌面/Android 运行时检测",
                  },
                },
              ]
            : [
                {
                  hop_index: 1,
                  label: `第一跳 (${chain.hop1.kind})`,
                  kind: chain.hop1.kind,
                  connectivity: {
                    url: activeProbeURL,
                    reachable: false,
                    latency_ms: 0,
                    error: "浏览器预览模式未连接后端",
                  },
                  exit_ip: {
                    ip: "--",
                    country_code: "",
                    flag: countryCodeToFlag(""),
                    error: "需在桌面/Android 运行时检测",
                  },
                },
              ],
        };
      }
    } catch (err) {
      statusError = `连通性检测失败：${err instanceof Error ? err.message : String(err)}`;
    } finally {
      probing = false;
    }
  }

  async function toggleConnect() {
    if (busyConnect) return;
    busyConnect = true;
    statusError = "";
    statusNotice = "";
    try {
      if (!bridgeReady) {
        connected = !connected;
        return;
      }
      if (connected) {
        await AppService.Disconnect();
        connected = false;
        backendWarnings = [];
      } else {
        if (!(await persistConfig(false))) {
          return;
        }
        const res = await AppService.Connect();
        backendWarnings = res?.warnings ?? [];
        connected = true;
        const lan = await AppService.GetLANInfo();
        lanIP = lan.ip;
        void runProbe();
      }
    } catch (err) {
      statusError = `操作失败：${err instanceof Error ? err.message : String(err)}`;
    } finally {
      busyConnect = false;
    }
  }

  async function onToggleKillSwitch() {
    if (!bridgeReady) return;
    try {
      await AppService.SetKillSwitch(killSwitch);
    } catch (err) {
      statusError = `Kill Switch 切换失败：${err instanceof Error ? err.message : String(err)}`;
    }
  }

  async function loadGephExits() {
    if (!bridgeReady) {
      statusNotice = "需在桌面/Android 运行时从 geph5 control RPC 获取出口列表";
      return;
    }
    try {
      gephExits = await AppService.FetchGephExits();
      statusNotice = `已获取 ${gephExits.length} 个 Geph 出口`;
    } catch (err) {
      statusError = `获取 Geph 出口失败：${err instanceof Error ? err.message : String(err)}`;
    }
  }

  async function importSub() {
    if (!subURL.trim()) {
      statusError = "请输入订阅链接";
      return;
    }
    if (!bridgeReady) {
      statusNotice = "需在桌面/Android 运行时拉取订阅";
      return;
    }
    try {
      const nodes = await AppService.ImportSubscription(
        subName.trim() || "订阅",
        subURL.trim(),
        24,
      );
      externalNodes = [...externalNodes, ...nodes];
      subName = "";
      subURL = "";
      statusNotice = `成功导入 ${nodes.length} 个节点`;
    } catch (err) {
      statusError = `导入订阅失败：${err instanceof Error ? err.message : String(err)}`;
    }
  }

  async function testAllNodes() {
    if (!bridgeReady) return;
    testingNodes = true;
    try {
      const results = await AppService.TestAllNodes();
      const map = new Map(results.map((r) => [r.node_id, r.latency_ms]));
      externalNodes = externalNodes.map((n) => ({
        ...n,
        latency_ms: map.get(n.id) ?? n.latency_ms,
      }));
    } catch (err) {
      statusError = `测速失败：${err instanceof Error ? err.message : String(err)}`;
    } finally {
      testingNodes = false;
    }
  }

  async function addClassicCNApps() {
    perAppMode = "blacklist";
    excludedPackages = mergeUniquePackages(
      excludedPackages,
      CLASSIC_CN_APPS.map((a) => a.package_name),
    );
    detectNotice = `已一键添加 ${CLASSIC_CN_APPS.length} 个经典中国软件（微信、QQ、小红书等）到排除列表`;
    await persistConfig(false);
  }

  async function autoDetectCNApps() {
    perAppMode = "blacklist";
    try {
      if (bridgeReady) {
        const res = await AndroidNetPlugin.listInstalledApps();
        const detected = await AppService.DetectCNApps(res.apps ?? []);
        detectedPreview = detected;
        excludedPackages = mergeUniquePackages(
          excludedPackages,
          detected.map((d) => d.package_name),
        );
        detectNotice = `已扫描本机 ${res.apps?.length ?? 0} 个已安装应用，自动识别出 ${detected.length} 个中国软件并加入黑名单`;
        await persistConfig(false);
        return;
      }
    } catch {
      // Desktop Linux or browser preview without Android PackageManager falls back gracefully below.
    }
    const detected = detectChinaApps(
      CLASSIC_CN_APPS.slice(0, 8).map((a) => ({
        package_name: a.package_name,
        label: a.name,
      })),
    );
    detectedPreview = detected;
    excludedPackages = mergeUniquePackages(
      excludedPackages,
      detected.map((d) => d.package_name),
    );
    detectNotice = `非 Android 环境：已按内置规则预览识别 ${detected.length} 个中国软件并加入黑名单`;
    await persistConfig(false);
  }

  async function openAndroidVpnSettings() {
    if (!bridgeReady) {
      statusNotice = "仅支持在 Android 设备上跳转系统设置";
      return;
    }
    try {
      await AndroidNetPlugin.openVpnSettings();
    } catch (err) {
      statusError = `打开系统设置失败：${err instanceof Error ? err.message : String(err)}`;
    }
  }

  async function removeExcludedPackage(pkg: string) {
    excludedPackages = excludedPackages.filter((p) => p !== pkg);
    await persistConfig(false);
  }

  $effect(() => {
    applyTheme(themeMode);
  });

  onMount(() => {
    applyTheme(themeMode);
    bridgeReady = hasMyGoBridge();

    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (themeMode === "system") {
        applyTheme("system");
      }
    };
    mq.addEventListener("change", onChange);

    if (bridgeReady) {
      void (async () => {
        try {
          const [cfg, stats, lan, presets] = await Promise.all([
            AppService.GetConfig(),
            AppService.GetStats(),
            AppService.GetLANInfo(),
            AppService.GetBuiltinProbeURLs(),
          ]);
          hydrateFromConfig(cfg);
          connected = stats.running;
          upBytes = stats.up_bytes;
          downBytes = stats.down_bytes;
          lanIP = lan.ip;
          if (Array.isArray(presets) && presets.length > 0) {
            builtinProbePresets = presets;
          }
        } catch (err) {
          statusError = `初始化加载失败：${err instanceof Error ? err.message : String(err)}`;
        }
      })();
    }

    const timer = window.setInterval(() => {
      if (!bridgeReady || !connected) return;
      void AppService.GetStats()
        .then((st) => {
          connected = st.running;
          upBytes = st.up_bytes;
          downBytes = st.down_bytes;
        })
        .catch(() => {});
    }, 1000);

    return () => {
      mq.removeEventListener("change", onChange);
      window.clearInterval(timer);
    };
  });

  function setHop(which: "hop1" | "hop2", kind: string) {
    chain = { ...chain, [which]: { kind: kind as HopKind } };
  }

  function setTheme(mode: ThemeMode) {
    themeMode = mode;
  }
</script>

<div
  class="flex min-h-screen flex-col-reverse bg-slate-50 text-slate-900 transition-colors dark:bg-slate-950 dark:text-slate-100 md:flex-row"
>
  <aside
    class="sticky bottom-0 z-10 flex w-full shrink-0 flex-row justify-around border-t border-slate-200 bg-white/95 p-2 backdrop-blur dark:border-slate-800 dark:bg-slate-900/95 md:static md:w-56 md:flex-col md:justify-start md:gap-1.5 md:border-r md:border-t-0 md:p-4"
  >
    <div
      class="mb-4 hidden items-center gap-2 px-2 text-lg font-bold text-sky-600 dark:text-sky-400 md:flex"
    >
      <Shield class="h-5 w-5" />
      <span>Looma</span>
    </div>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab ===
      'dashboard'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "dashboard")}
    >
      <Activity class="h-4 w-4" />
      <span>仪表盘</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab ===
      'chain'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "chain")}
    >
      <ArrowRightLeft class="h-4 w-4" />
      <span>链路</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab ===
      'protocols'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "protocols")}
    >
      <Globe class="h-4 w-4" />
      <span>协议</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab ===
      'routing'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "routing")}
    >
      <Route class="h-4 w-4" />
      <span>路由</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab ===
      'settings'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "settings")}
    >
      <Settings class="h-4 w-4" />
      <span>设置</span>
    </button>

    <div class="mt-auto hidden pt-4 md:block">
      <div
        class="grid grid-cols-3 gap-1 rounded-xl bg-slate-100 p-1 dark:bg-slate-800"
      >
        <button
          title="白天模式"
          class="flex items-center justify-center rounded-lg py-1.5 transition {themeMode ===
          'light'
            ? 'bg-white text-amber-500 shadow-sm dark:bg-slate-700'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100'}"
          onclick={() => setTheme("light")}
        >
          <Sun class="h-4 w-4" />
        </button>
        <button
          title="黑夜模式"
          class="flex items-center justify-center rounded-lg py-1.5 transition {themeMode ===
          'dark'
            ? 'bg-white text-sky-500 shadow-sm dark:bg-slate-700'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100'}"
          onclick={() => setTheme("dark")}
        >
          <Moon class="h-4 w-4" />
        </button>
        <button
          title="跟随系统"
          class="flex items-center justify-center rounded-lg py-1.5 transition {themeMode ===
          'system'
            ? 'bg-white text-emerald-600 shadow-sm dark:bg-slate-700 dark:text-emerald-400'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100'}"
          onclick={() => setTheme("system")}
        >
          <Laptop class="h-4 w-4" />
        </button>
      </div>
    </div>
  </aside>

  <main class="flex-1 space-y-4 p-4 md:p-6">
    {#if statusError}
      <div
        class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-2.5 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/60 dark:text-rose-300"
      >
        {statusError}
      </div>
    {/if}
    {#if statusNotice}
      <div
        class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-sm text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/60 dark:text-emerald-300"
      >
        {statusNotice}
      </div>
    {/if}

    {#if activeTab === "dashboard"}
      <section
        class="flex flex-col justify-between gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:flex-row sm:items-center"
      >
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <span
              class="inline-block h-2.5 w-2.5 rounded-full {connected
                ? 'bg-emerald-500'
                : 'bg-slate-400'}"
            ></span>
            <h1 class="text-xl font-bold">{connected ? "已连接" : "未连接"}</h1>
          </div>
          <p class="text-sm text-slate-500 dark:text-slate-400">
            当前链路：{chainInfo.label}
          </p>
          {#if chainInfo.warning}
            <p
              class="flex items-center gap-1.5 text-sm text-amber-600 dark:text-amber-400"
            >
              <ShieldAlert class="h-4 w-4 shrink-0" />
              <span>{chainInfo.warning}</span>
            </p>
          {/if}
          {#each backendWarnings as w}
            <p
              class="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400"
            >
              <ShieldAlert class="h-3.5 w-3.5 shrink-0" />
              <span>[{w.code}] {w.message}</span>
            </p>
          {/each}
        </div>
        <button
          disabled={busyConnect}
          class="inline-flex items-center justify-center gap-2 rounded-xl px-5 py-3 text-sm font-semibold text-white shadow-sm transition disabled:opacity-60 {connected
            ? 'bg-rose-600 hover:bg-rose-700'
            : 'bg-sky-600 hover:bg-sky-700'}"
          onclick={toggleConnect}
        >
          <Power class="h-4 w-4" />
          <span>{connected ? "断开连接" : "一键连接"}</span>
        </button>
      </section>

      <section
        class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div
          class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center"
        >
          <div>
            <h3 class="flex items-center gap-2 font-semibold">
              <Globe class="h-4 w-4 text-sky-500" />
              <span>逐跳连通性与出口信息</span>
            </h3>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              检测链接：{activeProbeURL}
            </p>
          </div>
          <button
            disabled={probing}
            class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3.5 py-2 text-xs font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:opacity-60"
            onclick={runProbe}
          >
            <RefreshCw class="h-3.5 w-3.5 {probing ? 'animate-spin' : ''}" />
            <span>{probing ? "检测中..." : "立即检测每跳"}</span>
          </button>
        </div>

        {#if probeResult && probeResult.hops.length > 0}
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
            {#each probeResult.hops as hop}
              <div
                class="space-y-1.5 rounded-xl border border-slate-200 bg-slate-50/70 p-3.5 dark:border-slate-800 dark:bg-slate-950/60"
              >
                <div class="flex items-center justify-between gap-2">
                  <span class="text-sm font-semibold">{hop.label}</span>
                  <span
                    class="rounded-full px-2.5 py-0.5 text-xs font-medium {hop
                      .connectivity.reachable
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                      : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-300'}"
                  >
                    {formatLatency(hop.connectivity)}
                  </span>
                </div>
                <div class="flex flex-wrap items-center gap-2 text-sm">
                  <span class="text-lg leading-none"
                    >{hop.exit_ip.flag || "🌐"}</span
                  >
                  <span class="font-mono font-semibold"
                    >{hop.exit_ip.country_code || "--"}</span
                  >
                  <span class="font-mono text-slate-600 dark:text-slate-300"
                    >{hop.exit_ip.ip || "--"}</span
                  >
                </div>
                {#if hop.exit_ip.city || hop.exit_ip.org}
                  <p class="text-xs text-slate-500 dark:text-slate-400">
                    {[hop.exit_ip.city, hop.exit_ip.org]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                {:else if hop.exit_ip.error}
                  <p class="text-xs text-amber-600 dark:text-amber-400">
                    {hop.exit_ip.error}
                  </p>
                {/if}
              </div>
            {/each}
          </div>
        {:else}
          <p class="text-xs text-slate-500 dark:text-slate-400">
            点击“立即检测每跳”查看{chain.two_hop
              ? "第一跳与第二跳"
              : "当前出口"}的连通性延迟、国家国旗、二字编码及出口 IP。
          </p>
        {/if}
      </section>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div
          class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
        >
          <h3 class="flex items-center gap-2 font-semibold">
            <Wifi class="h-4 w-4 text-sky-500" />
            <span>出口模式</span>
          </h3>
          <label class="flex items-center gap-2 text-sm">
            <input type="radio" bind:group={inboundMode} value="proxy" />
            <span>本地端口 (SOCKS5:{socksPort} / HTTP:{httpPort})</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input type="radio" bind:group={inboundMode} value="tun" />
            <span>全局接管 (TUN)</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" bind:checked={allowLAN} />
            <span
              >允许局域网共享 ({allowLAN
                ? `${lanIP}:${socksPort}`
                : "0.0.0.0"})</span
            >
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              bind:checked={killSwitch}
              onchange={onToggleKillSwitch}
            />
            <span>Kill Switch（断网保护：异常断开时阻断明文直连泄露）</span>
          </label>
        </div>

        <div
          class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
        >
          <h3 class="flex items-center gap-2 font-semibold">
            <Activity class="h-4 w-4 text-emerald-500" />
            <span>实时流量</span>
          </h3>
          <p class="text-sm text-slate-600 dark:text-slate-300">
            上行：{formatBytes(upBytes)}
          </p>
          <p class="text-sm text-slate-600 dark:text-slate-300">
            下行：{formatBytes(downBytes)}
          </p>
        </div>
      </section>
    {:else if activeTab === "chain"}
      <section
        class="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold">一级 / 二级链路编排</h2>
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-700"
            onclick={() => persistConfig(true)}
          >
            <Save class="h-3.5 w-3.5" />
            <span>保存</span>
          </button>
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={chain.two_hop} />
          <span>启用二级链路（前置破壁 + 落地干净出口）</span>
        </label>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-1.5 text-sm">
            <span class="font-medium">第一跳（破壁）</span>
            <select
              class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
              value={chain.hop1.kind}
              onchange={(e) => setHop("hop1", e.currentTarget.value)}
            >
              <option value="geph">迷雾通 (Geph5)</option>
              <option value="psiphon">赛风 (Psiphon · 仅 TCP)</option>
              <option value="warp">Cloudflare WARP</option>
              <option value="external">外部订阅/自建节点</option>
            </select>
          </label>
          {#if chain.two_hop}
            <label class="flex flex-col gap-1.5 text-sm">
              <span class="font-medium">第二跳（落地）</span>
              <select
                class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
                value={chain.hop2.kind}
                onchange={(e) => setHop("hop2", e.currentTarget.value)}
              >
                <option value="warp">Cloudflare WARP</option>
                <option value="geph">迷雾通 (Geph5)</option>
                <option value="psiphon">赛风 (Psiphon)</option>
                <option value="external">外部订阅/自建节点</option>
              </select>
            </label>
          {/if}
        </div>
        {#if chainInfo.warning}
          <p
            class="flex items-center gap-1.5 text-sm text-amber-600 dark:text-amber-400"
          >
            <ShieldAlert class="h-4 w-4 shrink-0" />
            <span>{chainInfo.warning}</span>
          </p>
        {/if}
      </section>
    {:else if activeTab === "protocols"}
      <section
        class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold">迷雾通 (Geph5)</h2>
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-slate-100 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-200"
            onclick={loadGephExits}
          >
            <RefreshCw class="h-3.5 w-3.5" />
            <span>拉取出口列表</span>
          </button>
        </div>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>账号 Secret</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="password"
            bind:value={gephSecret}
            placeholder="粘贴已有 Geph secret"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>出口国家</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={gephCountry}
            placeholder="auto / JP / US / SG"
          />
        </label>
        {#if gephExits.length > 0}
          <div class="flex flex-wrap gap-1.5 pt-1">
            {#each gephExits as ex}
              <button
                class="rounded-lg bg-slate-100 px-2.5 py-1 text-xs hover:bg-sky-100 dark:bg-slate-800 dark:hover:bg-sky-900"
                onclick={() => (gephCountry = ex.country)}
              >
                {countryCodeToFlag(ex.country)}
                {ex.country} · {ex.city}
              </button>
            {/each}
          </div>
        {/if}
      </section>

      <section
        class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h2 class="text-lg font-semibold">
          赛风 (Psiphon) &amp; Cloudflare WARP
        </h2>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>Psiphon 出口区域</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={psiphonRegion}
            placeholder="auto / US / JP / DE"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>WARP 隧道模式</span>
          <select
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            bind:value={warpMode}
          >
            <option value="auto">Auto</option>
            <option value="wg">WireGuard</option>
            <option value="awg">AmneziaWG</option>
            <option value="masque">MASQUE (HTTP/3 &amp; HTTP/2)</option>
          </select>
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>WARP+ License Key</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={warpLicense}
            placeholder="留空自动免费注册"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>WireGuard 噪声模式</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={warpNoiseMode}
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>MASQUE HTTP 版本</span>
          <select
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            bind:value={warpMasqueVhttp}
          >
            <option value="auto">Auto (H3 优先，3s 回退 H2)</option>
            <option value="h3">HTTP/3 (QUIC)</option>
            <option value="h2">HTTP/2 (TLS/TCP)</option>
          </select>
        </label>
        <div class="pt-1">
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3.5 py-2 text-xs font-medium text-white hover:bg-sky-700"
            onclick={() => persistConfig(true)}
          >
            <Save class="h-3.5 w-3.5" />
            <span>保存协议配置</span>
          </button>
        </div>
      </section>

      <section
        class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold">外部订阅与节点测速</h2>
          {#if externalNodes.length > 0}
            <button
              disabled={testingNodes}
              class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-700 disabled:opacity-60"
              onclick={testAllNodes}
            >
              <RefreshCw
                class="h-3.5 w-3.5 {testingNodes ? 'animate-spin' : ''}"
              />
              <span>并发测速</span>
            </button>
          {/if}
        </div>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={subName}
            placeholder="订阅名称"
          />
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-950 sm:col-span-2"
            type="url"
            bind:value={subURL}
            placeholder="订阅链接 (sing-box / Clash / Base64 / 分享链接)"
          />
        </div>
        <button
          class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3.5 py-2 text-xs font-medium text-white hover:bg-sky-700"
          onclick={importSub}
        >
          <span>导入订阅</span>
        </button>
        {#if externalNodes.length > 0}
          <div class="space-y-1 text-xs">
            {#each externalNodes as node}
              <div
                class="flex items-center justify-between rounded-lg bg-slate-100 px-3 py-1.5 dark:bg-slate-800"
              >
                <span>{node.name} ({node.protocol})</span>
                <span class="font-mono text-slate-500 dark:text-slate-400">
                  {node.latency_ms ? `${node.latency_ms} ms` : "--"}
                </span>
              </div>
            {/each}
          </div>
        {/if}
      </section>
    {:else if activeTab === "routing"}
      <section
        class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold">路由预设与 DNS 防泄露</h2>
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-700"
            onclick={() => persistConfig(true)}
          >
            <Save class="h-3.5 w-3.5" />
            <span>保存</span>
          </button>
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={bypassCN} />
          <span>中国大陆 IP 与域名直连 (geoip-cn / geosite-cn)</span>
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={blockAds} />
          <span>拦截广告与跟踪器 (geosite-category-ads-all)</span>
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={directBypass} />
          <span
            >启用轻量直连破壁 (direct-bypass · TLS 分片直连 Pixiv / E-Hentai /
            GitHub 等)</span
          >
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={fakeIP} />
          <span>开启 FakeIP（TUN 模式下防 DNS 泄露）</span>
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>国内直连 DNS</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={directDNS}
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>海外代理 DNS</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={proxyDNS}
          />
        </label>
      </section>
    {:else}
      <section
        class="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold">
              连通性检测链接（参考 Hiddify）
            </h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              选择内置检测链接或输入自定义
              URL，多跳模式下会逐跳检测延迟并显示国旗与出口 IP
            </p>
          </div>
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-700"
            onclick={() => persistConfig(true)}
          >
            <Save class="h-3.5 w-3.5" />
            <span>保存</span>
          </button>
        </div>

        <div class="flex flex-wrap gap-4 text-sm">
          <label class="flex items-center gap-2">
            <input type="radio" bind:group={probeMode} value="builtin" />
            <span>内置检测链接</span>
          </label>
          <label class="flex items-center gap-2">
            <input type="radio" bind:group={probeMode} value="custom" />
            <span>自定义链接</span>
          </label>
        </div>

        {#if probeMode === "builtin"}
          <label class="flex flex-col gap-1.5 text-sm">
            <span>选择内置检测地址</span>
            <select
              class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
              bind:value={probeBuiltinURL}
            >
              {#each builtinProbePresets as preset}
                <option value={preset.url}>
                  {preset.label} — {preset.url}
                </option>
              {/each}
            </select>
          </label>
        {:else}
          <label class="flex flex-col gap-1.5 text-sm">
            <span>自定义检测 URL</span>
            <input
              class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
              type="url"
              bind:value={probeCustomURL}
              placeholder="https://cp.cloudflare.com/generate_204"
            />
          </label>
          {#if !customProbeValid}
            <p class="text-xs text-rose-600 dark:text-rose-400">
              请输入合法的 http:// 或 https:// 链接
            </p>
          {/if}
        {/if}
      </section>

      <section
        class="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h2 class="text-lg font-semibold">外观主题</h2>
        <div class="grid grid-cols-3 gap-2">
          <button
            class="flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition {themeMode ===
            'light'
              ? 'border-sky-600 bg-sky-50 text-sky-700 dark:bg-sky-950 dark:text-sky-300'
              : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800'}"
            onclick={() => setTheme("light")}
          >
            <Sun class="h-4 w-4" />
            <span>白天</span>
          </button>
          <button
            class="flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition {themeMode ===
            'dark'
              ? 'border-sky-600 bg-sky-50 text-sky-700 dark:bg-sky-950 dark:text-sky-300'
              : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800'}"
            onclick={() => setTheme("dark")}
          >
            <Moon class="h-4 w-4" />
            <span>黑夜</span>
          </button>
          <button
            class="flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition {themeMode ===
            'system'
              ? 'border-sky-600 bg-sky-50 text-sky-700 dark:bg-sky-950 dark:text-sky-300'
              : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800'}"
            onclick={() => setTheme("system")}
          >
            <Laptop class="h-4 w-4" />
            <span>跟随系统</span>
          </button>
        </div>

        <h2 class="pt-2 text-lg font-semibold">平台与系统设置</h2>
        <div
          class="space-y-2.5 rounded-xl border border-slate-200 p-3.5 dark:border-slate-800"
        >
          <p class="text-xs font-medium text-slate-500 dark:text-slate-400">
            Linux (niri / Wayland)
          </p>
          <label class="flex items-center justify-between gap-3 text-sm">
            <span>关闭窗口时隐藏至 SNI 系统托盘（后台保持代理运行）</span>
            <input
              type="checkbox"
              class="h-4 w-4 accent-sky-600"
              bind:checked={minimizeToTray}
              onchange={() => persistConfig(false)}
            />
          </label>
          <label class="flex items-center justify-between gap-3 text-sm">
            <span>代理模式下自动写入 GNOME / GTK 系统代理</span>
            <input
              type="checkbox"
              class="h-4 w-4 accent-sky-600"
              bind:checked={setSystemProxy}
              onchange={() => persistConfig(false)}
            />
          </label>
        </div>
        <p class="text-sm text-slate-600 dark:text-slate-300">
          Android 10+ (arm64-v8a ·
          io.github.mumulhl.looma)：支持分应用白/黑名单、局域网共享与前台速率通知栏。
        </p>
        <button
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 px-3.5 py-2 text-xs font-medium hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          onclick={openAndroidVpnSettings}
        >
          <ExternalLink class="h-3.5 w-3.5" />
          <span>打开 Android 系统始终开启 / 无连接时阻断设置</span>
        </button>
      </section>

      <section
        class="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <div
          class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center"
        >
          <div>
            <h2 class="text-lg font-semibold">Android 分应用排除</h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              快速排除微信、QQ、小红书等经典中国软件，或按包名/商店来源/中文名自动识别
            </p>
          </div>
          <select
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-950"
            bind:value={perAppMode}
          >
            <option value="all">全局接管所有应用</option>
            <option value="blacklist">排除选中应用（黑名单）</option>
            <option value="whitelist">仅接管选中应用（白名单）</option>
          </select>
        </div>

        <div class="flex flex-wrap gap-2">
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3.5 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
            onclick={addClassicCNApps}
          >
            <Zap class="h-4 w-4" />
            <span>一键添加经典中国软件</span>
          </button>
          <button
            class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700"
            onclick={autoDetectCNApps}
          >
            <Sparkles class="h-4 w-4" />
            <span>自动识别中国软件</span>
          </button>
        </div>

        {#if detectNotice}
          <p class="text-xs text-emerald-600 dark:text-emerald-400">
            {detectNotice}
          </p>
        {/if}

        {#if excludedPackages.length > 0}
          <div class="flex flex-wrap gap-1.5">
            {#each excludedPackages as pkg}
              <span
                class="inline-flex items-center gap-1 rounded-lg bg-slate-100 px-2.5 py-1 text-xs text-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                <span>{pkg}</span>
                <button
                  class="text-slate-400 hover:text-rose-500"
                  title="移除"
                  onclick={() => removeExcludedPackage(pkg)}
                >
                  <Trash2 class="h-3 w-3" />
                </button>
              </span>
            {/each}
          </div>
        {/if}

        {#if detectedPreview.length > 0}
          <div class="space-y-1 text-xs text-slate-500 dark:text-slate-400">
            {#each detectedPreview as item}
              <div>{item.label} ({item.package_name}) · {item.reason}</div>
            {/each}
          </div>
        {/if}
      </section>
    {/if}
  </main>
</div>
