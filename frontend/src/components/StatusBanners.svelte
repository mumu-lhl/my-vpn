<script lang="ts">
  import { X } from "@lucide/svelte";

  interface Props {
    error: string;
    notice: string;
    onClearError: () => void;
    onClearNotice: () => void;
  }

  let { error, notice, onClearError, onClearNotice }: Props = $props();
</script>

{#if error}
  <div
    class="flex items-start justify-between gap-3 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700 dark:border-red-900/70 dark:bg-red-950/50 dark:text-red-300"
  >
    <span>{error}</span>
    <button
      type="button"
      aria-label="关闭提示"
      onclick={onClearError}
      class="shrink-0 opacity-70 hover:opacity-100"
    >
      <X class="h-4 w-4" />
    </button>
  </div>
{/if}
{#if notice}
  <div
    class="flex items-start justify-between gap-3 rounded-lg border border-zinc-200 bg-white px-3.5 py-2.5 text-sm text-zinc-700 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-200"
  >
    <span>{notice}</span>
    <button
      type="button"
      aria-label="关闭提示"
      onclick={onClearNotice}
      class="shrink-0 opacity-70 hover:opacity-100"
    >
      <X class="h-4 w-4" />
    </button>
  </div>
{/if}
