import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { stubApi } from "@/shared/lib/testing";
import { useSession } from "@/shared/model/session";
import { Protected } from "./Protected";

function renderGuard() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={["/private"]}>
        <Routes>
          <Route
            path="/private"
            element={
              <Protected>
                <p>Панель</p>
              </Protected>
            }
          />
          <Route path="/login" element={<p>Форма входа</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("Protected", () => {
  it("verifies the saved token before showing the panel", async () => {
    useSession.getState().signIn("valid", "analyst");
    const calls = stubApi({
      "GET /api/v1/auth/me": () => [
        200,
        { id: "u1", username: "analyst", authProvider: "password" },
      ],
    });
    renderGuard();
    expect(await screen.findByText("Панель")).toBeTruthy();
    expect(calls[0].headers.get("Authorization")).toBe("Bearer valid");
  });

  it("hides protected content when the saved session is rejected", async () => {
    useSession.getState().signIn("revoked", "analyst");
    stubApi({
      "GET /api/v1/auth/me": () => [
        401,
        { error: { code: "UNAUTHORIZED", message: "revoked", requestId: "r" } },
      ],
    });
    renderGuard();
    expect(await screen.findByText("Форма входа")).toBeTruthy();
    expect(screen.queryByText("Панель")).toBeNull();
    expect(useSession.getState().token).toBeNull();
  });

  it("keeps a session on temporary server failures and offers a retry", async () => {
    useSession.getState().signIn("valid", "analyst");
    stubApi({
      "GET /api/v1/auth/me": () => [
        503,
        {
          error: {
            code: "UNAVAILABLE",
            message: "db unavailable",
            requestId: "r",
          },
        },
      ],
    });
    renderGuard();
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.queryByText("Панель")).toBeNull();
    expect(useSession.getState().token).toBe("valid");
  });
});
