import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import type { CalculationUpdatedEvent } from "@/shared/api/generated/api";
import { useSession } from "@/shared/model/session";

/**
 * Optional SSE stream (GET /api/v1/events). Native EventSource cannot send the
 * Authorization header, so the stream is read with fetch. A newer published
 * version refreshes every query; without the endpoint the app simply keeps
 * TanStack Query's refetch-on-focus behaviour.
 */
export function useCalculationEvents() {
  const token = useSession((s) => s.token);
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!token) return;
    const abort = new AbortController();
    let version = -1;

    const listen = async () => {
      while (!abort.signal.aborted) {
        try {
          const res = await fetch("/api/v1/events", {
            headers: {
              Authorization: `Bearer ${token}`,
              Accept: "text/event-stream",
            },
            signal: abort.signal,
          });
          if (res.status === 404 || res.status === 401) return;
          if (!res.ok || !res.body) throw new Error(String(res.status));

          const reader = res.body
            .pipeThrough(new TextDecoderStream())
            .getReader();
          let buffer = "";
          for (
            let chunk = await reader.read();
            !chunk.done;
            chunk = await reader.read()
          ) {
            buffer += chunk.value;
            const frames = buffer.split("\n\n");
            buffer = frames.pop() ?? "";
            for (const frame of frames) {
              const data = frame
                .split("\n")
                .filter((line) => line.startsWith("data:"))
                .map((line) => line.slice(5).trim())
                .join("");
              if (!data) continue;
              const event = JSON.parse(data) as CalculationUpdatedEvent;
              if (version >= 0 && event.calculation.version !== version)
                queryClient.invalidateQueries();
              version = event.calculation.version;
            }
          }
        } catch {
          if (abort.signal.aborted) return;
        }
        await new Promise((resolve) => setTimeout(resolve, 5000));
      }
    };

    listen();
    return () => abort.abort();
  }, [token, queryClient]);
}
