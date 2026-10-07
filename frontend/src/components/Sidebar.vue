<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <div class="nav-main">
        <button
          class="action"
          :class="{ current: !isSettings }"
          @click="toRoot"
          :aria-label="$t('sidebar.myFiles')"
          :title="$t('sidebar.myFiles')"
        >
          <Icon name="folder" weight="duotone" size="1.5em" />
          <span>{{ $t("sidebar.myFiles") }}</span>
        </button>
      </div>

      <div class="nav-footer">
        <div class="usage" v-if="isFiles && !disableUsedPercentage">
          <progress-bar :val="usage.usedPercentage" size="small"></progress-bar>
          <p>
            {{
              $t("sidebar.diskUsed", {
                used: usage.used,
                total: usage.total,
              })
            }}
          </p>
        </div>

        <button
          class="action"
          :class="{ current: isSettings }"
          @click="toSettings"
          :aria-label="$t('sidebar.settings')"
          :title="$t('sidebar.settings')"
        >
          <Icon name="settings" weight="duotone" size="1.5em" />
          <span>{{ $t("sidebar.settings") }}</span>
        </button>
        <button
          v-if="canLogout"
          @click="logout"
          class="action"
          id="logout"
          :aria-label="$t('sidebar.logout')"
          :title="$t('sidebar.logout')"
        >
          <Icon name="sign-out" weight="duotone" size="1.5em" />
          <span>{{ $t("sidebar.logout") }}</span>
        </button>
        <div class="nav-tools" v-if="isListing">
          <button
            class="action icon-only"
            @click="switchView"
            :aria-label="$t('buttons.switchView')"
            :title="$t('buttons.switchView')"
          >
            <Icon :name="viewMode === 'list' ? 'grid' : 'list'" size="1.4em" />
          </button>
        </div>
      </div>
    </template>
    <template v-else>
      <div class="nav-main">
        <router-link
          class="action"
          to="/login"
          :aria-label="$t('sidebar.login')"
          :title="$t('sidebar.login')"
        >
          <Icon name="sign-in" weight="duotone" size="1.5em" />
          <span>{{ $t("sidebar.login") }}</span>
        </router-link>
      </div>
    </template>
  </nav>
</template>

<script>
import Icon from "@/components/Icon.vue";
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import * as auth from "@/utils/auth";
import {
  disableExternal,
  disableUsedPercentage,
  noAuth,
  logoutPage,
  loginPage,
} from "@/utils/constants";
import { files as api, users } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive(USAGE_DEFAULT);
    return { usage, usageAbortController: new AbortController() };
  },
  components: {
    Icon,
    ProgressBar,
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "isListing", "reload"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    // Only the list and the large grid exist, anything else shows as a list.
    viewMode() {
      return this.user?.viewMode === "mosaic gallery"
        ? "mosaic gallery"
        : "list";
    },
    isSettings() {
      return this.$route.path.startsWith("/settings");
    },
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && (loginPage || logoutPage !== "/login"),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    ...mapActions(useAuthStore, ["updateUser"]),
    switchView() {
      const data = {
        id: this.user?.id,
        viewMode: this.viewMode === "list" ? "mosaic gallery" : "list",
      };

      users.update(data, ["viewMode"]).catch(this.$showError);
      this.updateUser(data);
    },
    abortOngoingFetchUsage() {
      this.usageAbortController.abort();
    },
    async fetchUsage() {
      const path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        this.abortOngoingFetchUsage();
        this.usageAbortController = new AbortController();
        const usage = await api.usage(path, this.usageAbortController.signal);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } finally {
        return Object.assign(this.usage, usageStats);
      }
    },
    toRoot() {
      this.$router.push({ path: "/files" });
      this.closeHovers();
    },
    toSettings() {
      this.$router.push({ path: "/settings/profile" });
      this.closeHovers();
    },
    logout: auth.logout,
  },
  watch: {
    $route: {
      handler(to) {
        if (to.path.includes("/files")) {
          this.fetchUsage();
        }
      },
      immediate: true,
    },
  },
  unmounted() {
    this.abortOngoingFetchUsage();
  },
};
</script>
