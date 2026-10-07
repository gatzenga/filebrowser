<template>
  <div>
    <header-bar v-if="showHeader" showMenu />

    <h2 class="message">
      <Icon :name="info.icon" size="3.5em" weight="duotone" />
      <span>{{ t(info.message) }}</span>
    </h2>
  </div>
</template>

<script setup lang="ts">
import Icon from "@/components/Icon.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n({});

const errors: {
  [key: number]: {
    icon: string;
    message: string;
  };
} = {
  0: {
    icon: "offline",
    message: "errors.connection",
  },
  403: {
    icon: "error",
    message: "errors.forbidden",
  },
  404: {
    icon: "not-found",
    message: "errors.notFound",
  },
  500: {
    icon: "error",
    message: "errors.internal",
  },
};

const props = withDefaults(
  defineProps<{
    errorCode?: number;
    showHeader?: boolean;
  }>(),
  {
    errorCode: 500,
    showHeader: false,
  }
);

const info = computed(() => {
  return errors[props.errorCode] ? errors[props.errorCode] : errors[500];
});
</script>
