import { defineStore } from "pinia";

// How many folder levels below a staging folder still count as part of it.
const MAX_STAGING_DEPTH = 20;

export const useFileStore = defineStore("file", {
  // convert to a function
  state: (): {
    req: Resource | null;
    oldReq: Resource | null;
    reload: boolean;
    selected: number[];
    // Items ticked in a staging folder, as indices into req.items.
    picked: number[];
    pickAnchor: number | null;
    isFiles: boolean;
  } => ({
    req: null,
    oldReq: null,
    reload: false,
    selected: [],
    picked: [],
    pickAnchor: null,
    isFiles: false,
  }),
  getters: {
    // route: () => {
    //   const routerStore = useRouterStore();
    //   return routerStore.router.currentRoute;
    // },
    // isFiles: (state) => {
    //   const layoutStore = useLayoutStore();
    //   return !layoutStore.loading && state.route._value.name === "Files";
    // },
    isListing: (state) => {
      return state.isFiles && state?.req?.isDir;
    },
    // A folder whose name starts with an underscore and everything below it,
    // down to 20 levels, is a staging area, the only place where items can be
    // ticked, moved or deleted. The server checks the same rule.
    isStaging: (state) => {
      if (!state.isFiles || !state.req?.isDir) return false;

      const segments = state.req.path.split("/").filter(Boolean);
      for (let i = segments.length - 1; i >= 0; i--) {
        if (segments[i].startsWith("_")) {
          return segments.length - 1 - i <= MAX_STAGING_DEPTH;
        }
      }

      return false;
    },
    pickedPaths: (state): string[] => {
      const items = state.req?.items ?? [];
      return state.picked
        .map((index) => items[index]?.path)
        .filter((path): path is string => !!path);
    },
  },
  actions: {
    // no context as first argument, use `this` instead
    updateRequest(value: Resource | null) {
      const selectedItems = this.selected.map((i) => this.req?.items[i]);
      this.oldReq = this.req;
      this.req = value;

      this.selected = [];
      this.clearPicked();

      if (!this.req?.items) return;
      this.selected = this.req.items
        .filter((item) =>
          selectedItems.some((rItem) => rItem?.url === item.url)
        )
        .map((item) => item.index);
    },
    // Ticks or unticks one item. With range the items between the last one
    // that was ticked and this one are ticked as well.
    pick(index: number, range = false) {
      if (range && this.pickAnchor !== null) {
        const from = Math.min(this.pickAnchor, index);
        const to = Math.max(this.pickAnchor, index);
        const picked = new Set(this.picked);
        for (let i = from; i <= to; i++) picked.add(i);
        this.picked = [...picked];
        return;
      }

      this.picked = this.picked.includes(index)
        ? this.picked.filter((i) => i !== index)
        : [...this.picked, index];
      this.pickAnchor = index;
    },
    clearPicked() {
      this.picked = [];
      this.pickAnchor = null;
    },
    // easily reset state using `$reset`
    clearFile() {
      this.$reset();
    },
  },
});
