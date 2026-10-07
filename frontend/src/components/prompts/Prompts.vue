<template>
  <base-modal v-if="modal != null" :prompt="currentPromptName" @closed="close">
    <keep-alive>
      <component :is="modal" />
    </keep-alive>
  </base-modal>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";
import { useLayoutStore } from "@/stores/layout";

import BaseModal from "./BaseModal.vue";
import Help from "./Help.vue";
import CurrentPassword from "./CurrentPassword.vue";
import MoveTo from "./MoveTo.vue";
import DeleteStaged from "./DeleteStaged.vue";

const layoutStore = useLayoutStore();

const { currentPromptName } = storeToRefs(layoutStore);

const components = new Map<string, any>([
  ["help", Help],
  ["current-password", CurrentPassword],
  ["moveTo", MoveTo],
  ["deleteStaged", DeleteStaged],
]);

const modal = computed(() => {
  const modal = components.get(currentPromptName.value!);
  if (!modal) null;

  return modal;
});

const close = () => {
  if (!layoutStore.currentPrompt) return;
  layoutStore.closeHovers();
};
</script>
