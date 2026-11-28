<script lang="ts">
  import { onMount } from "svelte";
  import { Copy, Download, RefreshCw, Trash2 } from "@lucide/svelte";
  import { AppService, type LogEntry } from "../lib/mygo";
  import Toggle from "./Toggle.svelte";

  interface Props {
    bridgeReady: boolean;
    enabled: boolean;
    onToggleEnabled: (next: boolean) => void;
  }

  let { bridgeReady, enabled, onToggleEnabled }: Props = $props();

  let entries = $state<LogEntry[]>([]);
  let loading = $state(false);
  let notice = $state("");
  let errorMsg = $state("");

  async function refreshLogs() {
    if (!bridgeReady || loading) return;
    loading = true;
    errorMsg = "";
    try {
      const snap = await AppService.GetLogs();
      entries = Array.isArray(snap.entries) ? snap.entries : [];
    } catch (err) {
      errorMsg = err instanceof Error ? err.message : String(err);
    } finally {
      loading = false;
    }
  }

  async function handleToggle(next: boolean) {
    notice = "";
    errorMsg = "";
    onToggleEnabled(next);
    if (!bridgeReady) return;
    try {
      await AppService.SetLogsEnabled(next);
      await refreshLogs();
    } catch (err) {
      errorMsg = err instanceof Error ? err.message : String(err);
    }
  }

  async function handleClear() {
    notice = "";
    errorMsg = "";
    if (!bridgeReady) {
      entries = [];
      return;
    }
    try {
      await AppService.ClearLogs();
      entries = [];
      notice = "日志已清空";
    } catch (err) {
      errorMsg = err instanceof Error ? err.message : String(err);
    }
  }

  function formatAllLogs(): string {
    return entries
      .map((e) => `[${e.time}] [${e.source}] ${e.message}`)
      .join("\n");
  }

  async function handleCopy() {
    notice = "";
    errorMsg = "";
    const text = formatAllLogs();
    if (!text) {
      notice = "当前暂无可复制的日志";
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      notice = `已复制 ${entries.length} 条日志`;
    } catch {
      errorMsg = "剪贴板写入失败，请使用导出功能";
    }
  }

  async function handleExport() {
    notice = "";
    errorMsg = "";
    let content = formatAllLogs();
    let savedPath = "";
    if (bridgeReady) {
      try {
        const res = await AppService.ExportLogs();
        savedPath = res.path || "";
        if (res.content) content = res.content;
      } catch (err) {
        errorMsg = err instanceof Error ? err.message : String(err);
        return;
      }
    }
    if (content && typeof document !== "undefined") {
      try {
        const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `looma-logs-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-")}.log`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
      } catch {
        // Fallback to backend file path notice below.
      }
    }
    notice = savedPath ? `已导出至：${savedPath}` : "日志已导出";
  }

  onMount(() => {
    void refreshLogs();
    const timer = window.setInterval(() => {
      if (enabled && !document.hidden) {
        void refreshLogs();
      }
    }, 2000);
    return () => window.clearInterval(timer);
  });
</script>

<section
  class="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex flex-wrap items-center justify-between gap-2">
    <div>
      <h2 class="text-base font-semibold">诊断日志</h2>
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        默认关闭以节省资源；排错时开启，保留最近 2000 条运行与连接日志
      </p>
    </div>
    <div class="flex items-center gap-1.5">
      <button
        type="button"
        disabled={loading}
        aria-label="刷新日志"
        title="刷新日志"
        onclick={refreshLogs}
        class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 text-zinc-600 hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
      >
        <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
      </button>
      <button
        type="button"
        onclick={handleCopy}
        class="inline-flex items-center gap-1 rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
      >
        <Copy class="h-3.5 w-3.5" />
        <span>复制</span>
      </button>
      <button
        type="button"
        onclick={handleExport}
        class="inline-flex items-center gap-1 rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
      >
        <Download class="h-3.5 w-3.5" />
        <span>导出</span>
      </button>
      <button
        type="button"
        onclick={handleClear}
        class="inline-flex items-center gap-1 rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-50 dark:border-zinc-700 dark:text-rose-400 dark:hover:bg-rose-950/40"
      >
        <Trash2 class="h-3.5 w-3.5" />
        <span>清空</span>
      </button>
    </div>
  </div>

  <Toggle
    checked={enabled}
    label="开启诊断日志采集"
    description="记录连接状态、订阅更新、测速及 Geph5 内核日志"
    onchange={handleToggle}
  />

  {#if notice}
    <p
      class="rounded-lg bg-emerald-50 px-3 py-2 text-xs text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300"
    >
      {notice}
    </p>
  {/if}
  {#if errorMsg}
    <p
      class="rounded-lg bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:bg-rose-950/40 dark:text-rose-300"
    >
      {errorMsg}
    </p>
  {/if}

  <div
    class="max-h-96 overflow-y-auto rounded-lg border border-zinc-200 bg-zinc-50 p-3 font-mono text-[11px] leading-relaxed dark:border-zinc-800 dark:bg-zinc-950"
  >
    {#if entries.length === 0}
      <p class="py-8 text-center text-zinc-400">
        {enabled ? "暂无日志记录" : "日志采集已关闭，开启后将记录后续运行事件"}
      </p>
    {:else}
      {#each entries as entry}
        <div class="break-all py-0.5 text-zinc-700 dark:text-zinc-300">
          <span class="text-zinc-400">[{entry.time}]</span>
          <span class="mx-1 font-semibold text-blue-600 dark:text-blue-400">
            [{entry.source}]
          </span>
          <span>{entry.message}</span>
        </div>
      {/each}
    {/if}
  </div>
</section>
