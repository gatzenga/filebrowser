import { beforeEach, describe, expect, it } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useFileStore } from "@/stores/file";

// The server sorts folders and files together. Depending on the sorting the
// folders end up at the end, but the listing always shows them first.
const item = (index: number, name: string, isDir: boolean) =>
  ({ index, name, isDir, path: `/_inbox/${name}` }) as ResourceItem;

const listing = (items: ResourceItem[]) =>
  ({ isDir: true, path: "/_inbox", items }) as unknown as Resource;

describe("ticking a range in a staging folder", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("follows the order the items are shown in when folders come last", () => {
    const store = useFileStore();
    // Data order: four files, then the folder. Shown: folder, then the files.
    store.req = listing([
      item(0, "a.mkv", false),
      item(1, "b.mkv", false),
      item(2, "c.mkv", false),
      item(3, "d.mkv", false),
      item(4, "Folder", true),
    ]);

    store.pick(4); // the folder, shown first
    store.pick(2, true); // shift on the third file, c.mkv

    expect([...store.picked].sort()).toEqual([0, 1, 2, 4]);
  });

  it("does not tick what lies after the clicked item", () => {
    const store = useFileStore();
    store.req = listing([
      item(0, "Folder", true),
      item(1, "a.mkv", false),
      item(2, "b.mkv", false),
      item(3, "c.mkv", false),
      item(4, "d.mkv", false),
    ]);

    store.pick(0);
    store.pick(3, true);

    expect([...store.picked].sort()).toEqual([0, 1, 2, 3]);
  });

  it("works upwards from a later item too", () => {
    const store = useFileStore();
    store.req = listing([
      item(0, "a.mkv", false),
      item(1, "b.mkv", false),
      item(2, "c.mkv", false),
      item(3, "Folder", true),
    ]);

    store.pick(2); // c.mkv, shown last
    store.pick(3, true); // the folder, shown first

    expect([...store.picked].sort()).toEqual([0, 1, 2, 3]);
  });

  it("a click without shift ticks and unticks a single item", () => {
    const store = useFileStore();
    store.req = listing([item(0, "a.mkv", false), item(1, "b.mkv", false)]);

    store.pick(1);
    expect(store.picked).toEqual([1]);
    store.pick(1);
    expect(store.picked).toEqual([]);
  });
});
