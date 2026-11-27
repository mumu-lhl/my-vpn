<script lang="ts">
  import {
    Activity,
    Check,
    Lock,
    RefreshCw,
    Save,
    ShieldAlert,
    Trash2,
  } from "@lucide/svelte";
  import { onMount } from "svelte";
  import type { AppFormState } from "../lib/config_state";
  import {
    AppService,
    hasMyGoBridge,
    type AutoProbeReport,
    type BypassCapabilityStatus,
    type BypassStrategy,
    type DomainCacheEntry,
  } from "../lib/mygo";
  import Toggle from "./Toggle.svelte";

  interface Props {
    form: AppFormState;
    onSave: () => void;
  }

  let { form, onSave }: Props = $props();

  const FALLBACK_PRESETS: BypassStrategy[] = [
    {
      id: "standard_split",
      name: "标准 SNI 精准分片",
      description: "在 SNI 域名中点多段切分，叠加 TLS Record 拆分与 HTTP 混淆",
      methods: ["multisplit", "tlsrec"],
      split_at_sni: true,
      mix_sni_case: true,
      split_count: 3,
      fragment_size: "10-30",
      fragment_sleep: "2-8ms",
      ttl: 0,
      http_mix_host_case: true,
      quic_mode: "fake",
      requires_raw: false,
    },
    {
      id: "disorder_oob",
      name: "乱序 + OOB 带外注入（免 Root 强力）",
      description: "SNI 中点注入 MSG_OOB 字节并用低 TTL 制造首片重传乱序",
      methods: ["disorder", "oob", "multisplit"],
      split_at_sni: true,
      mix_sni_case: true,
      split_count: 3,
      fragment_size: "8-24",
      fragment_sleep: "2-6ms",
      ttl: 4,
      http_mix_host_case: true,
      quic_mode: "fake",
      requires_raw: false,
    },
    {
      id: "low_ttl_fake",
      name: "低 TTL 诱饵 + TLS Record 拆分",
      description:
        "先发低 TTL 伪造 ClientHello 诱饵包，再拆分为多段 TLS Record",
      methods: ["fake_ttl", "tlsrec", "multisplit"],
      split_at_sni: true,
      mix_sni_case: true,
      split_count: 4,
      fragment_size: "10-25",
      fragment_sleep: "2-8ms",
      ttl: 5,
      http_mix_host_case: true,
      quic_mode: "fake",
      requires_raw: false,
    },
    {
      id: "raw_fake_multidisorder",
      name: "强力内核 Fake + 多段乱序（Linux Raw）",
      description:
        "借助 CAP_NET_RAW 注入坏校验和与错序诱饵包；无权限自动无感降级",
      methods: ["raw_badsum", "raw_badseq", "disorder", "multisplit", "tlsrec"],
      split_at_sni: true,
      mix_sni_case: true,
      split_count: 4,
      fragment_size: "8-20",
      fragment_sleep: "1-5ms",
      ttl: 4,
      http_mix_host_case: true,
      quic_mode: "fake",
      requires_raw: true,
    },
    {
      id: "compat_minimal",
      name: "极简兼容（回落 TCP）",
      description: "温和 SNI 切分并拦截 UDP 443 迫使浏览器回落 TCP/TLS",
      methods: ["multisplit"],
      split_at_sni: true,
      mix_sni_case: false,
      split_count: 2,
      fragment_size: "16-40",
      fragment_sleep: "1-4ms",
      ttl: 0,
      http_mix_host_case: true,
      quic_mode: "block",
      requires_raw: false,
    },
  ];

  let presets = $state<BypassStrategy[]>(FALLBACK_PRESETS);
  let capability = $state<BypassCapabilityStatus | null>(null);
  let domainCache = $state<DomainCacheEntry[]>([]);
  let probeDomain = $state("www.pixiv.net");
  let probing = $state(false);
  let probeReport = $state<AutoProbeReport | null>(null);
  let probeError = $state<string | null>(null);

  let activePreset = $derived(
    presets.find((p) => p.id === form.directBypassPresetID) ?? presets[0],
  );

  async function loadBypassState() {
    if (!hasMyGoBridge()) return;
    try {
      const [p, cap, cache] = await Promise.all([
        AppService.GetBypassPresets(),
        AppService.GetBypassCapabilityStatus(),
        AppService.GetDomainBypassCache(),
      ]);
      if (Array.isArray(p) && p.length > 0) presets = p;
      capability = cap;
      domainCache = Array.isArray(cache) ? cache : [];
    } catch {
      // Keep fallback presets in standalone preview.
    }
  }

  onMount(() => {
    void loadBypassState();
  });

  function applyPreset(id: string) {
    form.directBypassPresetID = id;
    const p = presets.find((item) => item.id === id);
    if (!p) return;
    form.directBypassSplitAtSNI = p.split_at_sni;
    form.directBypassMixSNICase = p.mix_sni_case;
    form.directBypassSplitCount = p.split_count || 3;
    form.directBypassFragmentSize = p.fragment_size;
    form.directBypassFragmentSleep = p.fragment_sleep;
    form.directBypassRecordFragment = p.methods.includes("tlsrec");
    form.directBypassEnableDisorder = p.methods.includes("disorder");
    form.directBypassEnableOOB = p.methods.includes("oob");
    form.directBypassEnableFakeTTL = p.methods.includes("fake_ttl");
    if (p.ttl > 0) form.directBypassFakeTTL = p.ttl;
    form.directBypassEnableRawFake = p.requires_raw;
    form.directBypassHttpMixHostCase = p.http_mix_host_case;
    form.directBypassQUICMode =
      p.quic_mode === "block" || p.quic_mode === "pass" ? p.quic_mode : "fake";
  }

  async function runAutoProbe() {
    if (probing) return;
    probing = true;
    probeError = null;
    try {
      const rep = await AppService.AutoProbeBypass(
        probeDomain.trim() || "www.pixiv.net",
      );
      probeReport = rep;
      if (rep.error) {
        probeError = rep.error;
      }
      await loadBypassState();
    } catch (e) {
      probeError = e instanceof Error ? e.message : String(e);
    } finally {
      probing = false;
    }
  }

  async function removeCacheEntry(domain: string) {
    if (!hasMyGoBridge()) return;
    try {
      await AppService.ClearDomainBypassCache(domain);
      domainCache = await AppService.GetDomainBypassCache();
    } catch {
      // Ignore bridge errors in preview.
    }
  }

  async function lockCacheEntry(domain: string, strategyId: string) {
    if (!hasMyGoBridge()) return;
    try {
      await AppService.LockDomainBypassStrategy(domain, strategyId);
      domainCache = await AppService.GetDomainBypassCache();
    } catch {
      // Ignore bridge errors in preview.
    }
  }
</script>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex items-center justify-between">
    <h2 class="text-base font-semibold">路由与 DNS</h2>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
      onclick={onSave}
    >
      <Save class="h-3.5 w-3.5" />
      <span>保存</span>
    </button>
  </div>

  <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
    <Toggle
      bind:checked={form.bypassCN}
      label="中国大陆流量直连"
      description="国内 IP 与常见国内域名绕过代理直接访问"
    />
    <Toggle
      bind:checked={form.bypassPrivate}
      label="局域网与私有地址直连"
      description="内网设备与保留地址段不经代理转发"
    />
    <Toggle
      bind:checked={form.blockAds}
      label="拦截常见广告与跟踪域名"
      description="自动拒绝广告与隐私追踪请求"
    />
    <Toggle
      bind:checked={form.directBypass}
      label="启用预设防干扰域名清单"
      description="对 Pixiv、Wikipedia、GitHub 等常见受干扰站点自动应用直连分片"
    />
    <Toggle
      bind:checked={form.fakeIP}
      label="启用 FakeIP"
      description="在 TUN 模式下加快 DNS 响应并防止 DNS 泄露"
    />
  </div>

  <div class="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        直连 DNS
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        bind:value={form.directDNS}
      />
    </label>
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        代理 DNS
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        bind:value={form.proxyDNS}
      />
    </label>
  </div>
</section>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-wrap items-center justify-between gap-2">
    <div>
      <h2 class="text-base font-semibold">强力反 DPI（无服务器直连绕过）</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        免 Root 用户态极限（SNI 精准切分、乱序、OOB、QUIC、ECH）与 Linux
        内核级诱饵包
      </p>
    </div>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700"
      onclick={onSave}
    >
      <Save class="h-3.5 w-3.5" />
      <span>保存策略</span>
    </button>
  </div>

  {#if capability?.degraded && capability.notice}
    <p
      class="flex items-start gap-2 rounded-lg bg-amber-50 px-3 py-2.5 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
    >
      <ShieldAlert class="mt-0.5 h-4 w-4 shrink-0" />
      <span>{capability.notice}</span>
    </p>
  {/if}

  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm sm:col-span-2">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        反 DPI 策略预设
      </span>
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        value={form.directBypassPresetID}
        onchange={(e) => applyPreset(e.currentTarget.value)}
      >
        {#each presets as p}
          <option value={p.id}>{p.name}</option>
        {/each}
      </select>
      {#if activePreset}
        <span class="text-xs text-zinc-500 dark:text-zinc-400">
          {activePreset.description}
        </span>
      {/if}
    </label>

    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        无服务器模式作用范围
      </span>
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={form.directBypassScope}
      >
        <option value="global">除国内与私有地址外全部绕过</option>
        <option value="list">仅预设与自定义规则域名绕过</option>
      </select>
    </label>

    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        UDP 443 / QUIC 处理方式
      </span>
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={form.directBypassQUICMode}
      >
        <option value="fake">发送 QUIC Initial 诱饵包混淆（推荐）</option>
        <option value="block">拦截 UDP 443 强制回落 TCP/TLS</option>
        <option value="pass">原样放行 UDP 443</option>
      </select>
    </label>
  </div>

  <!-- One-click Auto-Probe (blockcheck-style) -->
  <div
    class="space-y-3 rounded-lg border border-zinc-200 bg-zinc-50/70 p-3.5 dark:border-zinc-800 dark:bg-zinc-950/60"
  >
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h3 class="text-xs font-semibold text-zinc-800 dark:text-zinc-200">
          一键自动探测当前网络最优策略
        </h3>
        <p class="text-xs text-zinc-500 dark:text-zinc-400">
          自动遍历 5 套预设并检测 DoH HTTPS RR / ECH
          支持情况，选出延迟最低的可用方案
        </p>
      </div>
    </div>
    <div class="flex flex-wrap gap-2">
      <input
        class="min-w-0 flex-1 rounded-lg border border-zinc-200 bg-white px-3 py-1.5 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-900"
        type="text"
        placeholder="输入测试域名，如 www.pixiv.net"
        bind:value={probeDomain}
      />
      <button
        type="button"
        disabled={probing}
        onclick={runAutoProbe}
        class="inline-flex items-center gap-1.5 rounded-lg bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-zinc-800 disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-200"
      >
        <Activity class="h-3.5 w-3.5" />
        <span>{probing ? "探测中..." : "一键自动探测"}</span>
      </button>
    </div>

    {#if probeError}
      <p class="text-xs text-red-600 dark:text-red-400">{probeError}</p>
    {/if}

    {#if probeReport}
      <div class="space-y-2 text-xs">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-zinc-600 dark:text-zinc-300">
            目标 <strong class="font-mono">{probeReport.domain}</strong>
            {#if probeReport.ech_available}
              · <span class="text-emerald-600 dark:text-emerald-400">
                ECH {probeReport.ech_handshake_ok ? "握手通过" : "已发布"}
              </span>
            {/if}
          </span>
          {#if probeReport.recommended_id}
            <button
              type="button"
              onclick={() => {
                if (probeReport?.recommended_id) {
                  applyPreset(probeReport.recommended_id);
                  onSave();
                }
              }}
              class="inline-flex items-center gap-1 rounded-md bg-emerald-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-emerald-700"
            >
              <Check class="h-3 w-3" />
              <span>应用推荐：{probeReport.recommended_name}</span>
            </button>
          {/if}
        </div>
        <div
          class="divide-y divide-zinc-200 rounded-md border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-900"
        >
          {#each probeReport.results as item}
            <div class="flex items-center justify-between px-3 py-1.5">
              <span class="truncate text-zinc-700 dark:text-zinc-200">
                {item.strategy_name}
              </span>
              {#if item.success}
                <span class="font-mono text-emerald-600 dark:text-emerald-400">
                  {item.latency_ms} ms
                </span>
              {:else}
                <span class="text-zinc-400">不可达</span>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <!-- Advanced desync toggles -->
  <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
    <Toggle
      bind:checked={form.directBypassSplitAtSNI}
      label="精准 SNI 域名中点切分"
      description="解析 TLS ClientHello server_name 扩展偏移，保证在域名字符中间切开"
    />
    <Toggle
      bind:checked={form.directBypassMixSNICase}
      label="TLS SNI 大小写混合"
      description="对 ClientHello 中的 SNI 域名进行大小写交替改写"
    />
    <Toggle
      bind:checked={form.directBypassRecordFragment}
      label="TLS Record 级分片"
      description="将 ClientHello 拆分为多个合法 TLS 握手记录"
    />
    <Toggle
      bind:checked={form.directBypassEnableDisorder}
      label="TCP Disorder 乱序发送"
      description="临时压低首片 IP_TTL 触发重传乱序到达，绕过顺序流检测（免 Root）"
    />
    <Toggle
      bind:checked={form.directBypassEnableOOB}
      label="TCP MSG_OOB 带外字节注入"
      description="在 SNI 切分点插入带外字节打乱 DPI 偏移计算（免 Root）"
    />
    <Toggle
      bind:checked={form.directBypassEnableRawFake}
      label="Linux 内核级 Raw Socket 诱饵包（badsum / badseq）"
      description="需要 looma-helper 具备 CAP_NET_RAW；无权限时自动无感降级到免 Root 乱序+OOB"
    />
    <Toggle
      bind:checked={form.directBypassHttpMixHostCase}
      label="明文 HTTP Host 大小写混淆"
      description="对 80 端口等明文 HTTP 请求改写 Host 头并分片"
    />
    <Toggle
      bind:checked={form.directBypassEnableECH}
      label="启用 DoH HTTPS 记录（Type 65）与 ECH 支持"
      description="查询并透传 HTTPS RR 中的 ECH 参数，免装证书支持加密 SNI"
    />
    <Toggle
      bind:checked={form.directBypassAutoLearn}
      label="按域名自动记忆最优策略"
      description="优先使用该域名实测通过的策略，连续失败自动回退"
    />
    <Toggle
      bind:checked={form.directBypassDoHDisableSNI}
      label="DoH 连接禁用 SNI"
      description="访问静态 IP DoH 时省略明文 SNI，防止 DoH 自身被干扰"
    />
  </div>

  <div class="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        分片字节范围
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        placeholder="10-30"
        bind:value={form.directBypassFragmentSize}
      />
    </label>

    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        分片间隔抖动
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        placeholder="2-8ms"
        bind:value={form.directBypassFragmentSleep}
      />
    </label>

    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        自定义 DoH 地址（可选）
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        placeholder="留空使用内置静态 IP DoH"
        bind:value={form.directBypassDoHURL}
      />
    </label>

    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        DoH 静态 IP（可选，逗号分隔）
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        placeholder="例如 1.1.1.1, 1.0.0.1"
        bind:value={form.directBypassDoHStaticIPs}
      />
    </label>
  </div>

  <!-- Per-Domain Strategy Cache -->
  <div class="space-y-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
    <div class="flex items-center justify-between">
      <h3 class="text-xs font-semibold text-zinc-700 dark:text-zinc-300">
        已记忆域名策略（{domainCache.length}）
      </h3>
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={loadBypassState}
          class="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200"
        >
          <RefreshCw class="h-3 w-3" />
          <span>刷新</span>
        </button>
        {#if domainCache.length > 0}
          <button
            type="button"
            onclick={() => removeCacheEntry("")}
            class="text-xs text-red-600 hover:underline dark:text-red-400"
          >
            清空全部
          </button>
        {/if}
      </div>
    </div>

    {#if domainCache.length === 0}
      <p class="text-xs text-zinc-500 dark:text-zinc-400">
        尚未缓存单域名策略，点击上方「一键自动探测」后会自动记住对应站点的最佳绕过方案。
      </p>
    {:else}
      <div
        class="divide-y divide-zinc-100 rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800"
      >
        {#each domainCache as entry}
          <div
            class="flex items-center justify-between gap-2 px-3 py-2 text-xs"
          >
            <div class="min-w-0">
              <span
                class="font-mono font-medium text-zinc-800 dark:text-zinc-100"
              >
                {entry.domain}
              </span>
              <span class="ml-2 text-zinc-500">
                {entry.strategy_id} · {entry.latency_ms} ms
              </span>
              {#if entry.ech_supported}
                <span
                  class="ml-1.5 rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"
                >
                  ECH
                </span>
              {/if}
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                title={entry.locked ? "已锁定" : "锁定当前策略"}
                onclick={() => lockCacheEntry(entry.domain, entry.strategy_id)}
                class="text-zinc-400 hover:text-blue-600"
              >
                <Lock class="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                title="移除记录"
                onclick={() => removeCacheEntry(entry.domain)}
                class="text-zinc-400 hover:text-red-600"
              >
                <Trash2 class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</section>
