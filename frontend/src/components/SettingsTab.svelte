<script lang="ts">
  import { ExternalLink, Laptop, Moon, Save, Sun } from "@lucide/svelte";
  import type { AppFormState } from "../lib/config_state";
  import type { TestURLPreset } from "../lib/probe";
  import type { ThemeMode } from "../lib/theme";
  import PerAppSelector from "./PerAppSelector.svelte";
  import Toggle from "./Toggle.svelte";

  interface Props {
    form: AppFormState;
    bridgeReady: boolean;
    builtinProbePresets: TestURLPreset[];
    customProbeValid: boolean;
    themeMode: ThemeMode;
    onAutoSave: () => void;
    onSave: () => void;
    onSetTheme: (mode: ThemeMode) => void;
    onOpenAndroidVpnSettings: () => void;
  }

  let {
    form,
    bridgeReady,
    builtinProbePresets,
    customProbeValid,
    themeMode,
    onAutoSave,
    onSave,
    onSetTheme,
    onOpenAndroidVpnSettings,
  }: Props = $props();
</script>

<PerAppSelector
  bind:mode={form.perAppMode}
  bind:selectedPackages={form.excludedPackages}
  {bridgeReady}
  onChange={onAutoSave}
/>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-base font-semibold">连通性检测地址</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        用于检测各跳延迟与可用性，支持内置地址或自定义 URL
      </p>
    </div>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
      onclick={onSave}
    >
      <Save class="h-3.5 w-3.5" />
      <span>保存</span>
    </button>
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
      onclick={() => (form.probeMode = "builtin")}
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
      onclick={() => (form.probeMode = "custom")}
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
      placeholder="https://cp.cloudflare.com/generate_204"
    />
    {#if !customProbeValid}
      <p class="text-xs text-red-600 dark:text-red-400">
        请输入有效的 http:// 或 https:// 地址
      </p>
    {/if}
  {/if}
</section>

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

  <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
    <Toggle
      bind:checked={form.minimizeToTray}
      label="关闭窗口时最小化到系统托盘"
      description="Linux 桌面下关闭主窗口后在后台保持代理运行"
      onchange={onAutoSave}
    />
    <Toggle
      bind:checked={form.setSystemProxy}
      label="自动配置桌面系统代理"
      description="使用本地代理模式时自动更新 GNOME / GTK 系统代理设置"
      onchange={onAutoSave}
    />
  </div>

  <div class="pt-1">
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-2 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
      onclick={onOpenAndroidVpnSettings}
    >
      <ExternalLink class="h-3.5 w-3.5" />
      <span>打开 Android 系统「始终开启 / 阻断非代理连接」设置</span>
    </button>
  </div>
</section>
