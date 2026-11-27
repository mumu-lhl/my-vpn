<script lang="ts">
  import { Save } from "@lucide/svelte";
  import type { AppFormState } from "../lib/config_state";
  import Toggle from "./Toggle.svelte";

  interface Props {
    form: AppFormState;
    onSave: () => void;
  }

  let { form, onSave }: Props = $props();
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
  <div>
    <h2 class="text-base font-semibold">直连绕过（防干扰）参数</h2>
    <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
      控制无服务器直连绕过与轻量防干扰分片的生效范围、分片抖动及防污染 DoH
    </p>
  </div>

  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm sm:col-span-2">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        无服务器模式作用范围
      </span>
      <select
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"
        bind:value={form.directBypassScope}
      >
        <option value="global">除国内与私有地址外，其余全部走防干扰直连</option>
        <option value="list"
          >仅预设域名与自定义规则走防干扰直连，其余普通直连</option
        >
      </select>
    </label>

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
  </div>

  <div class="divide-y divide-zinc-100 dark:divide-zinc-800">
    <Toggle
      bind:checked={form.directBypassRecordFragment}
      label="TLS Record 级分片"
      description="将 ClientHello 拆分为多个独立 TLS 握手记录，增强对深度包检测的绕过能力"
    />
    <Toggle
      bind:checked={form.directBypassHttpMixHostCase}
      label="明文 HTTP Host 大小写混淆"
      description="对 80 端口等明文 HTTP 请求改写 Host 头大小写并分片发送"
    />
    <Toggle
      bind:checked={form.directBypassDoHDisableSNI}
      label="DoH 连接禁用 SNI"
      description="访问静态 IP DoH 服务器时不发送明文 SNI，防止 DoH 自身被阻断"
    />
  </div>

  <div class="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-2">
    <label class="flex flex-col gap-1.5 text-sm">
      <span class="text-xs font-medium text-zinc-600 dark:text-zinc-300">
        自定义 DoH 地址（可选）
      </span>
      <input
        class="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"
        type="text"
        placeholder="留空使用内置静态 IP DoH (1.1.1.1 / 8.8.8.8)"
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
</section>
