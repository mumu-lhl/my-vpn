<script lang="ts">
  import { RefreshCw, Save } from "@lucide/svelte";
  import type { AppFormState } from "../lib/config_state";
  import type { ExternalNode, GephExitLocation } from "../lib/mygo";
  import { countryCodeToFlag } from "../lib/probe";

  interface Props {
    form: AppFormState;
    gephExits: GephExitLocation[];
    subName: string;
    subURL: string;
    sortedExternalNodes: ExternalNode[];
    testingNodes: boolean;
    onLoadGephExits: () => void;
    onAutoSave: () => void;
    onSave: () => void;
    onImportSub: () => void;
    onTestAllNodes: () => void;
  }

  let {
    form,
    gephExits,
    subName = $bindable(""),
    subURL = $bindable(""),
    sortedExternalNodes,
    testingNodes,
    onLoadGephExits,
    onAutoSave,
    onSave,
    onImportSub,
    onTestAllNodes,
  }: Props = $props();

  const QUICK_GEPH_REGIONS = [
    { code: "auto", label: "自动" },
    { code: "JP", label: "日本" },
    { code: "SG", label: "新加坡" },
    { code: "US", label: "美国" },
    { code: "DE", label: "德国" },
    { code: "NL", label: "荷兰" },
  ];
</script>

<section
  class="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-wrap items-center justify-between gap-2">
    <div>
      <h2 class="text-base font-semibold">迷雾通 (Geph5)</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        填写账号 Secret 后在主页连接迷雾通，连接后可拉取实时出口列表
      </p>
    </div>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
      onclick={onLoadGephExits}
    >
      <RefreshCw class="h-3.5 w-3.5" />
      <span>刷新实时出口</span>
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
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        出口国家 / 地区
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        bind:value={form.gephCountry}
        placeholder="auto / JP / US / SG"
      />
    </label>
  </div>

  <div class="flex flex-wrap gap-1.5 pt-1">
    {#if gephExits.length > 0}
      {#each gephExits as ex}
        <button
          type="button"
          class="rounded-md border px-2.5 py-1 text-xs transition {form.gephCountry.toUpperCase() ===
          ex.country.toUpperCase()
            ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300'
            : 'border-zinc-200 bg-zinc-50 hover:border-blue-400 dark:border-zinc-700 dark:bg-zinc-800'}"
          onclick={() => {
            form.gephCountry = ex.country;
            onAutoSave();
          }}
        >
          {countryCodeToFlag(ex.country)}
          {ex.country} · {ex.city}
        </button>
      {/each}
    {:else}
      {#each QUICK_GEPH_REGIONS as r}
        <button
          type="button"
          class="rounded-md border px-2.5 py-1 text-xs transition {form.gephCountry.toLowerCase() ===
          r.code.toLowerCase()
            ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300'
            : 'border-zinc-200 bg-zinc-50 hover:border-blue-400 dark:border-zinc-700 dark:bg-zinc-800'}"
          onclick={() => {
            form.gephCountry = r.code;
            onAutoSave();
          }}
        >
          {r.code === "auto" ? "🌐" : countryCodeToFlag(r.code)}
          {r.label}
        </button>
      {/each}
    {/if}
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
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={form.warpMode}
        onchange={onAutoSave}
      >
        <option value="auto">自动</option>
        <option value="wg">WireGuard</option>
        <option value="awg">AmneziaWG</option>
        <option value="masque">MASQUE (HTTP/3 &amp; HTTP/2)</option>
      </select>
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
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={form.warpMasqueVhttp}
        onchange={onAutoSave}
      >
        <option value="auto">自动（优先 HTTP/3，超时回退 HTTP/2）</option>
        <option value="h3">HTTP/3 (QUIC)</option>
        <option value="h2">HTTP/2 (TLS/TCP)</option>
      </select>
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
  <div class="grid grid-cols-1 gap-2 sm:grid-cols-4">
    <input
      class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
      type="text"
      bind:value={subName}
      placeholder="分组名称"
    />
    <input
      class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950 sm:col-span-2"
      type="url"
      bind:value={subURL}
      onkeydown={(e) => e.key === "Enter" && onImportSub()}
      placeholder="订阅地址或分享链接"
    />
    <button
      type="button"
      class="rounded-lg bg-blue-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-blue-700"
      onclick={onImportSub}
    >
      导入
    </button>
  </div>
  {#if sortedExternalNodes.length > 0}
    <div
      class="divide-y divide-zinc-100 rounded-lg border border-zinc-200 text-xs dark:divide-zinc-800 dark:border-zinc-800"
    >
      {#each sortedExternalNodes as node}
        <div class="flex items-center justify-between px-3 py-2">
          <div class="min-w-0">
            <span class="font-medium text-zinc-900 dark:text-zinc-100">
              {node.name}
            </span>
            <span class="ml-2 text-zinc-400">{node.protocol}</span>
          </div>
          <span class="font-mono text-zinc-500 dark:text-zinc-400">
            {node.latency_ms ? `${node.latency_ms} ms` : "--"}
          </span>
        </div>
      {/each}
    </div>
  {/if}
</section>
