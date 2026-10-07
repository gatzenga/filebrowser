<template>
  <button
    @click="emit('action')"
    :aria-label="label"
    :title="label"
    class="action"
  >
    <Icon :name="icon ?? 'file'" size="1.4em" />
    <span>{{ label }}</span>
  </button>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
defineProps<{
  icon?: string;
  label?: string;
}>();

const emit = defineEmits<{
  (e: "action"): any;
}>();
</script>
