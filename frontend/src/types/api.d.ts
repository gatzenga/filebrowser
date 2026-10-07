type ApiMethod = "GET" | "POST" | "PUT" | "DELETE" | "PATCH";

interface ApiOpts {
  method?: ApiMethod;
  headers?: object;
  body?: any;
  signal?: AbortSignal;
}

type ChecksumAlg = "md5" | "sha1" | "sha256" | "sha512";

interface SearchParams {
  [key: string]: string;
}
