<script lang="ts">
  import { Check, ChevronDown } from "@lucide/svelte";

  export interface SelectOption {
    value: string;
    label: string;
  }

  interface Props {
    value?: string;
    options: SelectOption[];
    placeholder?: string;
    disabled?: boolean;
    size?: "sm" | "xs";
    class?: string;
    ariaLabel?: string;
    onchange?: (value: string) => void;
  }

  let {
    value = $bindable(""),
    options,
    placeholder = "请选择",
    disabled = false,
    size = "sm",
    class: extraClass = "",
    ariaLabel,
    onchange,
  }: Props = $props();

  let open = $state(false);
  let openUpward = $state(false);
  let containerEl: HTMLDivElement | undefined = $state();

  let selectedOption = $derived(options.find((opt) => opt.value === value));

  function toggleOpen(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (disabled) return;
    if (!open && containerEl && typeof window !== "undefined") {
      const rect = containerEl.getBoundingClientRect();
      const spaceBelow = window.innerHeight - rect.bottom;
      openUpward = spaceBelow < 240 && rect.top > 240;
    }
    open = !open;
  }

  function selectOption(nextValue: string) {
    open = false;
    if (value !== nextValue) {
      value = nextValue;
    }
    onchange?.(nextValue);
  }

  $effect(() => {
    if (!open || typeof window === "undefined") return;
    const onPointerDown = (e: PointerEvent) => {
      if (containerEl && !containerEl.contains(e.target as Node)) {
        open = false;
      }
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        open = false;
      }
    };
    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  });
</script>

<div bind:this={containerEl} class="relative {extraClass}">
  <button
    type="button"
    {disabled}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-label={ariaLabel}
    onclick={toggleOpen}
    class="flex w-full items-center justify-between gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-left text-zinc-900 transition hover:border-zinc-300 focus:border-blue-500 focus:outline-none disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100 dark:hover:border-zinc-600 {size ===
    'xs'
      ? 'text-xs'
      : 'text-sm'}"
  >
    <span class="min-w-0 truncate">
      {selectedOption ? selectedOption.label : placeholder}
    </span>
    <ChevronDown
      class="h-4 w-4 shrink-0 text-zinc-400 transition-transform {open
        ? 'rotate-180'
        : ''}"
    />
  </button>

  {#if open}
    <div
      role="listbox"
      class="absolute left-0 z-50 w-full min-w-[10rem] overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-lg dark:border-zinc-700 dark:bg-zinc-900 {openUpward
        ? 'bottom-full mb-1'
        : 'top-full mt-1'}"
    >
      <div class="max-h-60 overflow-y-auto py-1">
        {#each options as opt (opt.value)}
          {@const selected = opt.value === value}
          <button
            type="button"
            role="option"
            aria-selected={selected}
            onclick={() => selectOption(opt.value)}
            class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left transition {size ===
            'xs'
              ? 'text-xs'
              : 'text-sm'} {selected
              ? 'bg-blue-50 font-medium text-blue-700 dark:bg-blue-950/60 dark:text-blue-300'
              : 'text-zinc-800 hover:bg-zinc-100 dark:text-zinc-100 dark:hover:bg-zinc-800'}"
          >
            <span class="min-w-0 truncate">{opt.label}</span>
            {#if selected}
              <Check
                class="h-3.5 w-3.5 shrink-0 text-blue-600 dark:text-blue-400"
              />
            {/if}
          </button>
        {/each}
      </div>
    </div>
  {/if}
</div>
