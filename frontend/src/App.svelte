<script lang="ts">
  import { onMount } from "svelte";
  import { analyzeChain } from "./lib/chain";
  import { applyTheme, type ThemeMode } from "./lib/theme";
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
  import ChainTab from "./components/ChainTab.svelte";
  import ProtocolsTab from "./components/ProtocolsTab.svelte";
  import RoutingTab from "./components/RoutingTab.svelte";
  import SettingsTab from "./components/SettingsTab.svelte";

  let activeTab = $state<AppTab>("dashboard");
  let themeMode = $state<ThemeMode>("system");
  let bridgeReady = $state(false);
  let rawConfig = $state<Record<string, any>>({});
  let form = $state(createDefaultFormState());

  let busyConnect = $state(false);
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

  async function persistConfig(showNotice = false): Promise<boolean> {
    if (!customProbeValid) {
      statusError = "自定义检测地址需以 http:// 或 https:// 开头";
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

  async function syncRuntimeState(includeConfig = false) {
    if (!bridgeReady) return;
    try {
      const stats = await AppService.GetStats();
      connected = stats.running;
      upBytes = stats.up_bytes;
      downBytes = stats.down_bytes;
      if (stats.mode === "tun" || stats.mode === "proxy") {
        form.inboundMode = stats.mode;
      }
      if (includeConfig) {
        const cfg = await AppService.GetConfig();
        rawConfig = cfg;
        form = hydrateFormState(cfg, form);
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

  async function setInboundMode(mode: "proxy" | "tun") {
    if (form.inboundMode === mode) return;
    form.inboundMode = mode;
    await persistConfig(false);
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
      statusNotice = "连接迷雾通后即可拉取实时出口列表";
      return;
    }
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
        AppService.GetConfig(),
        AppService.GetStats(),
        AppService.GetLANInfo(),
        AppService.GetBuiltinProbeURLs(),
      ])
        .then(([cfg, stats, lan, presets]) => {
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

    const onWindowFocus = () => void syncRuntimeState(true);
    const onVisibility = () => {
      if (!document.hidden) void syncRuntimeState(true);
    };
    window.addEventListener("focus", onWindowFocus);
    document.addEventListener("visibilitychange", onVisibility);

    const timer = window.setInterval(() => void syncRuntimeState(false), 1500);
    return () => {
      mq.removeEventListener("change", onThemeChange);
      window.removeEventListener("focus", onWindowFocus);
      document.removeEventListener("visibilitychange", onVisibility);
      window.clearInterval(timer);
    };
  });
</script>

<div
  class="flex min-h-screen flex-col-reverse bg-zinc-50 text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100 md:flex-row"
>
  <NavSidebar
    {activeTab}
    {themeMode}
    onSelectTab={(tab) => (activeTab = tab)}
    onSelectTheme={(mode) => (themeMode = mode)}
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
        {connected}
        {busyConnect}
        {lanIP}
        {upBytes}
        {downBytes}
        {chainInfo}
        {backendWarnings}
        {activeProbeURL}
        {probing}
        {probeResult}
        onToggleConnect={toggleConnect}
        onSetInboundMode={setInboundMode}
        onAllowLANChange={() => void persistConfig(false)}
        onKillSwitchChange={() => void onToggleKillSwitch()}
        onRunProbe={runProbe}
      />
    {:else if activeTab === "chain"}
      <ChainTab
        bind:chain={form.chain}
        {chainInfo}
        {sortedExternalNodes}
        onSave={() => void persistConfig(true)}
        onGoProtocols={() => (activeTab = "protocols")}
      />
    {:else if activeTab === "protocols"}
      <ProtocolsTab
        {form}
        {gephExits}
        bind:subName
        bind:subURL
        {sortedExternalNodes}
        {testingNodes}
        onLoadGephExits={loadGephExits}
        onSave={() => void persistConfig(true)}
        onImportSub={importSub}
        onTestAllNodes={testAllNodes}
      />
    {:else if activeTab === "routing"}
      <RoutingTab {form} onSave={() => void persistConfig(true)} />
    {:else}
      <SettingsTab
        {form}
        {bridgeReady}
        {builtinProbePresets}
        {customProbeValid}
        {themeMode}
        onAutoSave={() => void persistConfig(false)}
        onSave={() => void persistConfig(true)}
        onSetTheme={(mode) => (themeMode = mode)}
        onOpenAndroidVpnSettings={openAndroidVpnSettings}
      />
    {/if}
  </main>
</div>
