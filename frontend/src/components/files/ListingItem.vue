<template>
  <div
    class="item"
    role="button"
    tabindex="0"
    @click="open"
    :data-dir="isDir"
    :data-type="type"
    :aria-label="name"
    :aria-selected="isSelected"
    :data-ext="getExtension(name).toLowerCase()"
  >
    <div>
      <img
        v-if="type === 'image' && isThumbsEnabled"
        v-lazy="thumbnailUrl"
        :alt="name"
      />
      <i v-else class="material-icons"></i>
    </div>

    <div>
      <p class="name">{{ displayName }}</p>

      <p v-if="isDir" class="size" data-order="-1">&mdash;</p>
      <p v-else class="size" :data-order="humanSize()">{{ humanSize() }}</p>

      <p class="extension">{{ extension }}</p>

      <p class="modified">
        <time :datetime="modified">{{ humanTime() }}</time>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useFileStore } from "@/stores/file";

import { enableThumbs } from "@/utils/constants";
import { filesize } from "@/utils";
import dayjs from "dayjs";
import { files as api } from "@/api";
import { computed } from "vue";
import { useRouter } from "vue-router";

const router = useRouter();

const props = defineProps<{
  name: string;
  isDir: boolean;
  url: string;
  type: string;
  size: number;
  modified: string;
  index: number;
  path?: string;
}>();

const fileStore = useFileStore();

const isSelected = computed(
  () => fileStore.selected.indexOf(props.index) !== -1
);
const thumbnailUrl = computed(() => {
  const file = {
    path: props.path,
    modified: props.modified,
  };

  return api.getPreviewURL(file as Resource, "thumb");
});

const isThumbsEnabled = computed(() => {
  return enableThumbs;
});

const humanSize = () => {
  return props.type == "invalid_link" ? "invalid link" : filesize(props.size);
};

const humanTime = () => {
  return dayjs(props.modified).fromNow();
};

const open = () => {
  router.push({ path: props.url });
};

// Folders keep their full name; for files the extension is split off into
// its own column. A leading dot (".hidden") is part of the name, not an extension.
const extIndex = computed(() =>
  props.isDir ? -1 : props.name.lastIndexOf(".")
);
const displayName = computed(() =>
  extIndex.value > 0 ? props.name.substring(0, extIndex.value) : props.name
);
const extension = computed(() =>
  extIndex.value > 0 ? props.name.substring(extIndex.value + 1) : ""
);

const getExtension = (fileName: string): string => {
  const lastDotIndex = fileName.lastIndexOf(".");
  if (lastDotIndex === -1) {
    return fileName;
  }
  return fileName.substring(lastDotIndex);
};
</script>
