<script lang="ts">
  import { onMount } from "svelte";
  import {
    ArrowDown,
    ArrowLeft,
    ArrowUp,
    Check,
    RefreshCw,
  } from "@lucide/svelte";
  import type { AppFormState } from "../lib/config_state";
  import type { VPNGateNode } from "../lib/mygo";
  import {
    defaultVPNGateSortDir,
    extractVPNGateCountries,
    filterVPNGateNodes,
    formatVPNGateCountry,
    formatVPNGateScore,
    formatVPNGateSpeed,
    formatVPNGateTraffic,
    sortVPNGateNodes,
    type VPNGateSortDir,
    type VPNGateSortField,
  } from "../lib/vpngate";

  interface Props {
    form: AppFormState;
    nodes: VPNGateNode[];
    connected: boolean;
    loading: boolean;
    switching: boolean;
    errorMessage: string;
    onBack: () => void;
    onLoadCache: () => void;
    onRefresh: () => void;
    onSelectNode: (node: VPNGateNode) => void;
  }

  let {
    form,
    nodes,
    connected,
    loading,
    switching,
    errorMessage,
    onBack,
    onLoadCache,
    onRefresh,
    onSelectNode,
  }: Props = $props();

  let selectedCountry = $state("ALL");
  let sortField = $state<VPNGateSortField>("score");
  let sortDir = $state<VPNGateSortDir>("desc");

  const SORT_OPTIONS: Array<{ field: VPNGateSortField; label: string }> = [
    { field: "score", label: "分数" },
    { field: "speed", label: "吞吐量" },
    { field: "ping", label: "Ping" },
    { field: "total_traffic", label: "累计流量" },
  ];

  let countries = $derived(extractVPNGateCountries(nodes));
  let filteredNodes = $derived(filterVPNGateNodes(nodes, selectedCountry));
  let visibleNodes = $derived(
    sortVPNGateNodes(filteredNodes, sortField, sortDir),
  );

  function toggleSort(field: VPNGateSortField) {
    if (sortField === field) {
      sortDir = sortDir === "desc" ? "asc" : "desc";
    } else {
      sortField = field;
      sortDir = defaultVPNGateSortDir(field);
    }
  }

  onMount(() => {
    onLoadCache();
  });
</script>

<div class="space-y-4">
  <div
    class="flex items-center justify-between gap-2 rounded-xl border border-zinc-200 bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div class="flex items-center gap-2">
      <button
        type="button"
        onclick={onBack}
        class="inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs font-medium text-zinc-600 transition hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
      >
        <ArrowLeft class="h-4 w-4" />
        <span>返回主页</span>
      </button>
      <span class="text-zinc-300 dark:text-zinc-700">/</span>
      <h1 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
        VPNGate (OpenVPN)
      </h1>
    </div>

    <button
      type="button"
      disabled={loading}
      aria-label="刷新 VPNGate 节点"
      title={connected ? "经由当前代理刷新节点" : "需先连接代理后才能拉取"}
      onclick={onRefresh}
      class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 text-zinc-600 transition hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
    >
      <RefreshCw class="h-4 w-4 {loading ? 'animate-spin' : ''}" />
    </button>
  </div>

  {#if !connected}
    <div
      class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200"
    >
      VPNGate API 需要经由代理拉取。当前未连接，下方展示本地缓存；如需刷新最新列表，请先连接任意可用线路后再点击右上角刷新图标。
    </div>
  {/if}

  {#if errorMessage}
    <div
      class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300"
    >
      {errorMessage}
    </div>
  {/if}

  <!-- Country filter & Sort toolbar -->
  <section
    class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-zinc-200 bg-white p-3.5 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <label class="flex items-center gap-2 text-xs">
      <span class="font-medium text-zinc-600 dark:text-zinc-300">国家</span>
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-2.5 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={selectedCountry}
      >
        <option value="ALL">🌐 全部国家 ({nodes.length})</option>
        {#each countries as c}
          <option value={c.code}>{c.flag} {c.name} ({c.count})</option>
        {/each}
      </select>
    </label>

    <div class="flex flex-wrap items-center gap-1.5">
      {#each SORT_OPTIONS as opt}
        {@const active = sortField === opt.field}
        <button
          type="button"
          onclick={() => toggleSort(opt.field)}
          class="inline-flex items-center gap-1 rounded-lg border px-2.5 py-1 text-xs font-medium transition {active
            ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300'
            : 'border-zinc-200 bg-zinc-50 text-zinc-600 hover:border-zinc-300 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300'}"
        >
          <span>{opt.label}</span>
          {#if active}
            {#if sortDir === "asc"}
              <ArrowUp class="h-3 w-3" />
            {:else}
              <ArrowDown class="h-3 w-3" />
            {/if}
          {/if}
        </button>
      {/each}
    </div>
  </section>

  <!-- Node list -->
  <section
    class="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"
  >
    {#if visibleNodes.length === 0}
      <div class="px-4 py-10 text-center text-xs text-zinc-500 dark:text-zinc-400">
        {loading ? "正在拉取 VPNGate 节点列表..." : "暂无节点数据，连接代理后点击右上角刷新图标拉取"}
      </div>
    {:else}
      <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
        {#each visibleNodes as node (node.id)}
          {@const country = formatVPNGateCountry(
            node.country_short,
            node.country_long,
          )}
          {@const selected = form.vpngateNode?.id === node.id}
          <button
            type="button"
            disabled={switching}
            onclick={() => onSelectNode(node)}
            class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition hover:bg-zinc-50 disabled:opacity-60 dark:hover:bg-zinc-800/60 {selected
              ? 'bg-blue-50/70 dark:bg-blue-950/30'
              : ''}"
          >
            <div class="flex min-w-0 items-center gap-3">
              <span class="text-xl leading-none">{country.flag}</span>
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <p class="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">
                    {country.name} · {node.host_name || node.ip}
                  </p>
                  <span
                    class="rounded bg-zinc-100 px-1.5 py-0.5 font-mono text-[10px] uppercase text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                  >
                    {node.network}
                  </span>
                </div>
                <p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500 dark:text-zinc-400">
                  {node.ip}:{node.server_port} · 分数 {formatVPNGateScore(node.score)} · 流量 {formatVPNGateTraffic(node.total_traffic)}
                </p>
              </div>
            </div>

            <div class="flex shrink-0 items-center gap-2">
              <div class="text-right font-mono text-[11px]">
                <p class="font-medium text-zinc-800 dark:text-zinc-200">
                  {formatVPNGateSpeed(node.speed)}
                </p>
                <p class="text-zinc-500 dark:text-zinc-400">
                  {node.ping > 0 ? `${node.ping} ms` : "--"}
                </p>
              </div>
              {#if selected}
                <Check class="h-4 w-4 text-blue-600 dark:text-blue-400" />
              {/if}
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </section>
</div>
