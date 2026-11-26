<script lang="ts">
  import {
    Check,
    Plus,
    RefreshCw,
    ScanSearch,
    Search,
    X,
  } from "@lucide/svelte";
  import { AndroidNetPlugin, AppService } from "../lib/mygo";
  import {
    type DetectedCNApp,
    type InstalledApp,
    mergeUniquePackages,
  } from "../lib/cn_apps";
  import {
    buildPerAppCatalog,
    fallbackPreviewInstalledApps,
    filterAndSortPerApps,
    selectClassicCNPackages,
    selectDetectedCNPackages,
    togglePackageSelection,
  } from "../lib/per_app";

  interface Props {
    mode: "all" | "whitelist" | "blacklist";
    selectedPackages: string[];
    bridgeReady: boolean;
    onChange: () => void;
  }

  let {
    mode = $bindable<"all" | "whitelist" | "blacklist">("blacklist"),
    selectedPackages = $bindable<string[]>([]),
    bridgeReady,
    onChange,
  }: Props = $props();

  let installedApps = $state<InstalledApp[]>([]);
  let detectedOverride = $state<DetectedCNApp[] | undefined>(undefined);
  let loadedFromDevice = $state(false);
  let loadingApps = $state(false);
  let hasLoadedOnce = $state(false);

  let searchQuery = $state("");
  let showSystem = $state(false);
  let onlySelected = $state(false);
  let showManualInput = $state(false);
  let manualPkg = $state("");
  let actionNotice = $state("");

  let catalog = $derived(
    buildPerAppCatalog(installedApps, selectedPackages, detectedOverride),
  );
  let visibleApps = $derived(
    filterAndSortPerApps(catalog, {
      query: searchQuery,
      showSystem,
      onlySelected,
      selectedFirst: true,
    }),
  );
  let selectedCount = $derived(selectedPackages.length);

  async function refreshInstalledApps() {
    loadingApps = true;
    actionNotice = "";
    try {
      if (bridgeReady) {
        const res = await AndroidNetPlugin.listInstalledApps();
        if (Array.isArray(res?.apps) && res.apps.length > 0) {
          installedApps = res.apps;
          loadedFromDevice = true;
          hasLoadedOnce = true;
          return;
        }
      }
    } catch {
      // Desktop Linux / browser preview does not have Android PackageManager.
    } finally {
      loadingApps = false;
    }
    installedApps = fallbackPreviewInstalledApps();
    loadedFromDevice = false;
    hasLoadedOnce = true;
  }

  $effect(() => {
    if (mode !== "all" && !hasLoadedOnce && !loadingApps) {
      void refreshInstalledApps();
    }
  });

  function setMode(next: "all" | "whitelist" | "blacklist") {
    if (mode === next) return;
    mode = next;
    actionNotice = "";
    onChange();
  }

  function handleToggleApp(pkg: string) {
    selectedPackages = togglePackageSelection(selectedPackages, pkg);
    onChange();
  }

  function handleQuickSelectClassicCN() {
    if (mode === "all") mode = "blacklist";
    const { packages, addedCount } = selectClassicCNPackages(
      loadedFromDevice ? installedApps : [],
      selectedPackages,
    );
    selectedPackages = packages;
    actionNotice =
      addedCount > 0
        ? `已勾选 ${addedCount} 个常用中国应用`
        : "常用中国应用均已勾选";
    onChange();
  }

  async function handleAutoDetectCN() {
    if (mode === "all") mode = "blacklist";
    if (!hasLoadedOnce) {
      await refreshInstalledApps();
    }
    let detected: DetectedCNApp[] | undefined;
    if (bridgeReady && loadedFromDevice) {
      try {
        detected = await AppService.DetectCNApps(installedApps);
      } catch {
        detected = undefined;
      }
    }
    detectedOverride = detected;
    const res = selectDetectedCNPackages(
      installedApps,
      selectedPackages,
      detected,
    );
    selectedPackages = res.packages;
    actionNotice = `共识别 ${res.detectedCount} 个中国应用，新增勾选 ${res.addedCount} 个`;
    onChange();
  }

  function handleSelectAllVisible() {
    if (visibleApps.length === 0) return;
    selectedPackages = mergeUniquePackages(
      selectedPackages,
      visibleApps.map((a) => a.package_name),
    );
    onChange();
  }

  function handleClearSelected() {
    if (selectedPackages.length === 0) return;
    selectedPackages = [];
    actionNotice = "已清空所选应用";
    onChange();
  }

  function handleAddManualPackage() {
    const pkg = manualPkg.trim();
    if (!pkg) return;
    selectedPackages = mergeUniquePackages(selectedPackages, [pkg]);
    manualPkg = "";
    showManualInput = false;
    onChange();
  }

  function avatarInitial(label: string, pkg: string): string {
    const src = (label || pkg).trim();
    return src ? src.slice(0, 1).toUpperCase() : "?";
  }

  const AVATAR_TONES = [
    "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300",
    "bg-teal-100 text-teal-700 dark:bg-teal-950 dark:text-teal-300",
    "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300",
    "bg-indigo-100 text-indigo-700 dark:bg-indigo-950 dark:text-indigo-300",
    "bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  ];

  function avatarTone(pkg: string): string {
    let hash = 0;
    for (let i = 0; i < pkg.length; i++) {
      hash = (hash * 31 + pkg.charCodeAt(i)) | 0;
    }
    return AVATAR_TONES[Math.abs(hash) % AVATAR_TONES.length];
  }
</script>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
    <div>
      <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-100">
        分应用代理（Android）
      </h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        选择哪些应用走代理或直连，Looma 自身会自动排除以防流量回环
      </p>
    </div>
    {#if mode !== "all"}
      <span
        class="self-start rounded-md bg-zinc-100 px-2.5 py-1 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300 sm:self-auto"
      >
        已选 {selectedCount} / {catalog.length}
      </span>
    {/if}
  </div>

  <div
    role="tablist"
    aria-label="分应用代理模式"
    class="grid grid-cols-3 gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800/80"
  >
    <button
      type="button"
      role="tab"
      aria-selected={mode === "all"}
      onclick={() => setMode("all")}
      class="rounded-md px-2.5 py-1.5 text-xs font-medium transition sm:text-sm {mode ===
      'all'
        ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
        : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100'}"
    >
      全部应用
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={mode === "blacklist"}
      onclick={() => setMode("blacklist")}
      class="rounded-md px-2.5 py-1.5 text-xs font-medium transition sm:text-sm {mode ===
      'blacklist'
        ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
        : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100'}"
    >
      绕过选中
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={mode === "whitelist"}
      onclick={() => setMode("whitelist")}
      class="rounded-md px-2.5 py-1.5 text-xs font-medium transition sm:text-sm {mode ===
      'whitelist'
        ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
        : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100'}"
    >
      仅代理选中
    </button>
  </div>

  {#if mode === "all"}
    <p class="text-xs text-zinc-500 dark:text-zinc-400">
      当前所有应用均通过代理连接。切换到「绕过选中」或「仅代理选中」即可按应用逐一勾选。
    </p>
  {:else}
    <div class="space-y-2.5">
      <div class="flex flex-col gap-2 sm:flex-row">
        <div class="relative flex-1">
          <Search
            class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400"
          />
          <input
            type="search"
            bind:value={searchQuery}
            placeholder="搜索应用名称、包名或分类..."
            class="w-full rounded-lg border border-zinc-200 bg-zinc-50 py-2 pl-9 pr-8 text-sm text-zinc-900 outline-none transition focus:border-blue-500 focus:bg-white dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100 dark:focus:border-blue-500"
          />
          {#if searchQuery}
            <button
              type="button"
              aria-label="清除搜索"
              onclick={() => (searchQuery = "")}
              class="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
            >
              <X class="h-3.5 w-3.5" />
            </button>
          {/if}
        </div>
        <button
          type="button"
          disabled={loadingApps}
          onclick={refreshInstalledApps}
          class="inline-flex items-center justify-center gap-1.5 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-xs font-medium text-zinc-700 transition hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800"
        >
          <RefreshCw class="h-3.5 w-3.5 {loadingApps ? 'animate-spin' : ''}" />
          <span>{loadingApps ? "读取中..." : "刷新列表"}</span>
        </button>
      </div>

      <div class="flex flex-wrap items-center gap-1.5">
        <button
          type="button"
          onclick={handleQuickSelectClassicCN}
          class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 bg-zinc-50 px-2.5 py-1.5 text-xs font-medium text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-800/70 dark:text-zinc-200 dark:hover:bg-zinc-800"
        >
          <span>勾选常用中国应用</span>
        </button>
        <button
          type="button"
          onclick={handleAutoDetectCN}
          class="inline-flex items-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-2.5 py-1.5 text-xs font-medium text-blue-700 transition hover:bg-blue-100 dark:border-blue-900/60 dark:bg-blue-950/60 dark:text-blue-300 dark:hover:bg-blue-950"
        >
          <ScanSearch class="h-3.5 w-3.5" />
          <span>智能识别中国应用</span>
        </button>
        <button
          type="button"
          onclick={handleSelectAllVisible}
          class="rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs font-medium text-zinc-600 transition hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          全选当前
        </button>
        <button
          type="button"
          disabled={selectedCount === 0}
          onclick={handleClearSelected}
          class="rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs font-medium text-zinc-600 transition hover:bg-zinc-50 disabled:opacity-40 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          清空
        </button>
        <button
          type="button"
          onclick={() => (showManualInput = !showManualInput)}
          class="inline-flex items-center gap-1 rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs font-medium text-zinc-600 transition hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          <Plus class="h-3.5 w-3.5" />
          <span>手动添加</span>
        </button>
      </div>

      <div
        class="flex flex-wrap items-center justify-between gap-2 border-b border-zinc-100 pb-2 text-xs text-zinc-500 dark:border-zinc-800 dark:text-zinc-400"
      >
        <div class="flex items-center gap-3">
          <button
            type="button"
            onclick={() => (showSystem = !showSystem)}
            class="inline-flex items-center gap-1.5 transition hover:text-zinc-900 dark:hover:text-zinc-100"
          >
            <span
              class="flex h-3.5 w-3.5 items-center justify-center rounded border {showSystem
                ? 'border-blue-600 bg-blue-600 text-white'
                : 'border-zinc-300 dark:border-zinc-600'}"
            >
              {#if showSystem}
                <Check class="h-2.5 w-2.5" />
              {/if}
            </span>
            <span>显示系统应用</span>
          </button>
          <button
            type="button"
            onclick={() => (onlySelected = !onlySelected)}
            class="inline-flex items-center gap-1.5 transition hover:text-zinc-900 dark:hover:text-zinc-100"
          >
            <span
              class="flex h-3.5 w-3.5 items-center justify-center rounded border {onlySelected
                ? 'border-blue-600 bg-blue-600 text-white'
                : 'border-zinc-300 dark:border-zinc-600'}"
            >
              {#if onlySelected}
                <Check class="h-2.5 w-2.5" />
              {/if}
            </span>
            <span>仅看已选</span>
          </button>
        </div>
        {#if !loadedFromDevice}
          <span class="text-[11px] text-zinc-400 dark:text-zinc-500">
            桌面预览模式 · 展示常用应用目录
          </span>
        {/if}
      </div>

      {#if showManualInput}
        <div class="flex gap-2">
          <input
            type="text"
            bind:value={manualPkg}
            placeholder="输入完整包名，例如 com.example.app"
            onkeydown={(e) => e.key === "Enter" && handleAddManualPackage()}
            class="flex-1 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-1.5 font-mono text-xs text-zinc-900 outline-none focus:border-blue-500 focus:bg-white dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
          />
          <button
            type="button"
            onclick={handleAddManualPackage}
            class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
          >
            添加
          </button>
        </div>
      {/if}

      {#if actionNotice}
        <p class="text-xs text-blue-600 dark:text-blue-400">{actionNotice}</p>
      {/if}

      <div
        class="max-h-80 divide-y divide-zinc-100 overflow-y-auto rounded-lg border border-zinc-200 dark:divide-zinc-800/80 dark:border-zinc-800"
      >
        {#if visibleApps.length === 0}
          <div
            class="px-4 py-8 text-center text-xs text-zinc-500 dark:text-zinc-400"
          >
            {onlySelected
              ? "尚未勾选任何应用"
              : "未找到匹配的应用，可尝试切换搜索词或显示系统应用"}
          </div>
        {:else}
          {#each visibleApps as app (app.package_name)}
            <button
              type="button"
              role="checkbox"
              aria-checked={app.selected}
              onclick={() => handleToggleApp(app.package_name)}
              class="flex w-full items-center gap-3 px-3 py-2.5 text-left transition hover:bg-zinc-50 dark:hover:bg-zinc-800/60"
            >
              <span
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-semibold {avatarTone(
                  app.package_name,
                )}"
              >
                {avatarInitial(app.label, app.package_name)}
              </span>

              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5">
                  <span
                    class="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100"
                  >
                    {app.label}
                  </span>
                  {#if app.is_system}
                    <span
                      class="shrink-0 rounded bg-zinc-100 px-1.5 py-0.5 text-[10px] font-medium text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400"
                    >
                      系统
                    </span>
                  {/if}
                  {#if app.cn_reason}
                    <span
                      class="shrink-0 rounded bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-950/70 dark:text-blue-300"
                    >
                      {app.cn_reason}
                    </span>
                  {/if}
                </div>
                <p
                  class="truncate font-mono text-[11px] text-zinc-500 dark:text-zinc-400"
                >
                  {app.package_name}
                </p>
              </div>

              <span
                class="flex h-5 w-5 shrink-0 items-center justify-center rounded border transition {app.selected
                  ? 'border-blue-600 bg-blue-600 text-white'
                  : 'border-zinc-300 bg-white dark:border-zinc-600 dark:bg-zinc-900'}"
              >
                {#if app.selected}
                  <Check class="h-3.5 w-3.5" />
                {/if}
              </span>
            </button>
          {/each}
        {/if}
      </div>
    </div>
  {/if}
</section>
