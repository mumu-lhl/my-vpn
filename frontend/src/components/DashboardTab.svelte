<script lang="ts">
  import {
    ArrowDown,
    ArrowUp,
    ChevronRight,
    ExternalLink,
    Power,
    RefreshCw,
    ShieldAlert,
    SlidersHorizontal,
  } from "@lucide/svelte";
  import { formatBytes, type ChainAnalysis, type HopKind } from "../lib/chain";
  import type { AppFormState } from "../lib/config_state";
  import { formatGephExitSummary } from "../lib/geph_exits";
  import { formatVPNGateCountry } from "../lib/vpngate";
  import type {
    ExternalNode,
    GephBridgeStatus,
    VPNGateNode,
  } from "../lib/mygo";
  import type { PlatformUIProfile } from "../lib/platform";
  import {
    countryCodeToFlag,
    formatLatency,
    type ChainProbeResult,
  } from "../lib/probe";
  import Select from "./Select.svelte";
  import Toggle from "./Toggle.svelte";

  interface Props {
    form: AppFormState;
    platformUI: PlatformUIProfile;
    connected: boolean;
    busyConnect: boolean;
    switchingMode: boolean;
    lanIP: string;
    upBytes: number;
    downBytes: number;
    chainInfo: ChainAnalysis;
    gephBridgeStatus: GephBridgeStatus | null;
    sortedExternalNodes: ExternalNode[];
    backendWarnings: Array<{ code: string; message: string }>;
    probing: boolean;
    probeResult: ChainProbeResult | null;
    onToggleConnect: () => void;
    onSetInboundMode: (mode: "proxy" | "tun") => void;
    onChainChange: () => void;
    onAllowLANChange: () => void;
    onKillSwitchChange: () => void;
    onOpenAndroidVpnSettings: () => void;
    onOpenSettingsSubpage: (sub: "protocols" | "routing") => void;
    onOpenGephExits: () => void;
    onOpenVPNGate: () => void;
    onRunProbe: () => void;
  }

  let {
    form,
    platformUI,
    connected,
    busyConnect,
    switchingMode,
    lanIP,
    upBytes,
    downBytes,
    chainInfo,
    gephBridgeStatus,
    sortedExternalNodes,
    backendWarnings,
    probing,
    probeResult,
    onToggleConnect,
    onSetInboundMode,
    onChainChange,
    onAllowLANChange,
    onKillSwitchChange,
    onOpenAndroidVpnSettings,
    onOpenSettingsSubpage,
    onOpenGephExits,
    onOpenVPNGate,
    onRunProbe,
  }: Props = $props();

  let isDirectBypass = $derived(form.chain.hop1.kind === "direct_bypass");
  let gephSummary = $derived(
    formatGephExitSummary(form.gephCountry, form.gephCity),
  );
  let vpngateSummary = $derived.by(() => {
    if (!form.vpngateNode) {
      return { flag: "🌐", label: "未选择节点，点击拉取并选择" };
    }
    const c = formatVPNGateCountry(
      form.vpngateNode.country_short,
      form.vpngateNode.country_long,
    );
    return {
      flag: c.flag,
      label: `${c.name} · ${form.vpngateNode.host_name || form.vpngateNode.ip}`,
    };
  });

  const HOP1_OPTIONS = [
    { value: "direct_bypass", label: "直连绕过（无服务器 · 防干扰）" },
    { value: "geph", label: "迷雾通 (Geph5)" },
    { value: "psiphon", label: "赛风 (Psiphon · 仅 TCP)" },
    { value: "warp", label: "Cloudflare WARP" },
    { value: "vpngate", label: "VPNGate (OpenVPN)" },
    { value: "external", label: "外部节点" },
  ];

  const HOP2_OPTIONS = [
    { value: "warp", label: "Cloudflare WARP" },
    { value: "geph", label: "迷雾通 (Geph5)" },
    { value: "psiphon", label: "赛风 (Psiphon)" },
    { value: "vpngate", label: "VPNGate (OpenVPN)" },
    { value: "external", label: "外部节点" },
  ];

  let externalNodeOptions = $derived(
    sortedExternalNodes.map((node) => ({
      value: node.id,
      label: `${node.name} (${node.protocol})`,
    })),
  );

  function setHop(which: "hop1" | "hop2", kind: string) {
    const nextKind = kind as HopKind;
    const defaultNodeId =
      nextKind === "external" ? sortedExternalNodes[0]?.id : undefined;
    form.chain = {
      ...form.chain,
      two_hop:
        which === "hop1" && nextKind === "direct_bypass"
          ? false
          : form.chain.two_hop,
      [which]: { kind: nextKind, node_id: defaultNodeId },
    };
    onChainChange();
  }

  function setHopNode(which: "hop1" | "hop2", nodeId: string) {
    form.chain = {
      ...form.chain,
      [which]: { ...form.chain[which], node_id: nodeId || undefined },
    };
    onChainChange();
  }
</script>

<!-- Hero VPN Connection Card -->
<section
  class="rounded-2xl border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-col items-center text-center">
    <div class="mb-4 flex items-center gap-2">
      <span
        class="inline-block h-2.5 w-2.5 rounded-full {connected
          ? 'bg-emerald-500'
          : 'bg-zinc-300 dark:bg-zinc-600'}"
      ></span>
      <span class="text-sm font-medium text-zinc-700 dark:text-zinc-200">
        {busyConnect
          ? connected
            ? "正在断开..."
            : "正在连接..."
          : connected
            ? "已保护连接"
            : "未连接"}
      </span>
      <span
        class="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
      >
        {form.inboundMode === "tun" ? platformUI.tunBadgeLabel : "本地代理"}
      </span>
    </div>

    <button
      type="button"
      disabled={busyConnect || switchingMode}
      aria-label={connected ? "断开连接" : "开始连接"}
      onclick={onToggleConnect}
      class="group relative flex h-28 w-28 items-center justify-center rounded-full border-4 transition-all active:scale-95 disabled:opacity-60 {connected
        ? 'border-emerald-200 bg-emerald-600 text-white shadow-lg shadow-emerald-600/20 hover:bg-emerald-700 dark:border-emerald-900/70'
        : 'border-blue-100 bg-blue-600 text-white shadow-lg shadow-blue-600/20 hover:bg-blue-700 dark:border-blue-950'}"
    >
      <Power class="h-10 w-10" />
    </button>

    <p class="mt-4 text-sm font-medium text-zinc-900 dark:text-zinc-100">
      {chainInfo.label}
    </p>
    {#if connected && (form.chain.hop1.kind === "geph" || (form.chain.two_hop && form.chain.hop2.kind === "geph"))}
      <div
        class="mt-2 inline-flex items-center gap-1.5 rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs text-zinc-700 dark:border-zinc-700 dark:bg-zinc-800/80 dark:text-zinc-200"
      >
        <span class="text-zinc-500 dark:text-zinc-400">迷雾通网桥</span>
        {#if gephBridgeStatus?.state === "Connected"}
          {#if gephBridgeStatus.bridge_ip}
            <span>
              {gephBridgeStatus.country_code
                ? countryCodeToFlag(gephBridgeStatus.country_code)
                : "🌐"}
            </span>
            <span class="font-mono font-medium">
              {gephBridgeStatus.bridge_ip}
            </span>
          {:else}
            <span>直连出口</span>
          {/if}
        {:else}
          <span class="text-zinc-500 dark:text-zinc-400">正在建立网桥...</span>
        {/if}
      </div>
    {/if}
    <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
      {connected ? "点击按钮断开当前连接" : "点击按钮立即建立加密或防干扰连接"}
    </p>

    {#if chainInfo.warning}
      <p
        class="mt-3 flex items-center gap-1.5 rounded-lg bg-amber-50 px-3 py-1.5 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
      >
        <ShieldAlert class="h-3.5 w-3.5 shrink-0" />
        <span>{chainInfo.warning}</span>
      </p>
    {/if}
    {#each backendWarnings as w}
      <p
        class="mt-2 flex items-center gap-1.5 rounded-lg bg-amber-50 px-3 py-1.5 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
      >
        <ShieldAlert class="h-3.5 w-3.5 shrink-0" />
        <span>{w.message}</span>
      </p>
    {/each}
  </div>

  <div
    class="mt-6 grid grid-cols-2 gap-3 border-t border-zinc-100 pt-4 dark:border-zinc-800 sm:grid-cols-4"
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

<!-- Inline Route / Chain Selector (instant auto-save, no Save button) -->
<section
  class="space-y-3.5 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between gap-2">
    <h2 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
      连接线路
    </h2>
    {#if isDirectBypass}
      <button
        type="button"
        onclick={() => onOpenSettingsSubpage("routing")}
        class="inline-flex items-center gap-1 text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
      >
        <SlidersHorizontal class="h-3.5 w-3.5" />
        <span>反 DPI 策略</span>
      </button>
    {:else}
      <button
        type="button"
        onclick={() => onOpenSettingsSubpage("protocols")}
        class="inline-flex items-center gap-1 text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
      >
        <SlidersHorizontal class="h-3.5 w-3.5" />
        <span>节点与协议设置</span>
      </button>
    {/if}
  </div>

  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
    <div class="space-y-2">
      <label class="flex flex-col gap-1.5 text-sm">
        <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
          {isDirectBypass
            ? "工作模式"
            : form.chain.two_hop
              ? "第一跳（前置）"
              : "出口线路"}
        </span>
        <Select
          value={form.chain.hop1.kind}
          options={HOP1_OPTIONS}
          onchange={(val) => setHop("hop1", val)}
        />
      </label>
      {#if form.chain.hop1.kind === "external"}
        {#if sortedExternalNodes.length > 0}
          <Select
            size="xs"
            value={form.chain.hop1.node_id ?? ""}
            options={externalNodeOptions}
            onchange={(val) => setHopNode("hop1", val)}
          />
        {:else}
          <button
            type="button"
            onclick={() => onOpenSettingsSubpage("protocols")}
            class="text-xs text-blue-600 hover:underline dark:text-blue-400"
          >
            尚未导入外部节点，前往「设置 → 节点与协议」添加 →
          </button>
        {/if}
      {:else if form.chain.hop1.kind === "geph"}
        <button
          type="button"
          onclick={onOpenGephExits}
          class="flex w-full items-center justify-between gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-xs transition hover:border-blue-400 hover:bg-zinc-100/70 dark:border-zinc-700 dark:bg-zinc-950 dark:hover:border-blue-500 dark:hover:bg-zinc-900"
        >
          <span class="flex min-w-0 items-center gap-2">
            <span class="text-base leading-none">{gephSummary.flag}</span>
            <span class="truncate font-medium text-zinc-800 dark:text-zinc-200">
              {gephSummary.label}
            </span>
          </span>
          <ChevronRight class="h-3.5 w-3.5 shrink-0 text-zinc-400" />
        </button>
      {:else if form.chain.hop1.kind === "vpngate"}
        <button
          type="button"
          onclick={onOpenVPNGate}
          class="flex w-full items-center justify-between gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-xs transition hover:border-blue-400 hover:bg-zinc-100/70 dark:border-zinc-700 dark:bg-zinc-950 dark:hover:border-blue-500 dark:hover:bg-zinc-900"
        >
          <span class="flex min-w-0 items-center gap-2">
            <span class="text-base leading-none">{vpngateSummary.flag}</span>
            <span class="truncate font-medium text-zinc-800 dark:text-zinc-200">
              {vpngateSummary.label}
            </span>
          </span>
          <ChevronRight class="h-3.5 w-3.5 shrink-0 text-zinc-400" />
        </button>
      {/if}
    </div>

    {#if form.chain.two_hop && !isDirectBypass}
      <div class="space-y-2">
        <label class="flex flex-col gap-1.5 text-sm">
          <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
            第二跳（落地）
          </span>
          <Select
            value={form.chain.hop2.kind}
            options={HOP2_OPTIONS}
            onchange={(val) => setHop("hop2", val)}
          />
        </label>
        {#if form.chain.hop2.kind === "external"}
          {#if sortedExternalNodes.length > 0}
            <Select
              size="xs"
              value={form.chain.hop2.node_id ?? ""}
              options={externalNodeOptions}
              onchange={(val) => setHopNode("hop2", val)}
            />
          {:else}
            <button
              type="button"
              onclick={() => onOpenSettingsSubpage("protocols")}
              class="text-xs text-blue-600 hover:underline dark:text-blue-400"
            >
              尚未导入外部节点，前往「设置 → 节点与协议」添加 →
            </button>
          {/if}
        {:else if form.chain.hop2.kind === "geph"}
          <button
            type="button"
            onclick={onOpenGephExits}
            class="flex w-full items-center justify-between gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-xs transition hover:border-blue-400 hover:bg-zinc-100/70 dark:border-zinc-700 dark:bg-zinc-950 dark:hover:border-blue-500 dark:hover:bg-zinc-900"
          >
            <span class="flex min-w-0 items-center gap-2">
              <span class="text-base leading-none">{gephSummary.flag}</span>
              <span
                class="truncate font-medium text-zinc-800 dark:text-zinc-200"
              >
                {gephSummary.label}
              </span>
            </span>
            <ChevronRight class="h-3.5 w-3.5 shrink-0 text-zinc-400" />
          </button>
        {:else if form.chain.hop2.kind === "vpngate"}
          <button
            type="button"
            onclick={onOpenVPNGate}
            class="flex w-full items-center justify-between gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-xs transition hover:border-blue-400 hover:bg-zinc-100/70 dark:border-zinc-700 dark:bg-zinc-950 dark:hover:border-blue-500 dark:hover:bg-zinc-900"
          >
            <span class="flex min-w-0 items-center gap-2">
              <span class="text-base leading-none">{vpngateSummary.flag}</span>
              <span
                class="truncate font-medium text-zinc-800 dark:text-zinc-200"
              >
                {vpngateSummary.label}
              </span>
            </span>
            <ChevronRight class="h-3.5 w-3.5 shrink-0 text-zinc-400" />
          </button>
        {/if}
      </div>
    {/if}
  </div>

  {#if !isDirectBypass}
    <div class="border-t border-zinc-100 pt-1 dark:border-zinc-800">
      <Toggle
        bind:checked={form.chain.two_hop}
        label="启用双跳链式代理"
        description="流量先经第一跳前置通道，再由第二跳落地访问目标网站"
        onchange={onChainChange}
      />
    </div>
  {:else}
    <p
      class="rounded-lg bg-zinc-50 px-3 py-2 text-xs text-zinc-600 dark:bg-zinc-800/70 dark:text-zinc-300"
    >
      无需服务器，通过静态 IP DoH 与反 DPI 分片混淆直连受干扰网站。
    </p>
  {/if}
</section>

<!-- Access Mode & Protection Card -->
<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between gap-2">
    <div>
      <h2 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
        接入模式与防护
      </h2>
      {#if switchingMode}
        <p class="mt-0.5 text-xs text-blue-600 dark:text-blue-400">
          正在切换并重连...
        </p>
      {/if}
    </div>
    <div
      role="tablist"
      aria-label="接入模式"
      class="inline-grid grid-cols-2 gap-1 rounded-lg bg-zinc-100 p-0.5 dark:bg-zinc-800"
    >
      <button
        type="button"
        role="tab"
        disabled={switchingMode}
        aria-selected={form.inboundMode === "proxy"}
        onclick={() => onSetInboundMode("proxy")}
        class="rounded-md px-3 py-1 text-xs font-medium transition disabled:opacity-50 {form.inboundMode ===
        'proxy'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
      >
        本地代理
      </button>
      <button
        type="button"
        role="tab"
        disabled={switchingMode}
        aria-selected={form.inboundMode === "tun"}
        onclick={() => onSetInboundMode("tun")}
        class="rounded-md px-3 py-1 text-xs font-medium transition disabled:opacity-50 {form.inboundMode ===
        'tun'
          ? 'bg-white text-zinc-900 shadow-xs dark:bg-zinc-700 dark:text-zinc-100'
          : 'text-zinc-600 hover:text-zinc-900 dark:text-zinc-400'}"
      >
        {platformUI.tunModeLabel}
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

    {#if platformUI.killSwitchMode === "toggle"}
      <Toggle
        bind:checked={form.killSwitch}
        label="断网保护（Kill Switch）"
        description="隧道异常中断时自动阻断非直连明文流量，防止真实 IP 泄露"
        onchange={onKillSwitchChange}
      />
    {:else}
      <button
        type="button"
        onclick={onOpenAndroidVpnSettings}
        class="flex w-full items-center justify-between gap-3 py-3 text-left transition hover:opacity-80"
      >
        <div class="min-w-0">
          <p class="text-sm font-medium text-zinc-900 dark:text-zinc-100">
            断网保护（系统 VPN 设置）
          </p>
          <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
            前往 Android 系统开启「始终开启的 VPN」与「阻断非 VPN 连接」
          </p>
        </div>
        <span
          class="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-blue-600 dark:text-blue-400"
        >
          <ExternalLink class="h-3.5 w-3.5" />
          <ChevronRight class="h-4 w-4" />
        </span>
      </button>
    {/if}
  </div>
</section>

<!-- Connectivity & Exit Probe Card -->
<section
  class="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between gap-2">
    <h2 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
      链路连通性与出口检测
    </h2>
    <button
      type="button"
      disabled={probing}
      aria-label="检测延迟"
      title="检测延迟"
      class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 bg-white text-zinc-700 transition hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800"
      onclick={onRunProbe}
    >
      <RefreshCw class="h-4 w-4 {probing ? 'animate-spin' : ''}" />
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
      点击「检测延迟」查看{form.chain.two_hop && !isDirectBypass
        ? "每一跳"
        : "当前出口"}的连通延迟、归属地与出口 IP。
    </p>
  {/if}
</section>
