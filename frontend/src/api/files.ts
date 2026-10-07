import { createURL, fetchURL, removePrefix, StatusError } from "./utils";
import { encodePath } from "@/utils/url";

export async function fetch(url: string, signal?: AbortSignal) {
  url = removePrefix(url);
  const res = await fetchURL(`/api/resources${url}`, { signal });

  let data: Resource;
  try {
    data = (await res.json()) as Resource;
  } catch (e) {
    // Check if the error is an intentional cancellation
    if (e instanceof Error && e.name === "AbortError") {
      throw new StatusError("000 No connection", 0, true);
    }
    throw e;
  }
  data.url = `/files${url}`;

  if (data.isDir) {
    if (!data.url.endsWith("/")) data.url += "/";
    // Perhaps change the any
    data.items = data.items.map((item: any, index: any) => {
      item.index = index;
      item.url = `${data.url}${encodeURIComponent(item.name)}`;

      if (item.isDir) {
        item.url += "/";
      }

      return item;
    });
  }

  return data;
}

export function getRawURL(file: ResourceItem) {
  return createURL("api/raw" + file.path, { inline: "true" });
}

export function getPreviewURL(file: ResourceItem, size: string) {
  const params = {
    inline: "true",
    key: Date.parse(file.modified),
  };

  return createURL("api/preview/" + size + file.path, params);
}

// Replaces the stored thumbnail of a video with a frame from another position.
export async function renewThumbnail(path: string) {
  await fetchURL(`/api/thumbnail${encodePath(path)}`, { method: "POST" });
}

export function getSubtitlesURL(file: ResourceItem) {
  const params = {
    inline: "true",
  };

  return file.subtitles?.map((d) => createURL("api/subtitle" + d, params));
}

export async function usage(url: string, signal: AbortSignal) {
  url = removePrefix(url);

  const res = await fetchURL(`/api/usage${url}`, { signal });

  try {
    return await res.json();
  } catch (e) {
    // Check if the error is an intentional cancellation
    if (e instanceof Error && e.name == "AbortError") {
      throw new StatusError("000 No connection", 0, true);
    }
    throw e;
  }
}
