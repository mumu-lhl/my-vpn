<script lang="ts">
  import { onMount } from "svelte";
  import { analyzeChain } from "./lib/chain";
  import { applyTheme, type ThemeMode } from "./lib/theme";
  import {
    detectFallbackPlatform,
    getPlatformUIProfile,
    normalizeRuntimePlatform,
    type RuntimePlatform,
  } from "./lib/platform";
  import {
    BUILTIN_PROBE_URLS,
    buildPreviewProbeResult,
    validateProbeUrl,
    type ChainProbeResult,
    type TestURLPreset,
  } from "./lib/probe";
  import {
    AppService,
    hasMyGoBridge,
    type GephBridgeStatus,
    type GephExitLocation,
    type VPNGateNode,
  } from "./lib/mygo";
  import {
    buildConfigPayload,
    createDefaultFormState,
    hydrateFormState,
  } from "./lib/config_state";
  import NavSidebar, { type AppTab } from "./components/NavSidebar.svelte";
  import StatusBanners from "./components/StatusBanners.svelte";
  import DashboardTab from "./components/DashboardTab.svelte";
  import GephExitsSubpage from "./components/GephExitsSubpage.svelte";
  import VPNGateSubpage from "./components/VPNGateSubpage.svelte";
  import SettingsTab, {
    type SettingsSubpage,
  } from "./components/SettingsTab.svelte";

  type DashboardSubpage = null | "geph-exits" | "vpngate";

  const THEME_STORAGE_KEY = "looma_theme_mode";

  function loadInitialTheme(): ThemeMode {
    if (typeof localStorage === "undefined") return "system";
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (saved === "light" || saved === "dark" || saved === "system") {
      return saved;
    }
    return "system";
  }

  let activeTab = $state<AppTab>("dashboard");
  let dashboardSubpage = $state<DashboardSubpage>(null);
  let settingsSubpage = $state<SettingsSubpage>(null);
  let themeMode = $state<ThemeMode>(loadInitialTheme());
  let runtimePlatform = $state<RuntimePlatform>(detectFallbackPlatform());
  let bridgeReady = $state(false);
  let rawConfig = $state<Record<string, any>>({});
  let form = $state(createDefaultFormState());

  let busyConnect = $state(false);
  let switchingMode = $state(false);
  let statusError = $state("");
  let statusNotice = $state("");
  let backendWarnings = $state<Array<{ code: string; message: string }>>([]);

  let connected = $state(false);
  let lanIP = $state("127.0.0.1");
  let upBytes = $state(0);
  let downBytes = $state(0);

  let gephExits = $state<GephExitLocation[]>([]);
  let gephBridgeStatus = $state<GephBridgeStatus | null>(null);
  let loadingGephExits = $state(false);
  let switchingGephExit = $state(false);
  let gephExitsError = $state("");
  let gephExitsUpdatedAt = $state(0);

  let vpngateNodes = $state<VPNGateNode[]>([]);
  let loadingVPNGate = $state(false);
  let switchingVPNGate = $state(false);
  let vpngateError = $state("");
  let subName = $state("");
  let subURL = $state("");
  let subIntervalHours = $state(24);
  let subProxyMode = $state<"auto" | "proxy" | "direct">("auto");
  let refreshingSubId = $state("");
  let testingNodes = $state(false);

  let builtinProbePresets = $state<TestURLPreset[]>(BUILTIN_PROBE_URLS);
  let probing = $state(false);
  let probeResult = $state<ChainProbeResult | null>(null);

  let debouncedSaveTimer: number | undefined;

  let platformUI = $derived(getPlatformUIProfile(runtimePlatform));
  let chainInfo = $derived(
    analyzeChain(form.chain, true, form.vpngateNode?.supports_udp ?? true),
  );
  let activeProbeURL = $derived(
    form.probeMode === "custom" && form.probeCustomURL.trim()
      ? form.probeCustomURL.trim()
      : form.probeBuiltinURL,
  );
  let customProbeValid = $derived(
    form.probeMode !== "custom" || validateProbeUrl(form.probeCustomURL),
  );
  let sortedExternalNodes = $derived(
    [...form.externalNodes].sort((a, b) => {
      const la = a.latency_ms && a.latency_ms > 0 ? a.latency_ms : 999999;
      const lb = b.latency_ms && b.latency_ms > 0 ? b.latency_ms : 999999;
      return la - lb;
    }),
  );

  const errMsg = (err: unknown) =>
    err instanceof Error ? err.message : String(err);

  function selectTheme(mode: ThemeMode) {
    themeMode = mode;
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(THEME_STORAGE_KEY, mode);
    }
  }

  function buildRouteHash(
    tab: AppTab,
    dashSub: DashboardSubpage,
    settSub: SettingsSubpage,
  ): string {
    if (tab === "settings") {
      return settSub ? `#settings/${settSub}` : "#settings";
    }
    return dashSub ? `#dashboard/${dashSub}` : "#dashboard";
  }

  function navigateTo(
    tab: AppTab,
    options?: {
      dashboardSubpage?: DashboardSubpage;
      settingsSubpage?: SettingsSubpage;
    },
  ) {
    const nextDashSub: DashboardSubpage =
      tab === "dashboard" ? (options?.dashboardSubpage ?? null) : null;
    const nextSettSub: SettingsSubpage =
      tab === "settings" ? (options?.settingsSubpage ?? null) : null;
    if (
      activeTab === tab &&
      dashboardSubpage === nextDashSub &&
      settingsSubpage === nextSettSub
    ) {
      return;
    }
    activeTab = tab;
    dashboardSubpage = nextDashSub;
    settingsSubpage = nextSettSub;
    if (
      typeof window !== "undefined" &&
      typeof window.history?.pushState === "function"
    ) {
      window.history.pushState(
        {
          tab,
          dashboardSubpage: nextDashSub,
          settingsSubpage: nextSettSub,
        },
        "",
        buildRouteHash(tab, nextDashSub, nextSettSub),
      );
    }
  }

  function openSettingsSubpage(sub: SettingsSubpage) {
    navigateTo("settings", { settingsSubpage: sub });
  }

  function openDashboardGephExits() {
    navigateTo("dashboard", { dashboardSubpage: "geph-exits" });
  }

  function openDashboardVPNGate() {
    navigateTo("dashboard", { dashboardSubpage: "vpngate" });
  }

  async function persistConfig(showNotice = false): Promise<boolean> {
    if (!customProbeValid) {
      if (showNotice) {
        statusError = "自定义检测地址需以 http:// 或 https:// 开头";
      }
      return false;
    }
    statusError = "";
    const payload = buildConfigPayload(rawConfig, form);
    rawConfig = payload;
    if (!bridgeReady) {
      if (showNotice) statusNotice = "已更新本地配置";
      return true;
    }
    try {
      await AppService.SaveConfig(payload);
      if (showNotice) statusNotice = "设置已保存";
      return true;
    } catch (err) {
      statusError = `保存失败：${errMsg(err)}`;
      return false;
    }
  }

  function scheduleDebouncedSave() {
    if (debouncedSaveTimer) window.clearTimeout(debouncedSaveTimer);
    debouncedSaveTimer = window.setTimeout(() => {
      debouncedSaveTimer = undefined;
      void persistConfig(false);
    }, 400);
  }

  function flushPendingSave() {
    if (debouncedSaveTimer) {
      window.clearTimeout(debouncedSaveTimer);
      debouncedSaveTimer = undefined;
      void persistConfig(false);
    }
  }

  async function refreshGephBridgeStatus() {
    const usesGeph =
      form.chain.hop1.kind === "geph" ||
      (form.chain.two_hop && form.chain.hop2.kind === "geph");
    if (!bridgeReady || !connected || !usesGeph) {
      gephBridgeStatus = null;
      return;
    }
    try {
      gephBridgeStatus = await AppService.GetGephBridgeStatus();
    } catch {
      // Ignore transient RPC errors while geph5-client initializes.
    }
  }

  // Only sync live connection status & traffic counters; NEVER overwrite form.inboundMode
  // or user-edited fields from background polling or app resume.
  async function syncRuntimeStats() {
    if (!bridgeReady || document.hidden) return;
    try {
      const stats = await AppService.GetStats();
      connected = stats.running;
      upBytes = stats.up_bytes;
      downBytes = stats.down_bytes;
      if (stats.running) {
        void refreshGephBridgeStatus();
      } else {
        gephBridgeStatus = null;
      }
    } catch {
      // Ignore transient poll errors while backend is restarting.
    }
  }

  async function runProbe() {
    if (!(await persistConfig(false))) return;
    probing = true;
    statusError = "";
    try {
      probeResult = bridgeReady
        ? await AppService.ProbeChain()
        : buildPreviewProbeResult(activeProbeURL, form.chain);
    } catch (err) {
      statusError = `连通性检测失败：${errMsg(err)}`;
    } finally {
      probing = false;
    }
  }

  async function toggleConnect() {
    if (busyConnect || switchingMode) return;
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
        gephBridgeStatus = null;
        backendWarnings = [];
      } else if (await persistConfig(false)) {
        const res = await AppService.Connect();
        backendWarnings = res?.warnings ?? [];
        connected = true;
        lanIP = (await AppService.GetLANInfo()).ip;
        void refreshGephBridgeStatus();
        void runProbe();
      }
    } catch (err) {
      statusError = `连接操作失败：${errMsg(err)}`;
    } finally {
      busyConnect = false;
    }
  }

  // Persists inboundMode immediately; if currently connected, disconnects and reconnects
  // cleanly in the new mode without losing user selection.
  async function setInboundMode(mode: "proxy" | "tun") {
    if (form.inboundMode === mode || switchingMode) return;
    const prevMode = form.inboundMode;
    form.inboundMode = mode;
    statusError = "";
    statusNotice = "";

    if (!bridgeReady) {
      return;
    }

    if (connected) {
      switchingMode = true;
      try {
        await AppService.SwitchInboundMode(mode);
        const cfg = await AppService.GetConfig();
        rawConfig = cfg;
        await syncRuntimeStats();
        statusNotice = `已切换至${mode === "tun" ? platformUI.tunBadgeLabel : "本地代理"}并重连`;
      } catch (err) {
        form.inboundMode = prevMode;
        statusError = `切换模式失败：${errMsg(err)}`;
      } finally {
        switchingMode = false;
      }
      return;
    }

    const ok = await persistConfig(false);
    if (!ok) {
      form.inboundMode = prevMode;
    }
  }

  async function onToggleKillSwitch() {
    await persistConfig(false);
    if (!bridgeReady) return;
    try {
      await AppService.SetKillSwitch(form.killSwitch);
    } catch (err) {
      statusError = `断网保护切换失败：${errMsg(err)}`;
    }
  }

  async function loadGephExits() {
    if (loadingGephExits) return;
    if (!bridgeReady) {
      gephExitsError = "浏览器预览模式下无法连接迷雾通节点接口";
      return;
    }
    if (!form.gephSecret.trim()) {
      gephExitsError = "";
      return;
    }
    loadingGephExits = true;
    gephExitsError = "";
    try {
      gephExits = await AppService.FetchGephExits();
      gephExitsUpdatedAt = Date.now();
    } catch (err) {
      gephExitsError = errMsg(err);
    } finally {
      loadingGephExits = false;
    }
  }

  async function selectGephExit(country: string, city: string) {
    if (switchingGephExit) return;
    const prevCountry = form.gephCountry;
    const prevCity = form.gephCity;
    const nextCountry =
      !country || country.toLowerCase() === "auto"
        ? "auto"
        : country.toUpperCase();
    const nextCity = nextCountry === "auto" ? "" : city;

    form.gephCountry = nextCountry;
    form.gephCity = nextCity;
    statusError = "";
    statusNotice = "";

    if (!bridgeReady) {
      navigateTo("dashboard", { dashboardSubpage: null });
      return;
    }

    switchingGephExit = true;
    try {
      await AppService.SelectGephExit(nextCountry, nextCity);
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      await syncRuntimeStats();
      statusNotice =
        nextCountry === "auto"
          ? "已切换为迷雾通自动选择出口"
          : `已切换迷雾通出口：${nextCountry}${nextCity ? ` · ${nextCity}` : ""}`;
      navigateTo("dashboard", { dashboardSubpage: null });
    } catch (err) {
      form.gephCountry = prevCountry;
      form.gephCity = prevCity;
      gephExitsError = `切换出口失败：${errMsg(err)}`;
    } finally {
      switchingGephExit = false;
    }
  }

  async function loadVPNGateCache() {
    if (!bridgeReady) return;
    try {
      const cached = await AppService.GetVPNGateNodes();
      if (Array.isArray(cached)) {
        vpngateNodes = cached;
      }
    } catch {
      // Ignore missing cache on first launch.
    }
  }

  async function refreshVPNGateNodes() {
    if (loadingVPNGate) return;
    if (!bridgeReady) {
      vpngateError = "浏览器预览模式下无法拉取 VPNGate 节点";
      return;
    }
    if (!connected) {
      vpngateError = "请先连接任意可用代理或 VPN 后再拉取 VPNGate 节点列表";
      return;
    }
    loadingVPNGate = true;
    vpngateError = "";
    try {
      vpngateNodes = await AppService.FetchVPNGateNodes();
    } catch (err) {
      vpngateError = errMsg(err);
    } finally {
      loadingVPNGate = false;
    }
  }

  async function selectVPNGateNode(node: VPNGateNode) {
    if (switchingVPNGate) return;
    const prevNode = form.vpngateNode;
    form.vpngateNode = {
      id: node.id,
      host_name: node.host_name,
      ip: node.ip,
      country_long: node.country_long,
      country_short: node.country_short,
      network: node.network,
      supports_udp: node.supports_udp,
    };
    statusError = "";
    statusNotice = "";

    if (!bridgeReady) {
      navigateTo("dashboard", { dashboardSubpage: null });
      return;
    }

    switchingVPNGate = true;
    try {
      await AppService.SelectVPNGateNode(node);
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      form = hydrateFormState(cfg, form);
      await syncRuntimeStats();
      statusNotice = `已选择 VPNGate 节点：${node.country_short} · ${node.host_name || node.ip}`;
      navigateTo("dashboard", { dashboardSubpage: null });
    } catch (err) {
      form.vpngateNode = prevNode;
      vpngateError = `切换 VPNGate 节点失败：${errMsg(err)}`;
    } finally {
      switchingVPNGate = false;
    }
  }

  async function importSub() {
    if (!subURL.trim()) {
      statusError = "请输入订阅链接或分享 URI";
      return;
    }
    if (!bridgeReady) {
      statusNotice = "请先启动客户端后再导入订阅";
      return;
    }
    statusError = "";
    try {
      const nodes = await AppService.ImportSubscription(
        subName.trim() || "订阅",
        subURL.trim(),
        subIntervalHours,
        subProxyMode,
      );
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      form = hydrateFormState(cfg, form);
      subName = "";
      subURL = "";
      statusNotice = `已导入 ${nodes.length} 个节点`;
    } catch (err) {
      statusError = `导入订阅失败：${errMsg(err)}`;
    }
  }

  async function refreshSub(subId: string) {
    if (!bridgeReady || refreshingSubId) return;
    refreshingSubId = subId;
    statusError = "";
    try {
      const nodes = await AppService.RefreshSubscription(subId);
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      form = hydrateFormState(cfg, form);
      statusNotice = `已更新订阅，共 ${nodes.length} 个节点`;
    } catch (err) {
      const cfg = await AppService.GetConfig().catch(() => null);
      if (cfg) {
        rawConfig = cfg;
        form = hydrateFormState(cfg, form);
      }
      statusError = `更新订阅失败：${errMsg(err)}`;
    } finally {
      refreshingSubId = "";
    }
  }

  async function updateSubSettings(
    subId: string,
    name: string,
    intervalHours: number,
    proxyMode: "auto" | "proxy" | "direct",
  ) {
    if (!bridgeReady) return;
    try {
      await AppService.UpdateSubscriptionSettings(
        subId,
        name,
        intervalHours,
        proxyMode,
      );
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      form = hydrateFormState(cfg, form);
    } catch (err) {
      statusError = `更新订阅设置失败：${errMsg(err)}`;
    }
  }

  async function deleteSub(subId: string) {
    if (!bridgeReady) return;
    try {
      await AppService.DeleteSubscription(subId);
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      form = hydrateFormState(cfg, form);
      statusNotice = "已删除订阅及其节点";
    } catch (err) {
      statusError = `删除订阅失败：${errMsg(err)}`;
    }
  }

  async function deleteExternalNode(nodeId: string) {
    if (!bridgeReady) return;
    try {
      await AppService.DeleteExternalNode(nodeId);
      const cfg = await AppService.GetConfig();
      rawConfig = cfg;
      form = hydrateFormState(cfg, form);
    } catch (err) {
      statusError = `删除节点失败：${errMsg(err)}`;
    }
  }

  async function testAllNodes() {
    if (!bridgeReady) return;
    testingNodes = true;
    try {
      const results = await AppService.TestAllNodes();
      const map = new Map(results.map((r) => [r.node_id, r.latency_ms]));
      form.externalNodes = form.externalNodes.map((n) => ({
        ...n,
        latency_ms: map.get(n.id) ?? n.latency_ms,
      }));
    } catch (err) {
      statusError = `测速失败：${errMsg(err)}`;
    } finally {
      testingNodes = false;
    }
  }

  async function openAndroidVpnSettings() {
    if (!bridgeReady) {
      statusNotice = "此快捷入口仅在 Android 设备可用";
      return;
    }
    try {
      await AppService.OpenVpnSettings();
    } catch (err) {
      statusError = `无法打开系统设置：${errMsg(err)}`;
    }
  }

  $effect(() => {
    applyTheme(themeMode);
  });

  // Poll traffic stats ONLY while connected and visible in foreground.
  $effect(() => {
    if (!bridgeReady || !connected) return;
    const timer = window.setInterval(() => {
      if (!document.hidden) {
        void syncRuntimeStats();
      }
    }, 2000);
    return () => window.clearInterval(timer);
  });

  onMount(() => {
    applyTheme(themeMode);
    bridgeReady = hasMyGoBridge();
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onThemeChange = () => {
      if (themeMode === "system") applyTheme("system");
    };
    mq.addEventListener("change", onThemeChange);

    if (bridgeReady) {
      void Promise.all([
        AppService.GetRuntimePlatform().catch(() => ""),
        AppService.GetConfig(),
        AppService.GetStats(),
        AppService.GetLANInfo(),
        AppService.GetBuiltinProbeURLs(),
      ])
        .then(([plat, cfg, stats, lan, presets]) => {
          const normalized = normalizeRuntimePlatform(plat);
          if (normalized !== "unknown") {
            runtimePlatform = normalized;
          }
          rawConfig = cfg;
          form = hydrateFormState(cfg, form);
          connected = stats.running;
          upBytes = stats.up_bytes;
          downBytes = stats.down_bytes;
          lanIP = lan.ip;
          if (Array.isArray(presets) && presets.length > 0) {
            builtinProbePresets = presets;
          }
        })
        .catch((err) => {
          statusError = `初始化失败：${errMsg(err)}`;
        });
    }

    const onPopState = (event: PopStateEvent) => {
      const st = event.state as {
        tab?: AppTab;
        dashboardSubpage?: DashboardSubpage;
        settingsSubpage?: SettingsSubpage;
      } | null;
      if (st && (st.tab === "dashboard" || st.tab === "settings")) {
        activeTab = st.tab;
        dashboardSubpage =
          st.tab === "dashboard" ? (st.dashboardSubpage ?? null) : null;
        settingsSubpage =
          st.tab === "settings" ? (st.settingsSubpage ?? null) : null;
      } else {
        activeTab = "dashboard";
        dashboardSubpage = null;
        settingsSubpage = null;
      }
    };
    if (typeof window !== "undefined") {
      if (typeof window.history?.replaceState === "function") {
        window.history.replaceState(
          {
            tab: activeTab,
            dashboardSubpage,
            settingsSubpage,
          },
          "",
          buildRouteHash(activeTab, dashboardSubpage, settingsSubpage),
        );
      }
      window.addEventListener("popstate", onPopState);
    }

    const onVisibility = () => {
      if (document.hidden) {
        flushPendingSave();
      } else {
        void syncRuntimeStats();
      }
    };
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      flushPendingSave();
      mq.removeEventListener("change", onThemeChange);
      document.removeEventListener("visibilitychange", onVisibility);
      if (typeof window !== "undefined") {
        window.removeEventListener("popstate", onPopState);
      }
    };
  });
</script>

<div
  class="flex min-h-screen flex-col-reverse bg-zinc-50 text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100 md:flex-row"
>
  <NavSidebar
    {activeTab}
    {themeMode}
    onSelectTab={(tab) => navigateTo(tab)}
    onSelectTheme={selectTheme}
  />

  <main class="mx-auto w-full max-w-4xl flex-1 space-y-4 p-4 md:p-6">
    <StatusBanners
      error={statusError}
      notice={statusNotice}
      onClearError={() => (statusError = "")}
      onClearNotice={() => (statusNotice = "")}
    />

    {#if activeTab === "dashboard"}
      {#if dashboardSubpage === "geph-exits"}
        <GephExitsSubpage
          {form}
          {gephExits}
          loading={loadingGephExits}
          switching={switchingGephExit}
          errorMessage={gephExitsError}
          lastUpdatedAt={gephExitsUpdatedAt}
          onBack={() => navigateTo("dashboard", { dashboardSubpage: null })}
          onRefresh={loadGephExits}
          onSelectExit={selectGephExit}
          onOpenProtocolsSettings={() => openSettingsSubpage("protocols")}
        />
      {:else if dashboardSubpage === "vpngate"}
        <VPNGateSubpage
          {form}
          nodes={vpngateNodes}
          {connected}
          loading={loadingVPNGate}
          switching={switchingVPNGate}
          errorMessage={vpngateError}
          onBack={() => navigateTo("dashboard", { dashboardSubpage: null })}
          onLoadCache={loadVPNGateCache}
          onRefresh={refreshVPNGateNodes}
          onSelectNode={selectVPNGateNode}
        />
      {:else}
        <DashboardTab
          {form}
          {platformUI}
          {connected}
          {busyConnect}
          {switchingMode}
          {lanIP}
          {upBytes}
          {downBytes}
          {chainInfo}
          {gephBridgeStatus}
          {sortedExternalNodes}
          {backendWarnings}
          {probing}
          {probeResult}
          onToggleConnect={toggleConnect}
          onSetInboundMode={setInboundMode}
          onChainChange={() => void persistConfig(false)}
          onAllowLANChange={() => void persistConfig(false)}
          onKillSwitchChange={() => void onToggleKillSwitch()}
          onOpenAndroidVpnSettings={openAndroidVpnSettings}
          onOpenSettingsSubpage={(sub) => openSettingsSubpage(sub)}
          onOpenGephExits={openDashboardGephExits}
          onOpenVPNGate={openDashboardVPNGate}
          onRunProbe={runProbe}
        />
      {/if}
    {:else}
      <SettingsTab
        {form}
        {platformUI}
        subpage={settingsSubpage}
        {bridgeReady}
        {builtinProbePresets}
        {customProbeValid}
        {themeMode}
        bind:subName
        bind:subURL
        bind:subIntervalHours
        bind:subProxyMode
        {sortedExternalNodes}
        {testingNodes}
        {refreshingSubId}
        onSelectSubpage={(sub) =>
          navigateTo("settings", { settingsSubpage: sub })}
        onAutoSave={() => void persistConfig(false)}
        onDebouncedAutoSave={scheduleDebouncedSave}
        onSave={() => void persistConfig(true)}
        onSetTheme={selectTheme}
        onOpenGephExits={openDashboardGephExits}
        onImportSub={importSub}
        onRefreshSub={refreshSub}
        onUpdateSubSettings={updateSubSettings}
        onDeleteSub={deleteSub}
        onDeleteNode={deleteExternalNode}
        onTestAllNodes={testAllNodes}
        onOpenAndroidVpnSettings={openAndroidVpnSettings}
      />
    {/if}
  </main>
</div>
