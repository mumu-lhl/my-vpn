<script lang="ts">
  interface Props {
    checked: boolean;
    label: string;
    description?: string;
    disabled?: boolean;
    onchange?: (checked: boolean) => void;
  }

  let {
    checked = $bindable(false),
    label,
    description = "",
    disabled = false,
    onchange,
  }: Props = $props();

  function toggle() {
    if (disabled) return;
    checked = !checked;
    onchange?.(checked);
  }
</script>

<div
  class="flex items-center justify-between gap-4 py-1.5 {disabled
    ? 'opacity-50'
    : ''}"
>
  <div class="min-w-0 flex-1">
    <p class="text-sm font-medium text-zinc-900 dark:text-zinc-100">{label}</p>
    {#if description}
      <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
        {description}
      </p>
    {/if}
  </div>
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    {disabled}
    onclick={toggle}
    class="relative inline-flex h-6 w-10 shrink-0 items-center rounded-full transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 {checked
      ? 'bg-blue-600 dark:bg-blue-500'
      : 'bg-zinc-200 dark:bg-zinc-700'}"
  >
    <span
      class="inline-block h-4 w-4 rounded-full bg-white shadow-xs transition-transform {checked
        ? 'translate-x-5'
        : 'translate-x-1'}"
    ></span>
  </button>
</div>
