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
import Info from "./Info.vue";
import Delete from "./Delete.vue";
import DeleteUser from "./DeleteUser.vue";
import Download from "./Download.vue";
import Rename from "./Rename.vue";
import DiscardEditorChanges from "./DiscardEditorChanges.vue";
import CurrentPassword from "./CurrentPassword.vue";

const layoutStore = useLayoutStore();

const { currentPromptName } = storeToRefs(layoutStore);

const components = new Map<string, any>([
  ["info", Info],
  ["help", Help],
  ["delete", Delete],
  ["rename", Rename],
  ["download", Download],
  ["deleteUser", DeleteUser],
  ["discardEditorChanges", DiscardEditorChanges],
  ["current-password", CurrentPassword],
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
