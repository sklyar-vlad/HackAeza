import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render } from "@testing-library/react";
import { MotionGlobalConfig } from "motion/react";
import type { ReactElement } from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, vi } from "vitest";
import { useSession } from "@/shared/model/session";

// happy-dom cannot run Web Animations; tests only care about the end state.
MotionGlobalConfig.skipAnimations = true;

type Call = { method: string; url: string; headers: Headers; body: unknown };
type Reply = [status: number, body?: unknown];
type Routes = Record<string, (call: Call) => Reply>;

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  useSession.setState({ token: null, username: null });
});

/** Replaces fetch and XHR with contract-shaped replies keyed by "METHOD /path"; unknown routes answer 404. */
export function stubApi(routes: Routes) {
  const calls: Call[] = [];
  const reply = (call: Call): Reply => {
    calls.push(call);
    const handler = routes[`${call.method} ${call.url.split("?")[0]}`];
    return handler ? handler(call) : [404];
  };
  vi.stubGlobal(
    "XMLHttpRequest",
    class {
      upload: { onprogress?: (e: ProgressEvent) => void } = {};
      onload?: () => void;
      responseType = "";
      status = 0;
      response: unknown = null;
      private call = { headers: new Headers() } as Call;
      open(method: string, url: string) {
        Object.assign(this.call, { method, url });
      }
      setRequestHeader(name: string, value: string) {
        this.call.headers.set(name, value);
      }
      getResponseHeader() {
        return null;
      }
      send(body: Blob) {
        this.call.body = body;
        [this.status, this.response = null] = reply(this.call);
        setTimeout(() => this.onload?.());
      }
    },
  );
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init: RequestInit = {}) => {
      const call: Call = {
        method: init.method ?? "GET",
        url: input,
        headers: new Headers(init.headers),
        body: typeof init.body === "string" ? JSON.parse(init.body) : init.body,
      };
      const [status, body] = reply(call);
      return new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );
  return calls;
}

export function renderRoute(element: ReactElement, path: string, url = path) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      { path, element },
      { path: "/", element: <p>главная</p> },
    ],
    { initialEntries: [url] },
  );
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

export const calculation = {
  version: 1,
  sourceVersion: 1,
  modelVersion: 3,
  calculatedAt: "2026-10-24T12:00:00Z",
};

export const accepted = (jobId: string) => ({
  jobId,
  statusUrl: `/api/v1/jobs/${jobId}`,
  createdAt: "2026-10-24T12:00:00.000Z",
  calculation,
});
