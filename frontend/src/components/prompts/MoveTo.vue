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
        <li class="here">
          <button type="button" :disabled="moving" @click="moveTo(current)">
            <Icon name="move" />
            <span>{{ t("files.moveHere") }}</span>
          </button>
        </li>
        <li v-for="folder in folders" :key="folder.path">
          <button type="button" :disabled="moving" @click="moveTo(folder.path)">
            <Icon name="folder" weight="fill" />
            <span>{{ folder.name }}</span>
          </button>
          <button
            type="button"
            class="enter"
            :disabled="moving"
            :aria-label="t('files.openFolder')"
            :title="t('files.openFolder')"
            @click="load(folder.path)"
          >
            <Icon name="caret-right" />
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
import { inject, onMounted, ref } from "vue";
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
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const current = ref("/");
const folders = ref<ResourceItem[]>([]);
const loading = ref(false);
const moving = ref(false);

// Folders that start with an underscore are staging folders, nothing is
// moved into them, so they are not offered.
const load = async (path: string) => {
  loading.value = true;
  try {
    const dir = await api.fetch(`/files${path}`);
    current.value = path;
    folders.value = dir.items.filter(
      (item) => item.isDir && !item.name.startsWith("_")
    );
  } catch (e: any) {
    $showError(e);
  } finally {
    loading.value = false;
  }
};

const up = () => {
  const parent = current.value.replace(/\/+$/, "").split("/").slice(0, -1);
  load(parent.join("/") || "/");
};

const moveTo = async (destination: string) => {
  moving.value = true;
  try {
    await api.moveItems(fileStore.pickedPaths, destination);
    $showSuccess(t("files.moved"));
    fileStore.reload = true;
    layoutStore.closeHovers();
  } catch (e: any) {
    $showError(
      e instanceof StatusError && e.status === 409
        ? new Error(t("files.moveConflict"))
        : e
    );
  } finally {
    moving.value = false;
  }
};

onMounted(() => load("/"));
</script>
