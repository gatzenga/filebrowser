<template>
  <component
    :is="icons[name] ?? icons.file"
    :weight="weight"
    :size="size"
    class="icon"
    aria-hidden="true"
  />
</template>

<script setup lang="ts">
import {
  PhArrowDown,
  PhArrowLeft,
  PhArrowUp,
  PhArrowsClockwise,
  PhArrowsOutSimple,
  PhCaretLeft,
  PhCaretRight,
  PhCircleNotch,
  PhCloudSlash,
  PhFile,
  PhFileArchive,
  PhFilePdf,
  PhFileText,
  PhFilmSlate,
  PhFolderSimple,
  PhGear,
  PhHighDefinition,
  PhHouseSimple,
  PhImage,
  PhLinkBreak,
  PhList,
  PhMagnifyingGlass,
  PhMapPinLine,
  PhMinus,
  PhMusicNotes,
  PhPlus,
  PhRows,
  PhSignIn,
  PhSignOut,
  PhSmileySad,
  PhSquaresFour,
  PhStopCircle,
  PhUser,
  PhWarningCircle,
  PhX,
} from "@phosphor-icons/vue";
import type { Component } from "vue";

// One place that decides which Phosphor icon stands for which name.
const icons: Record<string, Component> = {
  archive: PhFileArchive,
  "arrow-down": PhArrowDown,
  "arrow-left": PhArrowLeft,
  "arrow-up": PhArrowUp,
  audio: PhMusicNotes,
  "caret-left": PhCaretLeft,
  "caret-right": PhCaretRight,
  close: PhX,
  error: PhWarningCircle,
  expand: PhArrowsOutSimple,
  refresh: PhArrowsClockwise,
  file: PhFile,
  folder: PhFolderSimple,
  grid: PhSquaresFour,
  hd: PhHighDefinition,
  home: PhHouseSimple,
  image: PhImage,
  "link-broken": PhLinkBreak,
  list: PhRows,
  menu: PhList,
  minus: PhMinus,
  "not-found": PhMapPinLine,
  offline: PhCloudSlash,
  pdf: PhFilePdf,
  plus: PhPlus,
  sad: PhSmileySad,
  search: PhMagnifyingGlass,
  settings: PhGear,
  "sign-in": PhSignIn,
  "sign-out": PhSignOut,
  spinner: PhCircleNotch,
  stop: PhStopCircle,
  text: PhFileText,
  user: PhUser,
  video: PhFilmSlate,
};

withDefaults(
  defineProps<{
    name: string;
    weight?: "thin" | "light" | "regular" | "bold" | "fill" | "duotone";
    size?: string | number;
  }>(),
  { weight: "regular", size: "1.25em" }
);
</script>
