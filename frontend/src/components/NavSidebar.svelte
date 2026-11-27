<script lang="ts">
  import {
    Activity,
    ArrowRightLeft,
    Globe,
    Laptop,
    Moon,
    Route,
    Settings,
    Shield,
    Sun,
  } from "@lucide/svelte";
  import type { ThemeMode } from "../lib/theme";

  export type AppTab =
    "dashboard" | "chain" | "protocols" | "routing" | "settings";

  interface Props {
    activeTab: AppTab;
    themeMode: ThemeMode;
    onSelectTab: (tab: AppTab) => void;
    onSelectTheme: (mode: ThemeMode) => void;
  }

  let { activeTab, themeMode, onSelectTab, onSelectTheme }: Props = $props();

  const NAV_ITEMS: Array<{ id: AppTab; label: string }> = [
    { id: "dashboard", label: "概览" },
    { id: "chain", label: "链路" },
    { id: "protocols", label: "节点与协议" },
    { id: "routing", label: "路由与 DNS" },
    { id: "settings", label: "设置" },
  ];
</script>

<aside
  class="safe-bottom sticky bottom-0 z-10 flex w-full shrink-0 flex-row justify-around border-t border-zinc-200 bg-white/95 px-2 pt-2 backdrop-blur dark:border-zinc-800 dark:bg-zinc-900/95 md:static md:w-52 md:flex-col md:justify-start md:gap-1 md:border-r md:border-t-0 md:p-3"
>
  <div
    class="mb-3 hidden items-center gap-2 px-2.5 py-1.5 text-base font-semibold tracking-tight md:flex"
  >
    <Shield class="h-4 w-4 text-blue-600 dark:text-blue-400" />
    <span>Looma</span>
  </div>

  {#each NAV_ITEMS as item}
    <button
      type="button"
      class="flex flex-col items-center gap-1 rounded-lg px-2.5 py-2 text-[11px] font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab ===
      item.id
        ? 'bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900'
        : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-100'}"
      onclick={() => onSelectTab(item.id)}
    >
      {#if item.id === "dashboard"}
        <Activity class="h-4 w-4" />
      {:else if item.id === "chain"}
        <ArrowRightLeft class="h-4 w-4" />
      {:else if item.id === "protocols"}
        <Globe class="h-4 w-4" />
      {:else if item.id === "routing"}
        <Route class="h-4 w-4" />
      {:else}
        <Settings class="h-4 w-4" />
      {/if}
      <span>{item.label}</span>
    </button>
  {/each}

  <div class="mt-auto hidden pt-4 md:block">
    <div
      class="grid grid-cols-3 gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800"
    >
      <button
        type="button"
        title="浅色"
        aria-label="浅色主题"
        class="flex items-center justify-center rounded-md py-1.5 transition {themeMode ===
        'light'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-500 hover:text-zinc-900 dark:text-zinc-400'}"
        onclick={() => onSelectTheme("light")}
      >
        <Sun class="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        title="深色"
        aria-label="深色主题"
        class="flex items-center justify-center rounded-md py-1.5 transition {themeMode ===
        'dark'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-500 hover:text-zinc-900 dark:text-zinc-400'}"
        onclick={() => onSelectTheme("dark")}
      >
        <Moon class="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        title="跟随系统"
        aria-label="跟随系统主题"
        class="flex items-center justify-center rounded-md py-1.5 transition {themeMode ===
        'system'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-500 hover:text-zinc-900 dark:text-zinc-400'}"
        onclick={() => onSelectTheme("system")}
      >
        <Laptop class="h-3.5 w-3.5" />
      </button>
    </div>
  </div>
</aside>
