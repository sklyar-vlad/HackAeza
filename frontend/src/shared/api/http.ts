import { useSession } from "@/shared/model/session";

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
    requestId: string;
    details?: { path: string; message: string }[];
  };
};

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly body: ApiErrorBody | null,
    readonly sessionExpired = false,
    readonly retryAfter?: number,
  ) {
    super(body?.error.message ?? `HTTP ${status}`);
  }
}

const failure = (
  status: number,
  body: unknown,
  token: string | null,
  retryAfter: string | null,
) => {
  const expired =
    status === 401 && !!token && useSession.getState().token === token;
  if (expired) useSession.getState().signOut();
  return new ApiError(
    status,
    body as ApiErrorBody | null,
    expired,
    Number(retryAfter) || undefined,
  );
};

// Mutator for the orval-generated client: Bearer session, JSON, contract error shape.
export const http = async <T>(
  url: string,
  init: RequestInit = {},
): Promise<T> => {
  const headers = new Headers(init.headers);
  const token = useSession.getState().token;
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const res = await fetch(url, { ...init, headers });
  const body = res.status === 204 ? null : await res.json().catch(() => null);

  if (!res.ok)
    throw failure(res.status, body, token, res.headers.get("Retry-After"));
  return body as T;
};

export type UploadProgress = { loaded: number; total: number };

// fetch cannot report request body progress, so large uploads go through XHR with the same contract.
export const upload = <T>(
  method: string,
  url: string,
  body: Blob,
  headers: Record<string, string>,
  onProgress: (progress: UploadProgress) => void,
): Promise<T> =>
  new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const token = useSession.getState().token;
    xhr.open(method, url);
    xhr.responseType = "json";
    for (const [name, value] of Object.entries(headers))
      xhr.setRequestHeader(name, value);
    if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    xhr.upload.onprogress = (e) =>
      onProgress({
        loaded: e.loaded,
        total: e.lengthComputable ? e.total : body.size,
      });
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300
        ? resolve(xhr.response as T)
        : reject(
            failure(
              xhr.status,
              xhr.response,
              token,
              xhr.getResponseHeader("Retry-After"),
            ),
          );
    xhr.onerror = () => reject(new TypeError("Network request failed"));
    xhr.send(body);
  });
