<script lang="ts">
  import { onMount } from "svelte";
  import {
    ArrowLeft,
    Check,
    RefreshCw,
    SlidersHorizontal,
  } from "@lucide/svelte";
  import type { AppFormState } from "../lib/config_state";
  import {
    formatGephCountryName,
    formatGephExitLoad,
    isGephExitSelected,
    sortGephExits,
  } from "../lib/geph_exits";
  import type { GephExitLocation } from "../lib/mygo";
  import { countryCodeToFlag } from "../lib/probe";

  interface Props {
    form: AppFormState;
    gephExits: GephExitLocation[];
    loading: boolean;
    switching: boolean;
    errorMessage: string;
    lastUpdatedAt: number;
    onBack: () => void;
    onRefresh: () => void;
    onSelectExit: (country: string, city: string) => void;
    onOpenProtocolsSettings: () => void;
  }

  let {
    form,
    gephExits,
    loading,
    switching,
    errorMessage,
    lastUpdatedAt,
    onBack,
    onRefresh,
    onSelectExit,
    onOpenProtocolsSettings,
  }: Props = $props();

  let sortedExits = $derived(sortGephExits(gephExits));
  let isAutoSelected = $derived(
    isGephExitSelected(form.gephCountry, form.gephCity, "auto", ""),
  );

  onMount(() => {
    onRefresh();
    const timer = window.setInterval(() => {
      if (!document.hidden) {
        onRefresh();
      }
    }, 30000);
    return () => window.clearInterval(timer);
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
        迷雾通出口节点
      </h1>
    </div>

    <div class="flex items-center gap-2">
      {#if lastUpdatedAt > 0}
        <span class="hidden text-[11px] text-zinc-400 sm:inline">
          每 30 秒自动刷新
        </span>
      {/if}
      <button
        type="button"
        disabled={loading}
        aria-label="手动刷新出口列表"
        title="手动刷新出口列表"
        onclick={onRefresh}
        class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 text-zinc-600 transition hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
      >
        <RefreshCw class="h-4 w-4 {loading ? 'animate-spin' : ''}" />
      </button>
    </div>
  </div>

  {#if !form.gephSecret.trim()}
    <div
      class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200"
    >
      <span
        >尚未填写迷雾通账号 Secret，填写后即可在不连接 VPN
        的情况下拉取实时出口列表。</span
      >
      <button
        type="button"
        onclick={onOpenProtocolsSettings}
        class="inline-flex items-center gap-1.5 rounded-lg bg-amber-600 px-3 py-1.5 font-medium text-white hover:bg-amber-700"
      >
        <SlidersHorizontal class="h-3.5 w-3.5" />
        <span>前往填写 Secret</span>
      </button>
    </div>
  {/if}

  {#if errorMessage}
    <div
      class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300"
    >
      {errorMessage}
    </div>
  {/if}

  <section
    class="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
      <button
        type="button"
        disabled={switching}
        onclick={() => onSelectExit("auto", "")}
        class="flex w-full items-center justify-between gap-3 px-4 py-3.5 text-left transition hover:bg-zinc-50 disabled:opacity-60 dark:hover:bg-zinc-800/60 {isAutoSelected
          ? 'bg-blue-50/70 dark:bg-blue-950/30'
          : ''}"
      >
        <div class="flex min-w-0 items-center gap-3">
          <span class="text-xl leading-none">🌐</span>
          <div class="min-w-0">
            <p class="text-sm font-medium text-zinc-900 dark:text-zinc-100">
              自动选择（智能路由）
            </p>
            <p class="text-xs text-zinc-500 dark:text-zinc-400">
              由 Geph5 自动挑选低负载最优出口
            </p>
          </div>
        </div>
        {#if isAutoSelected}
          <Check class="h-4 w-4 shrink-0 text-blue-600 dark:text-blue-400" />
        {/if}
      </button>

      {#if sortedExits.length === 0}
        <div
          class="px-4 py-8 text-center text-xs text-zinc-500 dark:text-zinc-400"
        >
          {loading
            ? "正在拉取迷雾通实时出口列表..."
            : "暂无出口节点数据，可点击右上角刷新图标重试"}
        </div>
      {:else}
        {#each sortedExits as ex}
          {@const selected = isGephExitSelected(
            form.gephCountry,
            form.gephCity,
            ex.country,
            ex.city,
          )}
          {@const loadInfo = formatGephExitLoad(ex.load)}
          <button
            type="button"
            disabled={switching}
            onclick={() => onSelectExit(ex.country, ex.city)}
            class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition hover:bg-zinc-50 disabled:opacity-60 dark:hover:bg-zinc-800/60 {selected
              ? 'bg-blue-50/70 dark:bg-blue-950/30'
              : ''}"
          >
            <div class="flex min-w-0 items-center gap-3">
              <span class="text-xl leading-none">
                {countryCodeToFlag(ex.country)}
              </span>
              <div class="min-w-0">
                <p
                  class="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100"
                >
                  {formatGephCountryName(ex.country)}{ex.city
                    ? ` · ${ex.city}`
                    : ""}
                </p>
                <p class="text-xs text-zinc-500 dark:text-zinc-400">
                  {ex.country.toUpperCase()}
                </p>
              </div>
            </div>

            <div class="flex shrink-0 items-center gap-2.5">
              <span
                class="rounded-full px-2 py-0.5 font-mono text-[11px] font-medium {loadInfo.level ===
                'low'
                  ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300'
                  : loadInfo.level === 'medium'
                    ? 'bg-amber-50 text-amber-700 dark:bg-amber-950/60 dark:text-amber-300'
                    : loadInfo.level === 'high'
                      ? 'bg-rose-50 text-rose-700 dark:bg-rose-950/60 dark:text-rose-300'
                      : 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300'}"
              >
                负载 {loadInfo.percentText}
              </span>
              {#if selected}
                <Check class="h-4 w-4 text-blue-600 dark:text-blue-400" />
              {/if}
            </div>
          </button>
        {/each}
      {/if}
    </div>
  </section>
</div>
