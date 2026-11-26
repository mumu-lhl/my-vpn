<script lang="ts">
  import { Save, ShieldAlert } from "@lucide/svelte";
  import type { ChainAnalysis, ChainConfig, HopKind } from "../lib/chain";
  import type { ExternalNode } from "../lib/mygo";
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

  function setHop(which: "hop1" | "hop2", kind: string) {
    const nextKind = kind as HopKind;
    const defaultNodeId =
      nextKind === "external" ? sortedExternalNodes[0]?.id : undefined;
    chain = {
      ...chain,
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
        支持单跳直出或两跳链式转发，例如前置破壁叠加落地出口
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

  <Toggle
    bind:checked={chain.two_hop}
    label="启用二级链式代理"
    description="流量先经第一跳，再由第二跳落地访问目标网站"
  />

  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
    <div class="space-y-2">
      <label class="flex flex-col gap-1.5 text-sm">
        <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
          第一跳（前置）
        </span>
        <select
          class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
          value={chain.hop1.kind}
          onchange={(e) => setHop("hop1", e.currentTarget.value)}
        >
          <option value="geph">迷雾通 (Geph5)</option>
          <option value="psiphon">赛风 (Psiphon · 仅 TCP)</option>
          <option value="warp">Cloudflare WARP</option>
          <option value="external">外部节点</option>
        </select>
      </label>
      {#if chain.hop1.kind === "external"}
        {#if sortedExternalNodes.length > 0}
          <select
            class="w-full rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-xs dark:border-zinc-700 dark:bg-zinc-950"
            value={chain.hop1.node_id ?? ""}
            onchange={(e) => setHopNode("hop1", e.currentTarget.value)}
          >
            {#each sortedExternalNodes as node}
              <option value={node.id}>
                {node.name} ({node.protocol})
              </option>
            {/each}
          </select>
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

    {#if chain.two_hop}
      <div class="space-y-2">
        <label class="flex flex-col gap-1.5 text-sm">
          <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
            第二跳（落地）
          </span>
          <select
            class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
            value={chain.hop2.kind}
            onchange={(e) => setHop("hop2", e.currentTarget.value)}
          >
            <option value="warp">Cloudflare WARP</option>
            <option value="geph">迷雾通 (Geph5)</option>
            <option value="psiphon">赛风 (Psiphon)</option>
            <option value="external">外部节点</option>
          </select>
        </label>
        {#if chain.hop2.kind === "external"}
          {#if sortedExternalNodes.length > 0}
            <select
              class="w-full rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-xs dark:border-zinc-700 dark:bg-zinc-950"
              value={chain.hop2.node_id ?? ""}
              onchange={(e) => setHopNode("hop2", e.currentTarget.value)}
            >
              {#each sortedExternalNodes as node}
                <option value={node.id}>
                  {node.name} ({node.protocol})
                </option>
              {/each}
            </select>
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

  {#if chainInfo.warning}
    <p
      class="flex items-center gap-1.5 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
    >
      <ShieldAlert class="h-4 w-4 shrink-0" />
      <span>{chainInfo.warning}</span>
    </p>
  {/if}
</section>
