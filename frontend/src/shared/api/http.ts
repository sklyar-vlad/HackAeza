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

  const expired =
    res.status === 401 && !!token && useSession.getState().token === token;
  if (expired) useSession.getState().signOut();
  if (!res.ok)
    throw new ApiError(
      res.status,
      body,
      expired,
      Number(res.headers.get("Retry-After")) || undefined,
    );
  return body as T;
};
