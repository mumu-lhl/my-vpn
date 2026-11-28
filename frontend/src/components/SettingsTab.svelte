<script lang="ts">
  import {
    AppWindow,
    ArrowLeft,
    ChevronRight,
    ExternalLink,
    Globe,
    Laptop,
    Moon,
    Route,
    Sun,
  } from "@lucide/svelte";
  import type { AppFormState } from "../lib/config_state";
  import type { ExternalNode } from "../lib/mygo";
  import type { PlatformUIProfile } from "../lib/platform";
  import type { TestURLPreset } from "../lib/probe";
  import type { ThemeMode } from "../lib/theme";
  import PerAppSelector from "./PerAppSelector.svelte";
  import ProtocolsTab from "./ProtocolsTab.svelte";
  import RoutingTab from "./RoutingTab.svelte";
  import Toggle from "./Toggle.svelte";

  export type SettingsSubpage = null | "protocols" | "routing" | "per_app";

  interface Props {
    form: AppFormState;
    platformUI: PlatformUIProfile;
    subpage: SettingsSubpage;
    bridgeReady: boolean;
    builtinProbePresets: TestURLPreset[];
    customProbeValid: boolean;
    themeMode: ThemeMode;
    subName: string;
    subURL: string;
    sortedExternalNodes: ExternalNode[];
    testingNodes: boolean;
    onSelectSubpage: (sub: SettingsSubpage) => void;
    onAutoSave: () => void;
    onDebouncedAutoSave: () => void;
    onSave: () => void;
    onSetTheme: (mode: ThemeMode) => void;
    onOpenGephExits: () => void;
    onImportSub: () => void;
    onTestAllNodes: () => void;
    onOpenAndroidVpnSettings: () => void;
  }

  let {
    form,
    platformUI,
    subpage,
    bridgeReady,
    builtinProbePresets,
    customProbeValid,
    themeMode,
    subName = $bindable(""),
    subURL = $bindable(""),
    sortedExternalNodes,
    testingNodes,
    onSelectSubpage,
    onAutoSave,
    onDebouncedAutoSave,
    onSave,
    onSetTheme,
    onOpenGephExits,
    onImportSub,
    onTestAllNodes,
    onOpenAndroidVpnSettings,
  }: Props = $props();

  const SUBPAGE_TITLES: Record<Exclude<SettingsSubpage, null>, string> = {
    protocols: "节点与协议",
    routing: "路由与 DNS",
    per_app: "分应用代理",
  };

  let perAppSummary = $derived(
    form.perAppMode === "all"
      ? "全部应用走代理"
      : form.perAppMode === "blacklist"
        ? `绕过已选 ${form.excludedPackages.length} 个应用`
        : `仅代理已选 ${form.excludedPackages.length} 个应用`,
  );
</script>

{#if subpage !== null}
  <div class="space-y-4">
    <div
      class="flex items-center gap-2 rounded-xl border border-zinc-200 bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900"
    >
      <button
        type="button"
        onclick={() => onSelectSubpage(null)}
        class="inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs font-medium text-zinc-600 transition hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
      >
        <ArrowLeft class="h-4 w-4" />
        <span>返回设置</span>
      </button>
      <span class="text-zinc-300 dark:text-zinc-700">/</span>
      <h1 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
        {SUBPAGE_TITLES[subpage]}
      </h1>
    </div>

    {#if subpage === "protocols"}
      <ProtocolsTab
        {form}
        bind:subName
        bind:subURL
        {sortedExternalNodes}
        {testingNodes}
        {onOpenGephExits}
        {onAutoSave}
        {onSave}
        {onImportSub}
        {onTestAllNodes}
      />
    {:else if subpage === "routing"}
      <RoutingTab {form} {platformUI} {onAutoSave} {onSave} />
    {:else if subpage === "per_app" && platformUI.showPerAppProxy}
      <PerAppSelector
        bind:mode={form.perAppMode}
        bind:selectedPackages={form.excludedPackages}
        {bridgeReady}
        isAndroid={platformUI.isAndroid}
        onChange={onAutoSave}
      />
    {/if}
  </div>
{:else}
  <!-- Subpage Hub Navigation -->
  <section
    class="divide-y divide-zinc-100 overflow-hidden rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <button
      type="button"
      onclick={() => onSelectSubpage("protocols")}
      class="flex w-full items-center justify-between gap-3 px-5 py-4 text-left transition hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
    >
      <div class="flex min-w-0 items-center gap-3">
        <span
          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-950/70 dark:text-blue-400"
        >
          <Globe class="h-4 w-4" />
        </span>
        <div class="min-w-0">
          <p class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
            节点与协议
          </p>
          <p class="truncate text-xs text-zinc-500 dark:text-zinc-400">
            迷雾通、赛风、WARP 与外部订阅（{sortedExternalNodes.length} 个节点）
          </p>
        </div>
      </div>
      <ChevronRight class="h-4 w-4 shrink-0 text-zinc-400" />
    </button>

    <button
      type="button"
      onclick={() => onSelectSubpage("routing")}
      class="flex w-full items-center justify-between gap-3 px-5 py-4 text-left transition hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
    >
      <div class="flex min-w-0 items-center gap-3">
        <span
          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-950/70 dark:text-emerald-400"
        >
          <Route class="h-4 w-4" />
        </span>
        <div class="min-w-0">
          <p class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
            路由与 DNS
          </p>
          <p class="truncate text-xs text-zinc-500 dark:text-zinc-400">
            分流规则、DNS 与强力反 DPI 自动探测策略
          </p>
        </div>
      </div>
      <ChevronRight class="h-4 w-4 shrink-0 text-zinc-400" />
    </button>

    {#if platformUI.showPerAppProxy}
      <button
        type="button"
        onclick={() => onSelectSubpage("per_app")}
        class="flex w-full items-center justify-between gap-3 px-5 py-4 text-left transition hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
      >
        <div class="flex min-w-0 items-center gap-3">
          <span
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600 dark:bg-indigo-950/70 dark:text-indigo-400"
          >
            <AppWindow class="h-4 w-4" />
          </span>
          <div class="min-w-0">
            <p class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
              分应用代理
            </p>
            <p class="truncate text-xs text-zinc-500 dark:text-zinc-400">
              {perAppSummary}
            </p>
          </div>
        </div>
        <ChevronRight class="h-4 w-4 shrink-0 text-zinc-400" />
      </button>
    {/if}
  </section>

  <!-- Connectivity Probe URL (instant auto-save, no Save button) -->
  <section
    class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div>
      <h2 class="text-base font-semibold">连通性检测地址</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        用于检测各跳延迟与可用性，修改后自动保存
      </p>
    </div>

    <div
      role="tablist"
      aria-label="检测地址来源"
      class="inline-grid grid-cols-2 gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800"
    >
      <button
        type="button"
        role="tab"
        aria-selected={form.probeMode === "builtin"}
        onclick={() => {
          form.probeMode = "builtin";
          onAutoSave();
        }}
        class="rounded-md px-3 py-1.5 text-xs font-medium transition {form.probeMode ===
        'builtin'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
      >
        内置地址
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={form.probeMode === "custom"}
        onclick={() => {
          form.probeMode = "custom";
          if (customProbeValid) onAutoSave();
        }}
        class="rounded-md px-3 py-1.5 text-xs font-medium transition {form.probeMode ===
        'custom'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
      >
        自定义地址
      </button>
    </div>

    {#if form.probeMode === "builtin"}
      <select
        class="w-full rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={form.probeBuiltinURL}
        onchange={onAutoSave}
      >
        {#each builtinProbePresets as preset}
          <option value={preset.url}>
            {preset.label} — {preset.url}
          </option>
        {/each}
      </select>
    {:else}
      <input
        class="w-full rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="url"
        bind:value={form.probeCustomURL}
        oninput={onDebouncedAutoSave}
        onblur={onAutoSave}
        placeholder="https://cp.cloudflare.com/generate_204"
      />
      {#if !customProbeValid}
        <p class="text-xs text-red-600 dark:text-red-400">
          请输入有效的 http:// 或 https:// 地址（输入合法后自动保存）
        </p>
      {/if}
    {/if}
  </section>

  <!-- Appearance & Platform-Specific System Settings -->
  <section
    class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <h2 class="text-base font-semibold">外观与系统</h2>

    <div
      role="tablist"
      aria-label="外观主题"
      class="grid grid-cols-3 gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800"
    >
      <button
        type="button"
        role="tab"
        aria-selected={themeMode === "light"}
        class="flex items-center justify-center gap-1.5 rounded-md py-1.5 text-xs font-medium transition {themeMode ===
        'light'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
        onclick={() => onSetTheme("light")}
      >
        <Sun class="h-3.5 w-3.5" />
        <span>浅色</span>
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={themeMode === "dark"}
        class="flex items-center justify-center gap-1.5 rounded-md py-1.5 text-xs font-medium transition {themeMode ===
        'dark'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
        onclick={() => onSetTheme("dark")}
      >
        <Moon class="h-3.5 w-3.5" />
        <span>深色</span>
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={themeMode === "system"}
        class="flex items-center justify-center gap-1.5 rounded-md py-1.5 text-xs font-medium transition {themeMode ===
        'system'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
        onclick={() => onSetTheme("system")}
      >
        <Laptop class="h-3.5 w-3.5" />
        <span>跟随系统</span>
      </button>
    </div>

    {#if platformUI.showTrayOption || platformUI.showSystemProxyOption}
      <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
        {#if platformUI.showTrayOption}
          <Toggle
            bind:checked={form.minimizeToTray}
            label="关闭窗口时最小化到系统托盘"
            description="关闭主窗口后在后台保持代理运行"
            onchange={onAutoSave}
          />
        {/if}
        {#if platformUI.showSystemProxyOption}
          <Toggle
            bind:checked={form.setSystemProxy}
            label="自动配置桌面系统代理"
            description="使用本地代理模式时自动更新 GNOME / GTK 系统代理设置"
            onchange={onAutoSave}
          />
        {/if}
      </div>
    {/if}

    {#if platformUI.showAndroidVpnShortcut}
      <div class="pt-1">
        <button
          type="button"
          class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-2 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
          onclick={onOpenAndroidVpnSettings}
        >
          <ExternalLink class="h-3.5 w-3.5" />
          <span>打开系统「始终开启 VPN / 阻断非 VPN 连接」设置</span>
        </button>
      </div>
    {/if}
  </section>
{/if}
