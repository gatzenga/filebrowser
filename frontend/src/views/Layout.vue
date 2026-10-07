<template>
  <div>
    <sidebar></sidebar>
    <main>
      <router-view></router-view>
      <shell
        v-if="
          enableExec && authStore.isLoggedIn && authStore.user?.perm.execute
        "
      />
    </main>
    <prompts></prompts>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { useFileStore } from "@/stores/file";
import Sidebar from "@/components/Sidebar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Shell from "@/components/Shell.vue";
import { enableExec } from "@/utils/constants";
import { watch } from "vue";
import { useRoute } from "vue-router";

const layoutStore = useLayoutStore();
const authStore = useAuthStore();
const fileStore = useFileStore();
const route = useRoute();

watch(route, () => {
  fileStore.selected = [];
  fileStore.multiple = false;
  if (layoutStore.currentPromptName !== "success") {
    layoutStore.closeHovers();
  }
});
</script>
