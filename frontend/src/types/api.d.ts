type ApiMethod = "GET" | "POST" | "PUT" | "DELETE" | "PATCH";

interface ApiOpts {
  method?: ApiMethod;
  headers?: object;
  body?: any;
  signal?: AbortSignal;
}
