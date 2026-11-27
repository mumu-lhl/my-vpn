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
    AndroidNetPlugin,
    AppService,
    hasMyGoBridge,
    type GephExitLocation,
  } from "./lib/mygo";
  import {
    buildConfigPayload,
    createDefaultFormState,
    hydrateFormState,
  } from "./lib/config_state";
  import NavSidebar, { type AppTab } from "./components/NavSidebar.svelte";
  import StatusBanners from "./components/StatusBanners.svelte";
  import DashboardTab from "./components/DashboardTab.svelte";
  import SettingsTab, {
    type SettingsSubpage,
  } from "./components/SettingsTab.svelte";

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
  let subName = $state("");
  let subURL = $state("");
  let testingNodes = $state(false);

  let builtinProbePresets = $state<TestURLPreset[]>(BUILTIN_PROBE_URLS);
  let probing = $state(false);
  let probeResult = $state<ChainProbeResult | null>(null);

  let debouncedSaveTimer: number | undefined;

  let platformUI = $derived(getPlatformUIProfile(runtimePlatform));
  let chainInfo = $derived(analyzeChain(form.chain));
  let activeProbeURL = $derived(
    form.probeMode === "custom" && form.probeCustomURL.trim()
      ? form.probeCustomURL.trim()
      : form.probeBuiltinURL,
  );
  let customProbeValid = $derived(
    form.probeMode !== "custom" || validateProbeUrl(form.probeCustomURL),
  );
  let sortedExternalNodes = $derived(
    [...form.externalNodes].sort(
      (a, b) => (a.latency_ms || 999999) - (b.latency_ms || 999999),
    ),
  );

  const errMsg = (err: unknown) =>
    err instanceof Error ? err.message : String(err);

  function selectTheme(mode: ThemeMode) {
    themeMode = mode;
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(THEME_STORAGE_KEY, mode);
    }
  }

  function openSettingsSubpage(sub: SettingsSubpage) {
    activeTab = "settings";
    settingsSubpage = sub;
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

  // Only sync live connection status & traffic counters; NEVER overwrite form.inboundMode
  // or user-edited fields from background polling or app resume.
  async function syncRuntimeStats() {
    if (!bridgeReady || document.hidden) return;
    try {
      const stats = await AppService.GetStats();
      connected = stats.running;
      upBytes = stats.up_bytes;
      downBytes = stats.down_bytes;
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
        backendWarnings = [];
      } else if (await persistConfig(false)) {
        const res = await AppService.Connect();
        backendWarnings = res?.warnings ?? [];
        connected = true;
        lanIP = (await AppService.GetLANInfo()).ip;
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
    if (!bridgeReady) {
      statusNotice = "请先在客户端连接迷雾通后再刷新实时出口列表";
      return;
    }
    statusError = "";
    try {
      gephExits = await AppService.FetchGephExits();
      statusNotice = `已刷新 ${gephExits.length} 个可用出口`;
    } catch (err) {
      statusError = `获取出口列表失败：${errMsg(err)}`;
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
    try {
      const nodes = await AppService.ImportSubscription(
        subName.trim() || "订阅",
        subURL.trim(),
        24,
      );
      form.externalNodes = [...form.externalNodes, ...nodes];
      subName = "";
      subURL = "";
      statusNotice = `已导入 ${nodes.length} 个节点`;
    } catch (err) {
      statusError = `导入订阅失败：${errMsg(err)}`;
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
      await AndroidNetPlugin.openVpnSettings();
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
    };
  });
</script>

<div
  class="flex min-h-screen flex-col-reverse bg-zinc-50 text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100 md:flex-row"
>
  <NavSidebar
    {activeTab}
    {themeMode}
    onSelectTab={(tab) => {
      activeTab = tab;
      if (tab === "settings") {
        settingsSubpage = null;
      }
    }}
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
        {sortedExternalNodes}
        {backendWarnings}
        {activeProbeURL}
        {probing}
        {probeResult}
        onToggleConnect={toggleConnect}
        onSetInboundMode={setInboundMode}
        onChainChange={() => void persistConfig(false)}
        onAllowLANChange={() => void persistConfig(false)}
        onKillSwitchChange={() => void onToggleKillSwitch()}
        onOpenAndroidVpnSettings={openAndroidVpnSettings}
        onOpenSettingsSubpage={(sub) => openSettingsSubpage(sub)}
        onRunProbe={runProbe}
      />
    {:else}
      <SettingsTab
        {form}
        {platformUI}
        subpage={settingsSubpage}
        {bridgeReady}
        {builtinProbePresets}
        {customProbeValid}
        {themeMode}
        {gephExits}
        bind:subName
        bind:subURL
        {sortedExternalNodes}
        {testingNodes}
        onSelectSubpage={(sub) => (settingsSubpage = sub)}
        onAutoSave={() => void persistConfig(false)}
        onDebouncedAutoSave={scheduleDebouncedSave}
        onSave={() => void persistConfig(true)}
        onSetTheme={selectTheme}
        onLoadGephExits={loadGephExits}
        onImportSub={importSub}
        onTestAllNodes={testAllNodes}
        onOpenAndroidVpnSettings={openAndroidVpnSettings}
      />
    {/if}
  </main>
</div>
