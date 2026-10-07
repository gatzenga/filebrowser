<template>
  <div class="card floating move-to">
    <div class="card-title">
      <h2>{{ t("files.moveTo") }}</h2>
    </div>

    <div class="card-content">
      <div class="move-path">
        <button
          v-if="current !== '/'"
          type="button"
          class="move-up"
          :aria-label="t('buttons.back')"
          :title="t('buttons.back')"
          @click="up"
        >
          <Icon name="arrow-left" />
        </button>
        <span>{{ current }}</span>
      </div>

      <ul class="move-list" :class="{ busy: loading || moving }">
        <li v-for="folder in folders" :key="folder.path">
          <button type="button" :disabled="busy" @click="choose(folder.path)">
            <Icon name="folder" weight="fill" />
            <span>{{ folder.name }}</span>
          </button>
        </li>
        <li v-if="!loading && folders.length === 0" class="empty">
          {{ t("files.noFolders") }}
        </li>
      </ul>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="layoutStore.closeHovers()"
        :aria-label="t('buttons.cancel')"
        :title="t('buttons.cancel')"
      >
        {{ t("buttons.cancel") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import Icon from "@/components/Icon.vue";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { files as api } from "@/api";
import { StatusError } from "@/api/utils";

const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const $showError = inject<IToastError>("$showError")!;

// How long a failure message stays, in milliseconds.
const ERROR_TIMEOUT = 2000;

const current = ref("/");
const folders = ref<ResourceItem[]>([]);
const loading = ref(false);
const moving = ref(false);

// Folders that start with an underscore are staging folders, nothing is
// moved into them, so they are not offered.
const subfolders = async (path: string) => {
  const dir = await api.fetch(`/files${path}`);
  return dir.items.filter((item) => item.isDir && !item.name.startsWith("_"));
};

const busy = computed(() => loading.value || moving.value);

const load = async (path: string) => {
  loading.value = true;
  try {
    folders.value = await subfolders(path);
    current.value = path;
  } catch (e: any) {
    $showError(e, false, ERROR_TIMEOUT);
  } finally {
    loading.value = false;
  }
};

// A folder that still has folders in it is opened. One without any is where
// the items go, so the click moves them there.
const choose = async (path: string) => {
  loading.value = true;
  try {
    const inside = await subfolders(path);
    if (inside.length > 0) {
      folders.value = inside;
      current.value = path;
      return;
    }
  } catch (e: any) {
    $showError(e, false, ERROR_TIMEOUT);
    return;
  } finally {
    loading.value = false;
  }

  await moveTo(path);
};

const up = () => {
  const parent = current.value.replace(/\/+$/, "").split("/").slice(0, -1);
  load(parent.join("/") || "/");
};

const moveTo = async (destination: string) => {
  moving.value = true;
  try {
    await api.moveItems(fileStore.pickedPaths, destination);
    fileStore.reload = true;
    layoutStore.closeHovers();
  } catch (e: any) {
    // The page shows what happened, so only a failure needs a message, and
    // that one goes away by itself.
    $showError(
      e instanceof StatusError && e.status === 409
        ? new Error(t("files.moveConflict"))
        : e,
      false,
      ERROR_TIMEOUT
    );
  } finally {
    moving.value = false;
  }
};

onMounted(() => load("/"));
</script>
