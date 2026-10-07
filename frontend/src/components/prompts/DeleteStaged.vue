<template>
  <div class="card floating">
    <div class="card-content">
      <p>{{ t("files.deleteSelected", { count: fileStore.picked.length }) }}</p>
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
      <button
        id="focus-prompt"
        class="button button--flat button--red"
        :disabled="deleting"
        @click="remove"
        :aria-label="t('buttons.delete')"
        :title="t('buttons.delete')"
      >
        {{ t("buttons.delete") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { inject, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { files as api } from "@/api";

const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const deleting = ref(false);

const remove = async () => {
  deleting.value = true;
  try {
    await api.deleteItems(fileStore.pickedPaths);
    $showSuccess(t("files.deleted"));
    fileStore.reload = true;
    layoutStore.closeHovers();
  } catch (e: any) {
    $showError(e);
  } finally {
    deleting.value = false;
  }
};
</script>
