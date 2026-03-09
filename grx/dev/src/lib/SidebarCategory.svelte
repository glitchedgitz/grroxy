<script lang="ts">
  import type { ApiEndpoint } from "$lib/api";
  import { cn } from "$lib/utils";

  let {
    categoryName,
    endpoints,
    selectedEndpoint = null,
    onselect,
  }: {
    categoryName: string;
    endpoints: ApiEndpoint[];
    selectedEndpoint: ApiEndpoint | null;
    onselect: (endpoint: ApiEndpoint) => void;
  } = $props();

  let hoverItem = $state(-10);
</script>

{#if endpoints.length > 0}
  {@const cattegoriesparts = categoryName.replace(/^\/api\/?/, '').split("/").filter((p) => p)}
  <div
    class="mb-32"
    role="group"
    aria-label={categoryName}
    onmouseleave={() => {
      hoverItem = -10;
    }}
  >
    <div
      class="mb-4 px-24 gap-8 flex flex-row items-center text-xs font-OCR text-stone uppercase tracking-[2px]"
    >
      {#each cattegoriesparts as part, index}
        <div class="p-4 text-xs font-normal text-stone font-OCR">
          {part}
        </div>
        {#if cattegoriesparts.length != index + 1}
          /
        {/if}
      {/each}
    </div>
    <div class="flex flex-col gap-4">
      {#each endpoints as endpoint, index}
        {@const selected = selectedEndpoint?.path === endpoint.path && selectedEndpoint?.method === endpoint.method}
        {@const cropped = endpoint.path.startsWith(categoryName)
          ? endpoint.path.slice(categoryName.length)
          : endpoint.path}
        {@const parts = cropped.split("/").filter((p) => p)}
        <div
          role="button"
          tabindex="0"
          class={cn(
            "flex text-[8px] tracking-[2px] uppercase text-white/70 font-OCR flex-row font-normal hover:text-white group items-center gap-4 transition-all duration-200 origin-left cursor-pointer",
            selected && "text-white",
          )}
          onmouseenter={() => {
            hoverItem = index;
          }}
          onclick={() => onselect(endpoint)}
          onkeydown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onselect(endpoint);
            }
          }}
        >
          <div
            style="transition-timing-function: cubic-bezier(0.1, 0.9, 0.3, 1);"
            class={cn(
              "w-[20px] h-[1px] bg-white/20 transition-all duration-500",
              hoverItem === index - 1 && "w-[40px]",
              hoverItem === index + 1 && "w-[40px]",
              hoverItem === index - 2 && "w-[28px]",
              hoverItem === index + 2 && "w-[28px]",
              hoverItem === index && "bg-white w-[50px]",
              selected && "bg-white w-[60px]",
            )}
          ></div>
          {#if cropped !== endpoint.path}
            <div class="text-stone opacity-50 ml-8">/</div>
          {/if}
          {#each parts as part, i}
            <div class="p-4">
              {part}
            </div>
            {#if parts.length != i + 1}
              <div class="text-stone opacity-50">/</div>
            {/if}
          {/each}
        </div>
      {/each}
    </div>
  </div>
{/if}
