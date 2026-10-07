<template>
  <header>
    <Action
      v-if="showMenu"
      class="menu-button"
      icon="menu"
      :label="t('buttons.toggleSidebar')"
      @action="layoutStore.showHover('sidebar')"
    />

    <slot />
  </header>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";

import Action from "@/components/header/Action.vue";
import { useI18n } from "vue-i18n";

defineProps<{
  showMenu?: boolean;
}>();

const layoutStore = useLayoutStore();

const { t } = useI18n();
</script>
