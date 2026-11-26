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
      label="轻量直连分片"
      description="对部分仅受 SNI 干扰的站点尝试直连 TLS 分片，减少代理开销"
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
