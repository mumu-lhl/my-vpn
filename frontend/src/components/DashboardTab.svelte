<script lang="ts">
  import {
    ArrowDown,
    ArrowUp,
    Power,
    RefreshCw,
    ShieldAlert,
  } from "@lucide/svelte";
  import { formatBytes, type ChainAnalysis } from "../lib/chain";
  import type { AppFormState } from "../lib/config_state";
  import { formatLatency, type ChainProbeResult } from "../lib/probe";
  import Toggle from "./Toggle.svelte";

  interface Props {
    form: AppFormState;
    connected: boolean;
    busyConnect: boolean;
    lanIP: string;
    upBytes: number;
    downBytes: number;
    chainInfo: ChainAnalysis;
    backendWarnings: Array<{ code: string; message: string }>;
    activeProbeURL: string;
    probing: boolean;
    probeResult: ChainProbeResult | null;
    onToggleConnect: () => void;
    onSetInboundMode: (mode: "proxy" | "tun") => void;
    onAllowLANChange: () => void;
    onKillSwitchChange: () => void;
    onRunProbe: () => void;
  }

  let {
    form,
    connected,
    busyConnect,
    lanIP,
    upBytes,
    downBytes,
    chainInfo,
    backendWarnings,
    activeProbeURL,
    probing,
    probeResult,
    onToggleConnect,
    onSetInboundMode,
    onAllowLANChange,
    onKillSwitchChange,
    onRunProbe,
  }: Props = $props();
</script>

<section
  class="rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
    <div class="space-y-1">
      <div class="flex items-center gap-2">
        <span
          class="inline-block h-2.5 w-2.5 rounded-full {connected
            ? 'bg-emerald-500'
            : 'bg-zinc-300 dark:bg-zinc-600'}"
        ></span>
        <h1 class="text-lg font-semibold">
          {connected ? "已连接" : "未连接"}
        </h1>
        <span
          class="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
        >
          {form.inboundMode === "tun" ? "TUN 模式" : "本地代理"}
        </span>
      </div>
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        {chainInfo.label}
      </p>
      {#if chainInfo.warning}
        <p
          class="flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400"
        >
          <ShieldAlert class="h-3.5 w-3.5 shrink-0" />
          <span>{chainInfo.warning}</span>
        </p>
      {/if}
      {#each backendWarnings as w}
        <p
          class="flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400"
        >
          <ShieldAlert class="h-3.5 w-3.5 shrink-0" />
          <span>{w.message}</span>
        </p>
      {/each}
    </div>

    <button
      type="button"
      disabled={busyConnect}
      class="inline-flex items-center justify-center gap-2 rounded-lg px-5 py-2.5 text-sm font-medium text-white transition disabled:opacity-50 {connected
        ? 'bg-red-600 hover:bg-red-700'
        : 'bg-blue-600 hover:bg-blue-700'}"
      onclick={onToggleConnect}
    >
      <Power class="h-4 w-4" />
      <span>{connected ? "断开" : "连接"}</span>
    </button>
  </div>

  <div
    class="mt-4 grid grid-cols-2 gap-3 border-t border-zinc-100 pt-4 dark:border-zinc-800 sm:grid-cols-4"
  >
    <div>
      <p class="text-xs text-zinc-500 dark:text-zinc-400">上行流量</p>
      <p class="mt-0.5 flex items-center gap-1 font-mono text-sm font-medium">
        <ArrowUp class="h-3.5 w-3.5 text-zinc-400" />
        <span>{formatBytes(upBytes)}</span>
      </p>
    </div>
    <div>
      <p class="text-xs text-zinc-500 dark:text-zinc-400">下行流量</p>
      <p class="mt-0.5 flex items-center gap-1 font-mono text-sm font-medium">
        <ArrowDown class="h-3.5 w-3.5 text-zinc-400" />
        <span>{formatBytes(downBytes)}</span>
      </p>
    </div>
    <div>
      <p class="text-xs text-zinc-500 dark:text-zinc-400">SOCKS5 / HTTP</p>
      <p class="mt-0.5 font-mono text-sm font-medium">
        {form.socksPort} / {form.httpPort}
      </p>
    </div>
    <div>
      <p class="text-xs text-zinc-500 dark:text-zinc-400">局域网共享</p>
      <p class="mt-0.5 font-mono text-sm font-medium">
        {form.allowLAN ? `${lanIP}:${form.socksPort}` : "仅本机"}
      </p>
    </div>
  </div>
</section>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between gap-2">
    <h2 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
      接入模式与防护
    </h2>
    <div
      role="tablist"
      aria-label="接入模式"
      class="inline-grid grid-cols-2 gap-1 rounded-lg bg-zinc-100 p-0.5 dark:bg-zinc-800"
    >
      <button
        type="button"
        role="tab"
        aria-selected={form.inboundMode === "proxy"}
        onclick={() => onSetInboundMode("proxy")}
        class="rounded-md px-3 py-1 text-xs font-medium transition {form.inboundMode ===
        'proxy'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
      >
        本地代理
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={form.inboundMode === "tun"}
        onclick={() => onSetInboundMode("tun")}
        class="rounded-md px-3 py-1 text-xs font-medium transition {form.inboundMode ===
        'tun'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
      >
        全局 TUN
      </button>
    </div>
  </div>

  <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
    <Toggle
      bind:checked={form.allowLAN}
      label="允许局域网设备共享代理"
      description={form.allowLAN
        ? `局域网内设备可连接 ${lanIP}:${form.socksPort}（SOCKS5）或 ${lanIP}:${form.httpPort}（HTTP）`
        : "开启后允许同局域网的其他设备通过本机代理上网"}
      onchange={onAllowLANChange}
    />
    <Toggle
      bind:checked={form.killSwitch}
      label="断网保护（Kill Switch）"
      description="隧道异常中断时自动阻断非直连明文流量，防止真实 IP 泄露"
      onchange={onKillSwitchChange}
    />
  </div>
</section>

<section
  class="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
    <div>
      <h2 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
        链路连通性与出口检测
      </h2>
      <p class="mt-0.5 truncate text-xs text-zinc-500 dark:text-zinc-400">
        {activeProbeURL}
      </p>
    </div>
    <button
      type="button"
      disabled={probing}
      class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 transition hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800"
      onclick={onRunProbe}
    >
      <RefreshCw class="h-3.5 w-3.5 {probing ? 'animate-spin' : ''}" />
      <span>{probing ? "检测中..." : "检测各跳延迟"}</span>
    </button>
  </div>

  {#if probeResult && probeResult.hops.length > 0}
    <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
      {#each probeResult.hops as hop}
        <div
          class="space-y-1.5 rounded-lg border border-zinc-200 bg-zinc-50/70 p-3.5 dark:border-zinc-800 dark:bg-zinc-950/60"
        >
          <div class="flex items-center justify-between gap-2">
            <span
              class="text-xs font-semibold text-zinc-700 dark:text-zinc-200"
            >
              {hop.label}
            </span>
            <span
              class="rounded px-2 py-0.5 font-mono text-xs font-medium {hop
                .connectivity.reachable
                ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/70 dark:text-emerald-300'
                : 'bg-red-50 text-red-700 dark:bg-red-950/70 dark:text-red-300'}"
            >
              {formatLatency(hop.connectivity)}
            </span>
          </div>
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="text-base leading-none"
              >{hop.exit_ip.flag || "🌐"}</span
            >
            <span class="font-mono text-xs font-semibold"
              >{hop.exit_ip.country_code || "--"}</span
            >
            <span class="font-mono text-xs text-zinc-600 dark:text-zinc-300"
              >{hop.exit_ip.ip || "--"}</span
            >
          </div>
          {#if hop.exit_ip.city || hop.exit_ip.org}
            <p class="text-xs text-zinc-500 dark:text-zinc-400">
              {[hop.exit_ip.city, hop.exit_ip.org].filter(Boolean).join(" · ")}
            </p>
          {:else if hop.exit_ip.error}
            <p class="text-xs text-amber-700 dark:text-amber-400">
              {hop.exit_ip.error}
            </p>
          {/if}
        </div>
      {/each}
    </div>
  {:else}
    <p class="text-xs text-zinc-500 dark:text-zinc-400">
      点击「检测各跳延迟」查看{form.chain.two_hop
        ? "每一跳"
        : "当前出口"}的连通延迟、归属地与出口 IP。
    </p>
  {/if}
</section>
