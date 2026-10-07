<template>
  <div
    id="previewer"
    @touchmove.prevent.stop
    @wheel.prevent.stop
    @mousemove="toggleNavigation"
    @touchstart="toggleNavigation"
  >
    <header-bar v-if="isPdf || isEpub || showNav">
      <action icon="close" :label="$t('buttons.close')" @action="close()" />
      <title>{{ name }}</title>
      <action
        :disabled="layoutStore.loading"
        v-if="isResizeEnabled && fileStore.req?.type === 'image'"
        :icon="fullSize ? 'expand' : 'hd'"
        @action="toggleSize"
      />
    </header-bar>

    <div class="loading delayed" v-if="layoutStore.loading">
      <div class="spinner">
        <div class="bounce1"></div>
        <div class="bounce2"></div>
        <div class="bounce3"></div>
      </div>
    </div>
    <template v-else>
      <div class="preview">
        <div v-if="isEpub" class="epub-reader">
          <vue-reader
            :location="location"
            :url="previewUrl"
            :get-rendition="getRendition"
            :epubInitOptions="{
              requestCredentials: true,
            }"
            :epubOptions="{
              allowPopups: true,
            }"
            @update:location="locationChange"
          />
          <div class="size">
            <button
              @click="changeSize(Math.max(100, size - 10))"
              class="reader-button"
            >
              <Icon name="minus" />
            </button>
            <button
              @click="changeSize(Math.min(150, size + 10))"
              class="reader-button"
            >
              <Icon name="plus" />
            </button>
            <span>{{ size }}%</span>
          </div>
        </div>
        <ExtendedImage
          v-else-if="fileStore.req?.type == 'image'"
          :src="previewUrl"
        />
        <audio
          v-else-if="fileStore.req?.type == 'audio'"
          ref="player"
          :src="previewUrl"
          controls
          :autoplay="autoPlay"
          @play="autoPlay = true"
        ></audio>
        <VideoPlayer
          v-else-if="fileStore.req?.type == 'video'"
          ref="player"
          :source="previewUrl"
          :subtitles="subtitles"
          :options="videoOptions"
        >
        </VideoPlayer>
        <object v-else-if="isPdf" class="pdf" :data="previewUrl"></object>
        <div v-else class="info">
          <div class="title">
            <Icon name="error" weight="duotone" size="1.5em" />
            {{ $t("files.noPreview") }}
          </div>
        </div>
      </div>
    </template>

    <button
      @click="prev"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasPrevious || !showNav }"
      :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')"
    >
      <Icon name="caret-left" size="1.6em" />
    </button>
    <button
      @click="next"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasNext || !showNav }"
      :aria-label="$t('buttons.next')"
      :title="$t('buttons.next')"
    >
      <Icon name="caret-right" size="1.6em" />
    </button>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
import { useStorage } from "@vueuse/core";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import { files as api } from "@/api";
import { createURL } from "@/api/utils";
import { resizePreview } from "@/utils/constants";
import url from "@/utils/url";
import { throttle } from "lodash-es";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import VideoPlayer from "@/components/files/VideoPlayer.vue";
import { VueReader } from "vue-reader";
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { Rendition } from "epubjs";
import { getTheme } from "@/utils/theme";

const location = useStorage("book-progress", 0, undefined, {
  serializer: {
    read: (v) => JSON.parse(v),
    write: (v) => JSON.stringify(v),
  },
});
const size = useStorage("book-size", 120, undefined, {
  serializer: {
    read: (v) => JSON.parse(v),
    write: (v) => JSON.stringify(v),
  },
});

const locationChange = (epubcifi: number) => {
  location.value = epubcifi;
};
let rendition: Rendition | null = null;
const changeSize = (val: number) => {
  size.value = val;
  rendition?.themes.fontSize(`${val}%`);
};

const getRendition = (_rendition: Rendition) => {
  rendition = _rendition;
  switch (getTheme()) {
    case "dark": {
      rendition.themes.override("color", "rgba(255, 255, 255, 0.6)");
      break;
    }
    case "light": {
      rendition.themes.override("color", "rgb(111, 111, 111)");
      break;
    }
  }
  rendition.themes.registerRules("h2Transparent", {
    "h1,h2,h3,h4": {
      "background-color": "transparent !important",
    },
  });
  rendition?.themes.fontSize(`${size.value}%`);
  rendition.themes.select("h2Transparent");
  rendition.themes.override("background-color", "transparent", true);
};

const mediaTypes: ResourceType[] = ["image", "video", "audio", "blob"];

const previousLink = ref<string>("");
const nextLink = ref<string>("");
const listing = ref<ResourceItem[] | null>(null);
const name = ref<string>("");
const fullSize = ref<boolean>(false);
const showNav = ref<boolean>(true);
const navTimeout = ref<null | number>(null);
const hoverNav = ref<boolean>(false);
const autoPlay = ref<boolean>(false);
const previousRaw = ref<string>("");
const nextRaw = ref<string>("");

const player = ref<HTMLVideoElement | HTMLAudioElement | null>(null);

const $showError = inject<IToastError>("$showError")!;

const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const route = useRoute();
const router = useRouter();

const hasPrevious = computed(() => previousLink.value !== "");

const hasNext = computed(() => nextLink.value !== "");

const previewUrl = computed(() => {
  if (!fileStore.req) {
    return "";
  }

  if (fileStore.req.type === "image" && !fullSize.value) {
    return api.getPreviewURL(fileStore.req, "big");
  }

  if (isEpub.value) {
    return createURL("api/raw" + fileStore.req.path, {});
  }

  return api.getRawURL(fileStore.req);
});

const isPdf = computed(() => fileStore.req?.extension.toLowerCase() == ".pdf");
const isEpub = computed(
  () => fileStore.req?.extension.toLowerCase() == ".epub"
);

const isResizeEnabled = computed(() => resizePreview);

const subtitles = computed(() => {
  if (fileStore.req?.subtitles) {
    return api.getSubtitlesURL(fileStore.req);
  }
  return [];
});

const videoOptions = computed(() => {
  return { autoplay: autoPlay.value };
});

watch(route, () => {
  updatePreview();
  toggleNavigation();
});

// Specify hooks
onMounted(async () => {
  window.addEventListener("keydown", key);
  listing.value = fileStore.oldReq?.items ?? null;
  updatePreview();
});

onBeforeUnmount(() => window.removeEventListener("keydown", key));

// Specify methods
const prev = () => {
  hoverNav.value = false;
  router.replace({ path: previousLink.value });
};

const next = () => {
  hoverNav.value = false;
  router.replace({ path: nextLink.value });
};

const key = (event: KeyboardEvent) => {
  if (layoutStore.currentPrompt !== null) {
    return;
  }
  // When previewing a video, let arrow keys fall through to video.js for
  // seeking instead of switching to the prev/next file. Enter still advances.
  const isVideo = fileStore.req?.type === "video";
  if (event.which === 13) {
    // enter
    if (hasNext.value) next();
  } else if (event.which === 39) {
    // right arrow
    if (isVideo) return;
    if (hasNext.value) next();
  } else if (event.which === 37) {
    // left arrow
    if (isVideo) return;
    if (hasPrevious.value) prev();
  } else if (event.which === 27) {
    // esc
    close();
  }
};
const updatePreview = async () => {
  if (player.value && player.value.paused && !player.value.ended) {
    autoPlay.value = false;
  }

  const dirs = route.fullPath.split("/");
  name.value = decodeURIComponent(dirs[dirs.length - 1]);

  if (!listing.value) {
    try {
      const path = url.removeLastDir(route.path);
      const res = await api.fetch(path);
      listing.value = res.items;
    } catch (e: any) {
      $showError(e);
    }
  }

  previousLink.value = "";
  nextLink.value = "";
  if (listing.value) {
    for (let i = 0; i < listing.value.length; i++) {
      if (listing.value[i].name !== name.value) {
        continue;
      }

      for (let j = i - 1; j >= 0; j--) {
        if (mediaTypes.includes(listing.value[j].type)) {
          previousLink.value = listing.value[j].url;
          previousRaw.value = prefetchUrl(listing.value[j]);
          break;
        }
      }
      for (let j = i + 1; j < listing.value.length; j++) {
        if (mediaTypes.includes(listing.value[j].type)) {
          nextLink.value = listing.value[j].url;
          nextRaw.value = prefetchUrl(listing.value[j]);
          break;
        }
      }

      return;
    }
  }
};

const prefetchUrl = (item: ResourceItem) => {
  if (item.type !== "image") {
    return "";
  }

  return fullSize.value ? api.getRawURL(item) : api.getPreviewURL(item, "big");
};

const toggleSize = () => (fullSize.value = !fullSize.value);

const toggleNavigation = throttle(function () {
  showNav.value = true;

  if (navTimeout.value) {
    clearTimeout(navTimeout.value);
  }

  navTimeout.value = window.setTimeout(() => {
    showNav.value = false || hoverNav.value;
    navTimeout.value = null;
  }, 1500);
}, 500);

const close = () => {
  const uri = url.removeLastDir(route.path) + "/";
  router.push({ path: uri });
};
</script>
