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

  if (res.status === 401 && token) useSession.getState().signOut();
  if (!res.ok) throw new ApiError(res.status, body);
  return body as T;
};
