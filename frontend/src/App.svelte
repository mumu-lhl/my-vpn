<script lang="ts">
  import { analyzeChain, formatBytes, type ChainConfig, type HopKind } from "./lib/chain";

  type Tab = "dashboard" | "chain" | "protocols" | "routing" | "settings";
  let activeTab: Tab = "dashboard";

  let connected = false;
  let inboundMode: "proxy" | "tun" = "proxy";
  let allowLAN = false;
  let socksPort = 10808;
  let httpPort = 10809;
  let upBytes = 0;
  let downBytes = 0;

  let chain: ChainConfig = {
    two_hop: true,
    hop1: { kind: "geph" },
    hop2: { kind: "warp" },
  };

  let gephSecret = "";
  let gephCountry = "auto";
  let psiphonRegion = "auto";
  let warpMode: "auto" | "wg" | "awg" | "masque" = "auto";
  let warpLicense = "";
  let warpNoiseMode = "quic";
  let warpMasqueVhttp: "auto" | "h3" | "h2" = "auto";

  let bypassCN = true;
  let blockAds = true;
  let fakeIP = true;
  let directDNS = "https://dns.alidns.com/dns-query";
  let proxyDNS = "https://1.1.1.1/dns-query";

  $: chainInfo = analyzeChain(chain);

  function toggleConnect() {
    connected = !connected;
  }

  function setHop(which: "hop1" | "hop2", kind: string) {
    chain = { ...chain, [which]: { kind: kind as HopKind } };
  }
</script>

<div class="layout">
  <aside class="nav">
    <div class="brand">MyVPN</div>
    <button class:active={activeTab === "dashboard"} on:click={() => (activeTab = "dashboard")}>仪表盘</button>
    <button class:active={activeTab === "chain"} on:click={() => (activeTab = "chain")}>链路编排</button>
    <button class:active={activeTab === "protocols"} on:click={() => (activeTab = "protocols")}>协议与订阅</button>
    <button class:active={activeTab === "routing"} on:click={() => (activeTab = "routing")}>路由与 DNS</button>
    <button class:active={activeTab === "settings"} on:click={() => (activeTab = "settings")}>系统设置</button>
  </aside>

  <main class="content">
    {#if activeTab === "dashboard"}
      <section class="card hero">
        <div>
          <h1>{connected ? "已连接" : "未连接"}</h1>
          <p class="sub">当前链路：{chainInfo.label}</p>
          {#if chainInfo.warning}
            <p class="warn">{chainInfo.warning}</p>
          {/if}
        </div>
        <button class="primary" class:on={connected} on:click={toggleConnect}>
          {connected ? "断开连接" : "一键连接"}
        </button>
      </section>

      <section class="grid">
        <div class="card">
          <h3>出口模式</h3>
          <label><input type="radio" bind:group={inboundMode} value="proxy" /> 本地代理 (SOCKS5:{socksPort} / HTTP:{httpPort})</label>
          <label><input type="radio" bind:group={inboundMode} value="tun" /> 全局 VPN (TUN)</label>
          <label><input type="checkbox" bind:checked={allowLAN} /> 允许局域网共享代理 (0.0.0.0)</label>
        </div>
        <div class="card">
          <h3>实时流量</h3>
          <p>上行：{formatBytes(upBytes)}</p>
          <p>下行：{formatBytes(downBytes)}</p>
        </div>
      </section>
    {:else if activeTab === "chain"}
      <section class="card">
        <h2>一级 / 二级链路编排</h2>
        <label><input type="checkbox" bind:checked={chain.two_hop} /> 启用二级链路（前置破壁 + 落地干净出口）</label>
        <div class="row">
          <div>
            <h4>第一跳（破壁）</h4>
            <select value={chain.hop1.kind} on:change={(e) => setHop("hop1", e.currentTarget.value)}>
              <option value="geph">迷雾通 (Geph5)</option>
              <option value="psiphon">赛风 (Psiphon · 仅 TCP)</option>
              <option value="warp">Cloudflare WARP</option>
              <option value="external">外部订阅/自建节点</option>
            </select>
          </div>
          {#if chain.two_hop}
            <div>
              <h4>第二跳（落地）</h4>
              <select value={chain.hop2.kind} on:change={(e) => setHop("hop2", e.currentTarget.value)}>
                <option value="warp">Cloudflare WARP</option>
                <option value="geph">迷雾通 (Geph5)</option>
                <option value="psiphon">赛风 (Psiphon)</option>
                <option value="external">外部订阅/自建节点</option>
              </select>
            </div>
          {/if}
        </div>
        {#if chainInfo.warning}
          <p class="warn">{chainInfo.warning}</p>
        {/if}
      </section>
    {:else if activeTab === "protocols"}
      <section class="card">
        <h2>迷雾通 (Geph5)</h2>
        <label>账号 Secret <input type="password" bind:value={gephSecret} placeholder="粘贴已有 Geph secret" /></label>
        <label>出口国家 <input type="text" bind:value={gephCountry} placeholder="auto / JP / US / SG" /></label>
      </section>
      <section class="card">
        <h2>赛风 (Psiphon) & Cloudflare WARP</h2>
        <label>Psiphon 出口区域 <input type="text" bind:value={psiphonRegion} placeholder="auto / US / JP / DE" /></label>
        <label>WARP 隧道模式
          <select bind:value={warpMode}>
            <option value="auto">Auto</option>
            <option value="wg">WireGuard</option>
            <option value="awg">AmneziaWG</option>
            <option value="masque">MASQUE (HTTP/3 & HTTP/2)</option>
          </select>
        </label>
        <label>WARP+ License Key <input type="text" bind:value={warpLicense} placeholder="留空自动免费注册" /></label>
        <label>WireGuard 噪声模式 <input type="text" bind:value={warpNoiseMode} /></label>
        <label>MASQUE HTTP 版本
          <select bind:value={warpMasqueVhttp}>
            <option value="auto">Auto (H3 优先，3s 回退 H2)</option>
            <option value="h3">HTTP/3 (QUIC)</option>
            <option value="h2">HTTP/2 (TLS/TCP)</option>
          </select>
        </label>
      </section>
    {:else if activeTab === "routing"}
      <section class="card">
        <h2>路由预设与 DNS 防泄露</h2>
        <label><input type="checkbox" bind:checked={bypassCN} /> 中国大陆 IP 与域名直连 (geoip-cn / geosite-cn)</label>
        <label><input type="checkbox" bind:checked={blockAds} /> 拦截广告与跟踪器 (geosite-category-ads-all)</label>
        <label><input type="checkbox" bind:checked={fakeIP} /> 开启 FakeIP（TUN 模式下防 DNS 泄露）</label>
        <label>国内直连 DNS <input type="text" bind:value={directDNS} /></label>
        <label>海外代理 DNS <input type="text" bind:value={proxyDNS} /></label>
      </section>
    {:else}
      <section class="card">
        <h2>平台与系统设置</h2>
        <p>Linux (niri / Wayland)：使用独立 core-helper 一次性 setcap 提权，关闭窗口最小化至 SNI 托盘。</p>
        <p>Android 10+ (arm64-v8a)：支持 VpnService 分应用白/黑名单、局域网共享与前台速率通知栏。</p>
      </section>
    {/if}
  </main>
</div>

<style>
  :global(body) { margin: 0; font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #e2e8f0; }
  .layout { display: flex; min-height: 100vh; }
  .nav { width: 200px; background: #1e293b; padding: 16px; display: flex; flex-direction: column; gap: 8px; }
  .brand { font-size: 1.25rem; font-weight: 700; margin-bottom: 12px; color: #38bdf8; }
  .nav button { background: transparent; color: #cbd5e1; border: none; text-align: left; padding: 10px 12px; border-radius: 8px; cursor: pointer; }
  .nav button.active { background: #0284c7; color: #fff; }
  .content { flex: 1; padding: 20px; display: flex; flex-direction: column; gap: 16px; }
  .card { background: #1e293b; border-radius: 12px; padding: 16px; display: flex; flex-direction: column; gap: 10px; }
  .hero { flex-direction: row; justify-content: space-between; align-items: center; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 16px; }
  .row { display: flex; gap: 16px; flex-wrap: wrap; }
  .primary { background: #0284c7; color: #fff; border: none; padding: 12px 24px; border-radius: 10px; font-weight: 600; cursor: pointer; }
  .primary.on { background: #dc2626; }
  .warn { color: #fbbf24; font-size: 0.9rem; margin: 0; }
  .sub { color: #94a3b8; margin: 4px 0; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 0.92rem; }
  input[type="text"], input[type="password"], select { background: #0f172a; color: #e2e8f0; border: 1px solid #334155; border-radius: 6px; padding: 8px; }
  @media (max-width: 720px) {
    .layout { flex-direction: column-reverse; }
    .nav { width: 100%; flex-direction: row; justify-content: space-around; padding: 8px; box-sizing: border-box; position: sticky; bottom: 0; }
    .brand { display: none; }
    .hero { flex-direction: column; align-items: flex-start; }
  }
</style>
