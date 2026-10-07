<template>
  <div>
    <sidebar></sidebar>
    <main>
      <router-view></router-view>
    </main>
    <prompts></prompts>
  </div>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import Sidebar from "@/components/Sidebar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import { watch } from "vue";
import { useRoute } from "vue-router";

const layoutStore = useLayoutStore();
const route = useRoute();

watch(route, () => {
  if (layoutStore.currentPromptName !== "success") {
    layoutStore.closeHovers();
  }
});
</script>
