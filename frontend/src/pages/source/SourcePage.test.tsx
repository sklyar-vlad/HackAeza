import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  accepted,
  calculation,
  renderRoute,
  stubApi,
} from "@/shared/lib/testing";
import { SourcePage } from "./SourcePage";

const status = {
  snapshotPresent: false,
  counts: { accounts: 0, services: 0, transactions: 0 },
  receivedAt: null,
  capturedAt: null,
  calculation: {
    ...calculation,
    version: 0,
    sourceVersion: 0,
    calculatedAt: null,
  },
  pendingJobId: null,
};

const snapshot = {
  accounts: [
    {
      id: 1,
      email: "client@example.com",
      balance: 500,
      withdrawBalance: 0,
      bonusBalance: 0,
      bonusState: "locked",
      totalReplenished: 1000,
    },
  ],
  services: [],
  transactions: [],
};

const pick = (container: HTMLElement, text: string) =>
  fireEvent.change(container.querySelector("input[type=file]")!, {
    target: {
      files: [new File([text], "data.json", { type: "application/json" })],
    },
  });

describe("SourcePage", () => {
  it("rejects a file that does not match the snapshot schema before uploading", async () => {
    const calls = stubApi({ "GET /api/v1/source/status": () => [200, status] });
    const { container } = renderRoute(<SourcePage />, "/source");

    pick(container, JSON.stringify({ accounts: "nope" }));

    expect(await screen.findByText(/^accounts:/)).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Загрузить и пересчитать" }),
    ).toHaveProperty("disabled", true);
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });

  it("uploads a valid snapshot and shows accepted counts and warnings", async () => {
    const calls = stubApi({
      "GET /api/v1/source/status": () => [200, status],
      "PUT /api/v1/source/snapshot": () => [202, accepted("job-2")],
      "GET /api/v1/jobs/job-2": () => [
        200,
        {
          jobId: "job-2",
          kind: "source_snapshot",
          status: "succeeded",
          createdAt: "2026-10-24T12:00:00Z",
          startedAt: "2026-10-24T12:00:01Z",
          finishedAt: "2026-10-24T12:00:05Z",
          attempt: 1,
          result: {
            counts: { accounts: 1, services: 0, transactions: 0 },
            warnings: [
              {
                code: "STALE_PROLONG_WAIT",
                message: "Старый prolong_wait",
                path: "services[0].status",
              },
            ],
            calculation,
          },
        },
      ],
    });
    const { container } = renderRoute(<SourcePage />, "/source");

    pick(container, JSON.stringify(snapshot));
    const upload = screen.getByRole("button", {
      name: "Загрузить и пересчитать",
    });
    await waitFor(() => expect(upload).toHaveProperty("disabled", false));
    fireEvent.click(upload);

    expect(await screen.findByText("Предупреждения: 1")).toBeTruthy();
    expect(screen.getByText("Старый prolong_wait")).toBeTruthy();
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toEqual(snapshot);
    expect(put.headers.get("Idempotency-Key")).toBeTruthy();
  });
});
