<template>
  <button
    @click="emit('action')"
    :aria-label="label"
    :title="label"
    class="action"
  >
    <i class="material-icons">{{ icon }}</i>
    <span>{{ label }}</span>
  </button>
</template>

<script setup lang="ts">
defineProps<{
  icon?: string;
  label?: string;
}>();

const emit = defineEmits<{
  (e: "action"): any;
}>();
</script>
