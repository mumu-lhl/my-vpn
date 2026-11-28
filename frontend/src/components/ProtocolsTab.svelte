<script lang="ts">
  import { ChevronRight, RefreshCw, Save, Trash2 } from "@lucide/svelte";
  import { formatBytes } from "../lib/chain";
  import type { AppFormState } from "../lib/config_state";
  import { formatGephExitSummary } from "../lib/geph_exits";
  import type { ExternalNode } from "../lib/mygo";
  import Select from "./Select.svelte";

  interface Props {
    form: AppFormState;
    subName: string;
    subURL: string;
    subIntervalHours: number;
    subProxyMode: "auto" | "proxy" | "direct";
    sortedExternalNodes: ExternalNode[];
    testingNodes: boolean;
    refreshingSubId: string;
    onOpenGephExits: () => void;
    onAutoSave: () => void;
    onSave: () => void;
    onImportSub: () => void;
    onRefreshSub: (subId: string) => void;
    onUpdateSubSettings: (
      subId: string,
      name: string,
      intervalHours: number,
      proxyMode: "auto" | "proxy" | "direct",
    ) => void;
    onDeleteSub: (subId: string) => void;
    onDeleteNode: (nodeId: string) => void;
    onTestAllNodes: () => void;
  }

  let {
    form,
    subName = $bindable(""),
    subURL = $bindable(""),
    subIntervalHours = $bindable(24),
    subProxyMode = $bindable("auto"),
    sortedExternalNodes,
    testingNodes,
    refreshingSubId,
    onOpenGephExits,
    onAutoSave,
    onSave,
    onImportSub,
    onRefreshSub,
    onUpdateSubSettings,
    onDeleteSub,
    onDeleteNode,
    onTestAllNodes,
  }: Props = $props();

  const SUB_INTERVAL_OPTIONS = [
    { value: "6", label: "每 6 小时" },
    { value: "12", label: "每 12 小时" },
    { value: "24", label: "每 24 小时" },
    { value: "72", label: "每 3 天" },
    { value: "0", label: "不自动更新" },
  ];

  const SUB_PROXY_OPTIONS = [
    { value: "auto", label: "自动（已连走代理）" },
    { value: "proxy", label: "仅走代理更新" },
    { value: "direct", label: "仅直连更新" },
  ];

  let gephSummary = $derived(
    formatGephExitSummary(form.gephCountry, form.gephCity),
  );
</script>

<section
  class="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-wrap items-center justify-between gap-2">
    <div>
      <h2 class="text-base font-semibold">迷雾通 (Geph5)</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        填写账号 Secret 后，无需连接即可在主页选择并拉取实时出口列表
      </p>
    </div>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
      onclick={onOpenGephExits}
    >
      <span>{gephSummary.flag} {gephSummary.label}</span>
      <ChevronRight class="h-3.5 w-3.5 text-zinc-400" />
    </button>
  </div>
  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        账号 Secret
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        type="password"
        bind:value={form.gephSecret}
        placeholder="粘贴已有账号 Secret"
      />
    </label>
    <div class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        当前出口节点
      </span>
      <button
        type="button"
        onclick={onOpenGephExits}
        class="flex items-center justify-between rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-sm hover:border-blue-400 dark:border-zinc-700 dark:bg-zinc-950"
      >
        <span class="truncate">{gephSummary.flag} {gephSummary.label}</span>
        <ChevronRight class="h-4 w-4 shrink-0 text-zinc-400" />
      </button>
    </div>
  </div>
</section>

<section
  class="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between">
    <h2 class="text-base font-semibold">赛风 &amp; Cloudflare WARP</h2>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
      onclick={onSave}
    >
      <Save class="h-3.5 w-3.5" />
      <span>保存协议设置</span>
    </button>
  </div>
  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        Psiphon 出口区域
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        bind:value={form.psiphonRegion}
        placeholder="auto / US / JP / DE"
      />
    </label>
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        WARP 隧道模式
      </span>
      <Select
        bind:value={form.warpMode}
        options={[
          { value: "auto", label: "自动" },
          { value: "wg", label: "WireGuard" },
          { value: "awg", label: "AmneziaWG" },
          { value: "masque", label: "MASQUE (HTTP/3 & HTTP/2)" },
        ]}
        onchange={onAutoSave}
      />
    </label>
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        WARP+ License Key
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        bind:value={form.warpLicense}
        placeholder="留空自动免费注册"
      />
    </label>
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        WireGuard 噪声模式
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        bind:value={form.warpNoiseMode}
      />
    </label>
    <label class="flex flex-col gap-1.5 text-sm sm:col-span-2">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        MASQUE HTTP 版本
      </span>
      <Select
        bind:value={form.warpMasqueVhttp}
        options={[
          { value: "auto", label: "自动（优先 HTTP/3，超时回退 HTTP/2）" },
          { value: "h3", label: "HTTP/3 (QUIC)" },
          { value: "h2", label: "HTTP/2 (TLS/TCP)" },
        ]}
        onchange={onAutoSave}
      />
    </label>
  </div>
</section>

<section
  class="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between">
    <h2 class="text-base font-semibold">外部订阅与节点</h2>
    {#if sortedExternalNodes.length > 0}
      <button
        type="button"
        disabled={testingNodes}
        class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
        onclick={onTestAllNodes}
      >
        <RefreshCw class="h-3.5 w-3.5 {testingNodes ? 'animate-spin' : ''}" />
        <span>批量测速</span>
      </button>
    {/if}
  </div>
  <div class="grid grid-cols-1 gap-2 sm:grid-cols-6">
    <input
      class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
      type="text"
      bind:value={subName}
      placeholder="分组名称（可选）"
    />
    <input
      class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950 sm:col-span-2"
      type="text"
      bind:value={subURL}
      onkeydown={(e) => e.key === "Enter" && onImportSub()}
      placeholder="订阅 URL 或 vless:// / vmess:// / ss:// 分享链接"
    />
    <Select
      size="xs"
      value={String(subIntervalHours)}
      options={SUB_INTERVAL_OPTIONS}
      onchange={(val) => (subIntervalHours = Number(val))}
    />
    <Select
      size="xs"
      value={subProxyMode}
      options={SUB_PROXY_OPTIONS}
      onchange={(val) => (subProxyMode = val as "auto" | "proxy" | "direct")}
    />
    <button
      type="button"
      class="rounded-lg bg-blue-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-blue-700"
      onclick={onImportSub}
    >
      导入
    </button>
  </div>

  {#if form.subscriptions.length > 0}
    <div class="space-y-2 pt-1">
      <p class="text-xs font-medium text-zinc-500 dark:text-zinc-400">
        已添加订阅（{form.subscriptions.length}）
      </p>
      <div
        class="divide-y divide-zinc-100 rounded-lg border border-zinc-200 text-xs dark:divide-zinc-800 dark:border-zinc-800"
      >
        {#each form.subscriptions as sub (sub.id)}
          {@const usedBytes =
            (sub.upload_bytes ?? 0) + (sub.download_bytes ?? 0)}
          <div
            class="flex flex-col gap-2 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between"
          >
            <div class="min-w-0 space-y-0.5">
              <div class="flex items-center gap-2">
                <span
                  class="truncate font-medium text-zinc-900 dark:text-zinc-100"
                >
                  {sub.name}
                </span>
                {#if sub.total_bytes && sub.total_bytes > 0}
                  <span
                    class="rounded bg-zinc-100 px-1.5 py-0.5 font-mono text-[10px] text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                  >
                    {formatBytes(usedBytes)} / {formatBytes(sub.total_bytes)}
                  </span>
                {/if}
              </div>
              <p class="truncate font-mono text-[11px] text-zinc-400">
                {sub.url}
              </p>
              {#if sub.last_error}
                <p class="text-[11px] text-rose-600 dark:text-rose-400">
                  上次更新失败：{sub.last_error}
                </p>
              {:else if sub.last_updated_unix > 0}
                <p class="text-[11px] text-zinc-400">
                  上次更新：{new Date(
                    sub.last_updated_unix * 1000,
                  ).toLocaleString()}
                </p>
              {/if}
            </div>

            <div class="flex shrink-0 flex-wrap items-center gap-1.5">
              <Select
                size="xs"
                class="w-28"
                value={String(sub.update_interval_hour)}
                options={SUB_INTERVAL_OPTIONS}
                onchange={(val) =>
                  onUpdateSubSettings(
                    sub.id,
                    sub.name,
                    Number(val),
                    (sub.proxy_mode ?? "auto") as "auto" | "proxy" | "direct",
                  )}
              />
              <Select
                size="xs"
                class="w-32"
                value={sub.proxy_mode ?? "auto"}
                options={SUB_PROXY_OPTIONS}
                onchange={(val) =>
                  onUpdateSubSettings(
                    sub.id,
                    sub.name,
                    sub.update_interval_hour,
                    val as "auto" | "proxy" | "direct",
                  )}
              />
              <button
                type="button"
                disabled={refreshingSubId === sub.id}
                aria-label="刷新订阅"
                title="立即更新订阅"
                onclick={() => onRefreshSub(sub.id)}
                class="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-zinc-200 text-zinc-600 hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                <RefreshCw
                  class="h-3.5 w-3.5 {refreshingSubId === sub.id
                    ? 'animate-spin'
                    : ''}"
                />
              </button>
              <button
                type="button"
                aria-label="删除订阅"
                title="删除订阅及其节点"
                onclick={() => onDeleteSub(sub.id)}
                class="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-zinc-200 text-rose-600 hover:bg-rose-50 dark:border-zinc-700 dark:text-rose-400 dark:hover:bg-rose-950/40"
              >
                <Trash2 class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if sortedExternalNodes.length > 0}
    <div
      class="divide-y divide-zinc-100 rounded-lg border border-zinc-200 text-xs dark:divide-zinc-800 dark:border-zinc-800"
    >
      {#each sortedExternalNodes as node}
        <div class="flex items-center justify-between gap-2 px-3 py-2">
          <div class="min-w-0">
            <span class="font-medium text-zinc-900 dark:text-zinc-100">
              {node.name}
            </span>
            <span class="ml-2 text-zinc-400">{node.protocol}</span>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span
              class="font-mono {node.latency_ms && node.latency_ms > 0
                ? 'text-emerald-600 dark:text-emerald-400'
                : node.latency_ms === -1
                  ? 'text-rose-500 dark:text-rose-400'
                  : 'text-zinc-500 dark:text-zinc-400'}"
            >
              {node.latency_ms && node.latency_ms > 0
                ? `${node.latency_ms} ms`
                : node.latency_ms === -1
                  ? "不可达"
                  : "--"}
            </span>
            <button
              type="button"
              aria-label="删除节点"
              title="删除节点"
              onclick={() => onDeleteNode(node.id)}
              class="inline-flex h-6 w-6 items-center justify-center rounded text-zinc-400 hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/40 dark:hover:text-rose-400"
            >
              <Trash2 class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
