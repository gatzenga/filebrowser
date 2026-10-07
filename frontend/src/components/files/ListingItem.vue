<template>
  <div
    class="item"
    role="button"
    tabindex="0"
    @click="open"
    :data-dir="isDir"
    :data-type="type"
    :aria-label="name"
    :data-ext="getExtension(name).toLowerCase()"
  >
    <div>
      <img
        v-if="(type === 'image' || type === 'video') && isThumbsEnabled"
        v-lazy="thumbnailUrl"
        :alt="name"
      />
      <Icon
        v-else
        :name="iconName"
        :weight="isDir ? 'fill' : 'duotone'"
        size="1em"
      />
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
import Icon from "@/components/Icon.vue";
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

const archiveExtensions = [".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz"];

const iconName = computed(() => {
  if (props.isDir) return "folder";
  if (archiveExtensions.includes(getExtension(props.name).toLowerCase())) {
    return "archive";
  }

  switch (props.type) {
    case "video":
    case "audio":
    case "image":
    case "pdf":
    case "text":
      return props.type;
    case "invalid_link":
      return "link-broken";
    default:
      return "file";
  }
});

const getExtension = (fileName: string): string => {
  const lastDotIndex = fileName.lastIndexOf(".");
  if (lastDotIndex === -1) {
    return fileName;
  }
  return fileName.substring(lastDotIndex);
};
</script>
