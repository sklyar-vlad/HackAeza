import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  accepted,
  calculation,
  renderRoute,
  stubApi,
} from "@/shared/lib/testing";
import { ModelPage } from "./ModelPage";

const settings = {
  settings: {
    version: 3,
    updatedAt: "2026-10-23T18:00:00Z",
    coefficients: {
      renewalWithoutAutoProlong: 0.4,
      insufficientRenewalFunds: 0.35,
    },
    thresholds: { mediumFrom: 40, highFrom: 70 },
    horizonDays: 30,
    refreshIntervalSeconds: 3600,
    scoreFormula: "",
    lossFormula: "",
  },
  calculation,
  pendingJobId: null,
};

const features = {
  items: [
    ["renewalWithoutAutoProlong", "Автопродление выключено", 0.4],
    ["insufficientRenewalFunds", "Не хватает средств на продление", 0.35],
  ].map(([key, name, defaultCoefficient]) => ({
    key,
    name,
    description: "",
    defaultCoefficient,
    requiredFields: [],
    missingDataPolicy: "skip",
  })),
};

// The form is filled from GET /model/settings after the first render.
const ready = () => screen.findByDisplayValue("3600");

// happy-dom does not always turn a submit-button click into a submit event, so submit the form directly.
const save = async () => {
  const button = screen.getByRole("button", {
    name: "Сохранить и пересчитать",
  });
  await waitFor(() => expect(button).toHaveProperty("disabled", false));
  fireEvent.submit(button.closest("form")!);
};

const routes = (jobStatus: "succeeded" | "failed") => ({
  "GET /api/v1/model/settings": () => [200, settings] as [number, unknown],
  "GET /api/v1/model/features": () => [200, features] as [number, unknown],
  "PATCH /api/v1/model/settings": () =>
    [202, accepted("job-1")] as [number, unknown],
  "GET /api/v1/jobs/job-1": () =>
    [
      200,
      {
        jobId: "job-1",
        kind: "model_settings",
        status: jobStatus,
        createdAt: "2026-10-24T12:00:00Z",
        startedAt: "2026-10-24T12:00:01Z",
        finishedAt: "2026-10-24T12:00:05Z",
        attempt: 1,
        ...(jobStatus === "succeeded"
          ? { result: settings }
          : {
              error: {
                code: "CALCULATION_FAILED",
                message: "Расчёт прерван",
                requestId: "r",
              },
            }),
      },
    ] as [number, unknown],
});

describe("ModelPage", () => {
  it("sends only changed fields with version guard and idempotency key, then follows the job", async () => {
    const calls = stubApi(routes("succeeded"));
    renderRoute(<ModelPage />, "/model");
    await ready();

    fireEvent.change(screen.getByLabelText("Горизонт оценки потерь, дней"), {
      target: { value: "45" },
    });
    await save();

    expect(await screen.findByText("Новая версия опубликована")).toBeTruthy();
    const patch = calls.find((c) => c.method === "PATCH")!;
    expect(patch.body).toEqual({ expectedVersion: 3, horizonDays: 45 });
    expect(patch.headers.get("Idempotency-Key")).toBeTruthy();
  });

  it("disabling a feature sends a zero coefficient", async () => {
    const calls = stubApi(routes("succeeded"));
    renderRoute(<ModelPage />, "/model");
    await ready();

    fireEvent.click(
      screen.getByRole("switch", { name: "Признак «Автопродление выключено»" }),
    );
    await save();

    await waitFor(() =>
      expect(calls.some((c) => c.method === "PATCH")).toBe(true),
    );
    expect(calls.find((c) => c.method === "PATCH")!.body).toEqual({
      expectedVersion: 3,
      coefficients: { renewalWithoutAutoProlong: 0 },
    });
  });

  it("stops the medium threshold before the high one", async () => {
    stubApi(routes("succeeded"));
    renderRoute(<ModelPage />, "/model");
    await ready();

    fireEvent.change(screen.getByLabelText("Средний"), {
      target: { value: "80" },
    });

    expect(await screen.findByText("69 – 70")).toBeTruthy();
    expect(screen.getByText("0 – 69")).toBeTruthy();
  });

  it("cancel restores the published settings", async () => {
    stubApi(routes("succeeded"));
    renderRoute(<ModelPage />, "/model");
    await ready();

    fireEvent.change(screen.getByLabelText("Горизонт оценки потерь, дней"), {
      target: { value: "60" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Отменить изменения" }));

    expect(await screen.findByDisplayValue("30")).toBeTruthy();
  });

  it("shows why a background recalculation failed", async () => {
    stubApi(routes("failed"));
    renderRoute(<ModelPage />, "/model");
    await ready();

    fireEvent.change(screen.getByLabelText("Горизонт оценки потерь, дней"), {
      target: { value: "60" },
    });
    await save();

    expect(
      await screen.findByText("Пересчёт не выполнен, прежние данные сохранены"),
    ).toBeTruthy();
    expect(screen.getByText("Расчёт прерван")).toBeTruthy();
  });
});
