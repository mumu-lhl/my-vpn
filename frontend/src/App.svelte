<script lang="ts">
  import { onMount } from "svelte";
  import {
    Activity,
    ArrowRightLeft,
    Globe,
    Laptop,
    Moon,
    Power,
    Route,
    Settings,
    Shield,
    ShieldAlert,
    Sun,
    Wifi,
  } from "@lucide/svelte";
  import { analyzeChain, formatBytes, type ChainConfig, type HopKind } from "./lib/chain";
  import { applyTheme, type ThemeMode } from "./lib/theme";

  type Tab = "dashboard" | "chain" | "protocols" | "routing" | "settings";
  let activeTab = $state<Tab>("dashboard");
  let themeMode = $state<ThemeMode>("system");

  let connected = $state(false);
  let inboundMode = $state<"proxy" | "tun">("proxy");
  let allowLAN = $state(false);
  let socksPort = $state(10808);
  let httpPort = $state(10809);
  let upBytes = $state(0);
  let downBytes = $state(0);

  let chain = $state<ChainConfig>({
    two_hop: true,
    hop1: { kind: "geph" },
    hop2: { kind: "warp" },
  });

  let gephSecret = $state("");
  let gephCountry = $state("auto");
  let psiphonRegion = $state("auto");
  let warpMode = $state<"auto" | "wg" | "awg" | "masque">("auto");
  let warpLicense = $state("");
  let warpNoiseMode = $state("quic");
  let warpMasqueVhttp = $state<"auto" | "h3" | "h2">("auto");

  let bypassCN = $state(true);
  let blockAds = $state(true);
  let fakeIP = $state(true);
  let directDNS = $state("https://dns.alidns.com/dns-query");
  let proxyDNS = $state("https://1.1.1.1/dns-query");

  let chainInfo = $derived(analyzeChain(chain));

  $effect(() => {
    applyTheme(themeMode);
  });

  onMount(() => {
    applyTheme(themeMode);
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (themeMode === "system") {
        applyTheme("system");
      }
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  });

  function toggleConnect() {
    connected = !connected;
  }

  function setHop(which: "hop1" | "hop2", kind: string) {
    chain = { ...chain, [which]: { kind: kind as HopKind } };
  }

  function setTheme(mode: ThemeMode) {
    themeMode = mode;
  }
</script>

<div class="flex min-h-screen flex-col-reverse bg-slate-50 text-slate-900 transition-colors dark:bg-slate-950 dark:text-slate-100 md:flex-row">
  <aside class="sticky bottom-0 z-10 flex w-full shrink-0 flex-row justify-around border-t border-slate-200 bg-white/95 p-2 backdrop-blur dark:border-slate-800 dark:bg-slate-900/95 md:static md:w-56 md:flex-col md:justify-start md:gap-1.5 md:border-r md:border-t-0 md:p-4">
    <div class="mb-4 hidden items-center gap-2 px-2 text-lg font-bold text-sky-600 dark:text-sky-400 md:flex">
      <Shield class="h-5 w-5" />
      <span>MyVPN</span>
    </div>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab === 'dashboard'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "dashboard")}
    >
      <Activity class="h-4 w-4" />
      <span>仪表盘</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab === 'chain'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "chain")}
    >
      <ArrowRightLeft class="h-4 w-4" />
      <span>链路</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab === 'protocols'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "protocols")}
    >
      <Globe class="h-4 w-4" />
      <span>协议</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab === 'routing'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "routing")}
    >
      <Route class="h-4 w-4" />
      <span>路由</span>
    </button>

    <button
      class="flex flex-col items-center gap-1 rounded-xl px-3 py-2 text-xs font-medium transition md:flex-row md:gap-2.5 md:text-sm {activeTab === 'settings'
        ? 'bg-sky-600 text-white shadow-sm'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
      onclick={() => (activeTab = "settings")}
    >
      <Settings class="h-4 w-4" />
      <span>设置</span>
    </button>

    <div class="mt-auto hidden pt-4 md:block">
      <div class="grid grid-cols-3 gap-1 rounded-xl bg-slate-100 p-1 dark:bg-slate-800">
        <button
          title="白天模式"
          class="flex items-center justify-center rounded-lg py-1.5 transition {themeMode === 'light'
            ? 'bg-white text-amber-500 shadow-sm dark:bg-slate-700'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100'}"
          onclick={() => setTheme("light")}
        >
          <Sun class="h-4 w-4" />
        </button>
        <button
          title="黑夜模式"
          class="flex items-center justify-center rounded-lg py-1.5 transition {themeMode === 'dark'
            ? 'bg-white text-sky-500 shadow-sm dark:bg-slate-700'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100'}"
          onclick={() => setTheme("dark")}
        >
          <Moon class="h-4 w-4" />
        </button>
        <button
          title="跟随系统"
          class="flex items-center justify-center rounded-lg py-1.5 transition {themeMode === 'system'
            ? 'bg-white text-emerald-600 shadow-sm dark:bg-slate-700 dark:text-emerald-400'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100'}"
          onclick={() => setTheme("system")}
        >
          <Laptop class="h-4 w-4" />
        </button>
      </div>
    </div>
  </aside>

  <main class="flex-1 space-y-4 p-4 md:p-6">
    {#if activeTab === "dashboard"}
      <section class="flex flex-col justify-between gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:flex-row sm:items-center">
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <span class="inline-block h-2.5 w-2.5 rounded-full {connected ? 'bg-emerald-500' : 'bg-slate-400'}"></span>
            <h1 class="text-xl font-bold">{connected ? "已连接" : "未连接"}</h1>
          </div>
          <p class="text-sm text-slate-500 dark:text-slate-400">当前链路：{chainInfo.label}</p>
          {#if chainInfo.warning}
            <p class="flex items-center gap-1.5 text-sm text-amber-600 dark:text-amber-400">
              <ShieldAlert class="h-4 w-4 shrink-0" />
              <span>{chainInfo.warning}</span>
            </p>
          {/if}
        </div>
        <button
          class="inline-flex items-center justify-center gap-2 rounded-xl px-5 py-3 text-sm font-semibold text-white shadow-sm transition {connected
            ? 'bg-rose-600 hover:bg-rose-700'
            : 'bg-sky-600 hover:bg-sky-700'}"
          onclick={toggleConnect}
        >
          <Power class="h-4 w-4" />
          <span>{connected ? "断开连接" : "一键连接"}</span>
        </button>
      </section>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h3 class="flex items-center gap-2 font-semibold">
            <Wifi class="h-4 w-4 text-sky-500" />
            <span>出口模式</span>
          </h3>
          <label class="flex items-center gap-2 text-sm">
            <input type="radio" bind:group={inboundMode} value="proxy" />
            <span>本地代理 (SOCKS5:{socksPort} / HTTP:{httpPort})</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input type="radio" bind:group={inboundMode} value="tun" />
            <span>全局 VPN (TUN)</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" bind:checked={allowLAN} />
            <span>允许局域网共享代理 (0.0.0.0)</span>
          </label>
        </div>

        <div class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h3 class="flex items-center gap-2 font-semibold">
            <Activity class="h-4 w-4 text-emerald-500" />
            <span>实时流量</span>
          </h3>
          <p class="text-sm text-slate-600 dark:text-slate-300">上行：{formatBytes(upBytes)}</p>
          <p class="text-sm text-slate-600 dark:text-slate-300">下行：{formatBytes(downBytes)}</p>
        </div>
      </section>
    {:else if activeTab === "chain"}
      <section class="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h2 class="text-lg font-semibold">一级 / 二级链路编排</h2>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={chain.two_hop} />
          <span>启用二级链路（前置破壁 + 落地干净出口）</span>
        </label>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-1.5 text-sm">
            <span class="font-medium">第一跳（破壁）</span>
            <select
              class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
              value={chain.hop1.kind}
              onchange={(e) => setHop("hop1", e.currentTarget.value)}
            >
              <option value="geph">迷雾通 (Geph5)</option>
              <option value="psiphon">赛风 (Psiphon · 仅 TCP)</option>
              <option value="warp">Cloudflare WARP</option>
              <option value="external">外部订阅/自建节点</option>
            </select>
          </label>
          {#if chain.two_hop}
            <label class="flex flex-col gap-1.5 text-sm">
              <span class="font-medium">第二跳（落地）</span>
              <select
                class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
                value={chain.hop2.kind}
                onchange={(e) => setHop("hop2", e.currentTarget.value)}
              >
                <option value="warp">Cloudflare WARP</option>
                <option value="geph">迷雾通 (Geph5)</option>
                <option value="psiphon">赛风 (Psiphon)</option>
                <option value="external">外部订阅/自建节点</option>
              </select>
            </label>
          {/if}
        </div>
        {#if chainInfo.warning}
          <p class="flex items-center gap-1.5 text-sm text-amber-600 dark:text-amber-400">
            <ShieldAlert class="h-4 w-4 shrink-0" />
            <span>{chainInfo.warning}</span>
          </p>
        {/if}
      </section>
    {:else if activeTab === "protocols"}
      <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h2 class="text-lg font-semibold">迷雾通 (Geph5)</h2>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>账号 Secret</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="password"
            bind:value={gephSecret}
            placeholder="粘贴已有 Geph secret"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>出口国家</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={gephCountry}
            placeholder="auto / JP / US / SG"
          />
        </label>
      </section>

      <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h2 class="text-lg font-semibold">赛风 (Psiphon) &amp; Cloudflare WARP</h2>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>Psiphon 出口区域</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={psiphonRegion}
            placeholder="auto / US / JP / DE"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>WARP 隧道模式</span>
          <select
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            bind:value={warpMode}
          >
            <option value="auto">Auto</option>
            <option value="wg">WireGuard</option>
            <option value="awg">AmneziaWG</option>
            <option value="masque">MASQUE (HTTP/3 &amp; HTTP/2)</option>
          </select>
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>WARP+ License Key</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={warpLicense}
            placeholder="留空自动免费注册"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>WireGuard 噪声模式</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={warpNoiseMode}
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>MASQUE HTTP 版本</span>
          <select
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            bind:value={warpMasqueVhttp}
          >
            <option value="auto">Auto (H3 优先，3s 回退 H2)</option>
            <option value="h3">HTTP/3 (QUIC)</option>
            <option value="h2">HTTP/2 (TLS/TCP)</option>
          </select>
        </label>
      </section>
    {:else if activeTab === "routing"}
      <section class="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h2 class="text-lg font-semibold">路由预设与 DNS 防泄露</h2>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={bypassCN} />
          <span>中国大陆 IP 与域名直连 (geoip-cn / geosite-cn)</span>
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={blockAds} />
          <span>拦截广告与跟踪器 (geosite-category-ads-all)</span>
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" bind:checked={fakeIP} />
          <span>开启 FakeIP（TUN 模式下防 DNS 泄露）</span>
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>国内直连 DNS</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={directDNS}
          />
        </label>
        <label class="flex flex-col gap-1.5 text-sm">
          <span>海外代理 DNS</span>
          <input
            class="rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-950"
            type="text"
            bind:value={proxyDNS}
          />
        </label>
      </section>
    {:else}
      <section class="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h2 class="text-lg font-semibold">外观主题</h2>
        <div class="grid grid-cols-3 gap-2">
          <button
            class="flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition {themeMode === 'light'
              ? 'border-sky-600 bg-sky-50 text-sky-700 dark:bg-sky-950 dark:text-sky-300'
              : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800'}"
            onclick={() => setTheme("light")}
          >
            <Sun class="h-4 w-4" />
            <span>白天</span>
          </button>
          <button
            class="flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition {themeMode === 'dark'
              ? 'border-sky-600 bg-sky-50 text-sky-700 dark:bg-sky-950 dark:text-sky-300'
              : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800'}"
            onclick={() => setTheme("dark")}
          >
            <Moon class="h-4 w-4" />
            <span>黑夜</span>
          </button>
          <button
            class="flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition {themeMode === 'system'
              ? 'border-sky-600 bg-sky-50 text-sky-700 dark:bg-sky-950 dark:text-sky-300'
              : 'border-slate-200 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800'}"
            onclick={() => setTheme("system")}
          >
            <Laptop class="h-4 w-4" />
            <span>跟随系统</span>
          </button>
        </div>

        <h2 class="pt-2 text-lg font-semibold">平台与系统设置</h2>
        <p class="text-sm text-slate-600 dark:text-slate-300">
          Linux (niri / Wayland)：使用独立 core-helper 一次性 setcap 提权，关闭窗口最小化至 SNI 托盘。
        </p>
        <p class="text-sm text-slate-600 dark:text-slate-300">
          Android 10+ (arm64-v8a)：支持 VpnService 分应用白/黑名单、局域网共享与前台速率通知栏。
        </p>
      </section>
    {/if}
  </main>
</div>
