import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { calculation, renderRoute, stubApi } from "@/shared/lib/testing";
import { CustomersPage } from "./CustomersPage";

const customer = {
  id: 101,
  email: "risky@example.com",
  balance: 0,
  withdrawBalance: 0,
  bonusBalance: 0,
  bonusState: "locked",
  totalReplenished: 2490,
  servicesCount: 1,
  risk: {
    status: "assessed",
    score: 82.4,
    level: "high",
    reasons: [
      {
        code: "INSUFFICIENT_RENEWAL_FUNDS",
        message: "На ближайшее продление не хватает 2490 евроцентов.",
        featureKey: "insufficientRenewalFunds",
        serviceIds: [201],
      },
    ],
  },
  loss: {
    amountCents: 2052,
    currency: "EUR",
    horizonDays: 30,
    renewalRevenueCents: 2490,
    unassessedRevenueCents: 0,
    isPartial: false,
    explanation: "",
  },
};

const customersUrls = (calls: { url: string }[]) =>
  calls
    .filter((c) => c.url.startsWith("/api/v1/customers"))
    .map((c) => new URL(c.url, "http://x").searchParams);

describe("CustomersPage", () => {
  it("passes the risk filter from the URL to the API and renders the rating", async () => {
    const calls = stubApi({
      "GET /api/v1/customers": () => [
        200,
        {
          items: [customer],
          pagination: { page: 1, pageSize: 20, total: 1 },
          calculation,
        },
      ],
    });
    renderRoute(<CustomersPage />, "/customers", "/customers?risk=high");

    expect(await screen.findByText("risky@example.com")).toBeTruthy();
    expect(screen.getByText("€ 20,52")).toBeTruthy();
    expect(screen.getByText("82")).toBeTruthy();
    const params = customersUrls(calls)[0];
    expect(params.get("riskLevel")).toBe("high");
    expect(params.get("sortBy")).toBe("score");
    expect(params.get("sortOrder")).toBe("desc");
  });

  it("re-sorts on the server when a column header is clicked", async () => {
    const calls = stubApi({
      "GET /api/v1/customers": () => [
        200,
        {
          items: [customer],
          pagination: { page: 1, pageSize: 20, total: 1 },
          calculation,
        },
      ],
    });
    renderRoute(<CustomersPage />, "/customers");

    fireEvent.click(await screen.findByRole("button", { name: "Баланс" }));

    await waitFor(() =>
      expect(
        customersUrls(calls).some((p) => p.get("sortBy") === "balance"),
      ).toBe(true),
    );
    expect(customersUrls(calls).at(-1)!.get("sortOrder")).toBe("desc");
  });
});
