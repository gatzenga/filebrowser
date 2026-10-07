interface ResourceBase {
  path: string;
  name: string;
  size: number;
  extension: string;
  modified: string; // ISO 8601 datetime
  mode: number;
  isDir: boolean;
  isSymlink: boolean;
  type: ResourceType;
  url: string;
}

interface Resource extends ResourceBase {
  items: ResourceItem[];
  numDirs: number;
  numFiles: number;
  sorting: Sorting;
  index: number;
  subtitles?: string[];
  content?: string;
  rawContent?: ArrayBuffer;
}

interface ResourceItem extends ResourceBase {
  index: number;
  subtitles?: string[];
}

type ResourceType =
  | "dir"
  | "video"
  | "audio"
  | "image"
  | "pdf"
  | "text"
  | "blob"
  | "textImmutable";

type DownloadFormat =
  | "zip"
  | "tar"
  | "targz"
  | "tarbz2"
  | "tarxz"
  | "tarlz4"
  | "tarsz"
  | null;

interface BreadCrumb {
  name: string;
  url: string;
}

interface CsvData {
  headers: string[];
  rows: string[][];
}
