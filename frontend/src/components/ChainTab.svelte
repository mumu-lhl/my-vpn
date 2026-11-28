<script lang="ts">
  import { Save, ShieldAlert } from "@lucide/svelte";
  import type { ChainAnalysis, ChainConfig, HopKind } from "../lib/chain";
  import type { ExternalNode } from "../lib/mygo";
  import Select from "./Select.svelte";
  import Toggle from "./Toggle.svelte";

  interface Props {
    chain: ChainConfig;
    chainInfo: ChainAnalysis;
    sortedExternalNodes: ExternalNode[];
    onSave: () => void;
    onGoProtocols: () => void;
  }

  let {
    chain = $bindable(),
    chainInfo,
    sortedExternalNodes,
    onSave,
    onGoProtocols,
  }: Props = $props();

  let isDirectBypass = $derived(chain.hop1.kind === "direct_bypass");
  let externalNodeOptions = $derived([
    { value: "auto", label: "⚡ 自动选择优质节点（全部）" },
    ...sortedExternalNodes.map((node) => ({
      value: node.id,
      label: `${node.name} (${node.protocol})${node.latency_ms && node.latency_ms > 0 ? ` · ${node.latency_ms}ms` : ""}`,
    })),
  ]);

  function setHop(which: "hop1" | "hop2", kind: string) {
    const nextKind = kind as HopKind;
    const defaultNodeId = nextKind === "external" ? "auto" : undefined;
    chain = {
      ...chain,
      two_hop:
        which === "hop1" && nextKind === "direct_bypass"
          ? false
          : chain.two_hop,
      [which]: { kind: nextKind, node_id: defaultNodeId },
    };
  }

  function setHopNode(which: "hop1" | "hop2", nodeId: string) {
    chain = {
      ...chain,
      [which]: { ...chain[which], node_id: nodeId || undefined },
    };
  }
</script>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-base font-semibold">代理链路编排</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        支持无服务器直连绕过、单跳直出或两跳链式转发
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

  {#if !isDirectBypass}
    <Toggle
      bind:checked={chain.two_hop}
      label="启用二级链式代理"
      description="流量先经第一跳，再由第二跳落地访问目标网站"
    />
  {/if}

  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
    <div class="space-y-2">
      <label class="flex flex-col gap-1.5 text-sm">
        <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
          {isDirectBypass ? "工作模式" : "第一跳（前置）"}
        </span>
        <Select
          value={chain.hop1.kind}
          options={[
            { value: "direct_bypass", label: "直连绕过（无服务器 · 防干扰）" },
            { value: "geph", label: "迷雾通 (Geph5)" },
            { value: "psiphon", label: "赛风 (Psiphon · 仅 TCP)" },
            { value: "warp", label: "Cloudflare WARP" },
            { value: "vpngate", label: "VPNGate (OpenVPN)" },
            { value: "external", label: "外部节点" },
          ]}
          onchange={(val) => setHop("hop1", val)}
        />
      </label>
      {#if chain.hop1.kind === "external"}
        {#if sortedExternalNodes.length > 0}
          <Select
            size="xs"
            value={chain.hop1.node_id ?? ""}
            options={externalNodeOptions}
            onchange={(val) => setHopNode("hop1", val)}
          />
        {:else}
          <button
            type="button"
            onclick={onGoProtocols}
            class="text-xs text-blue-600 hover:underline dark:text-blue-400"
          >
            尚未导入外部节点，前往「节点与协议」添加 →
          </button>
        {/if}
      {/if}
    </div>

    {#if chain.two_hop && !isDirectBypass}
      <div class="space-y-2">
        <label class="flex flex-col gap-1.5 text-sm">
          <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
            第二跳（落地）
          </span>
          <Select
            value={chain.hop2.kind}
            options={[
              { value: "warp", label: "Cloudflare WARP" },
              { value: "geph", label: "迷雾通 (Geph5)" },
              { value: "psiphon", label: "赛风 (Psiphon)" },
              { value: "vpngate", label: "VPNGate (OpenVPN)" },
              { value: "external", label: "外部节点" },
            ]}
            onchange={(val) => setHop("hop2", val)}
          />
        </label>
        {#if chain.hop2.kind === "external"}
          {#if sortedExternalNodes.length > 0}
            <Select
              size="xs"
              value={chain.hop2.node_id ?? ""}
              options={externalNodeOptions}
              onchange={(val) => setHopNode("hop2", val)}
            />
          {:else}
            <button
              type="button"
              onclick={onGoProtocols}
              class="text-xs text-blue-600 hover:underline dark:text-blue-400"
            >
              尚未导入外部节点，前往「节点与协议」添加 →
            </button>
          {/if}
        {/if}
      </div>
    {/if}
  </div>

  {#if isDirectBypass}
    <p
      class="rounded-lg bg-zinc-50 px-3 py-2.5 text-xs text-zinc-600 dark:bg-zinc-800/70 dark:text-zinc-300"
    >
      无需任何远程服务器或账号。连接后通过静态 IP DoH 解析与 TLS/HTTP
      分片混淆直连目标站点，适用于仅受 DNS/SNI 干扰而 IP
      未被封锁的网站。可在「路由与 DNS」中切换作用范围及微调分片与 DoH 参数。
    </p>
  {/if}

  {#if chainInfo.warning}
    <p
      class="flex items-center gap-1.5 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
    >
      <ShieldAlert class="h-4 w-4 shrink-0" />
      <span>{chainInfo.warning}</span>
    </p>
  {/if}
</section>
