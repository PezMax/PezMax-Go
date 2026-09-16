<script setup lang="ts">
import { computed } from 'vue';

import { ChevronsLeft, ChevronsRight } from '@vben-core/icons';

const collapsed = defineModel<boolean>('collapsed');

const buttonLabel = computed(() => (collapsed.value ? '展开菜单' : '收回菜单'));

function handleCollapsed() {
  collapsed.value = !collapsed.value;
}
</script>

<template>
  <button
    :aria-label="buttonLabel"
    :title="buttonLabel"
    class="absolute inset-x-2 bottom-px z-10 flex h-10 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-foreground/60 transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    type="button"
    @click.stop="handleCollapsed"
  >
    <ChevronsRight v-if="collapsed" class="size-4" />
    <ChevronsLeft v-else class="size-4" />
  </button>
</template>
