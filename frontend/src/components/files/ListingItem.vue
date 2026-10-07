<template>
  <div
    class="item"
    role="button"
    tabindex="0"
    @click="open"
    @contextmenu="openMenu"
    :data-dir="isDir"
    :data-type="type"
    :aria-label="name"
    :data-ext="getExtension(name).toLowerCase()"
  >
    <div>
      <img
        v-if="(type === 'image' || type === 'video') && isThumbsEnabled"
        :key="thumbVersion"
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

      <p class="duration">{{ humanDuration }}</p>
    </div>

    <Teleport to="body">
      <ul
        v-if="menu"
        class="thumb-menu"
        role="menu"
        :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
        @click.stop
        @contextmenu.prevent.stop
      >
        <li role="none">
          <button
            type="button"
            role="menuitem"
            :disabled="renewing"
            @click="renewThumbnail"
          >
            <Icon name="refresh" />
            {{ t("files.renewThumbnail") }}
          </button>
        </li>
      </ul>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
import { enableThumbs } from "@/utils/constants";
import { filesize } from "@/utils";
import { files as api } from "@/api";
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

const router = useRouter();

const props = defineProps<{
  name: string;
  isDir: boolean;
  url: string;
  type: string;
  size: number;
  modified: string;
  duration?: number;
  index: number;
  path?: string;
}>();

const { t } = useI18n();
const $showError = inject<IToastError>("$showError")!;

const thumbVersion = ref(0);
const renewing = ref(false);
const menu = ref<{ x: number; y: number } | null>(null);

const canRenew = computed(() => props.type === "video" && enableThumbs);

const closeMenu = () => {
  menu.value = null;
  window.removeEventListener("click", closeMenu);
  window.removeEventListener("keydown", closeOnEscape);
  window.removeEventListener("scroll", closeMenu, true);
};

const closeOnEscape = (event: KeyboardEvent) => {
  if (event.key === "Escape") closeMenu();
};

// Right click on a video offers to roll a new thumbnail.
const openMenu = (event: MouseEvent) => {
  if (!canRenew.value) return;

  event.preventDefault();
  menu.value = { x: event.clientX, y: event.clientY };
  window.addEventListener("click", closeMenu);
  window.addEventListener("keydown", closeOnEscape);
  window.addEventListener("scroll", closeMenu, true);
};

const renewThumbnail = async () => {
  if (!props.path || renewing.value) return;

  renewing.value = true;
  try {
    await api.renewThumbnail(props.path);
    thumbVersion.value++;
  } catch (e: any) {
    $showError(e);
  } finally {
    renewing.value = false;
    closeMenu();
  }
};

onBeforeUnmount(closeMenu);

const thumbnailUrl = computed(() => {
  const file = {
    path: props.path,
    modified: props.modified,
  };

  const url = api.getPreviewURL(file as Resource, "thumb");

  // A renewed thumbnail has the same address, so make the browser ask again.
  return thumbVersion.value ? `${url}&v=${thumbVersion.value}` : url;
});

const isThumbsEnabled = computed(() => {
  return enableThumbs;
});

const humanSize = () => {
  return props.type == "invalid_link" ? "invalid link" : filesize(props.size);
};

// The listing only knows lengths that are already stored, ask for the others.
const fetchedDuration = ref(0);
const durationRequest = new AbortController();

onMounted(async () => {
  if (props.type !== "video" || props.duration || !props.path) return;

  try {
    fetchedDuration.value = await api.getDuration(
      props.path,
      durationRequest.signal
    );
  } catch {
    // No length is not worth an error message, the column stays empty.
  }
});

onBeforeUnmount(() => durationRequest.abort());

const humanDuration = computed(() => {
  const length = props.duration || fetchedDuration.value;
  if (props.type !== "video" || !length) return "";

  const total = Math.round(length);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = String(total % 60).padStart(2, "0");

  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${seconds}`
    : `${minutes}:${seconds}`;
});

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
