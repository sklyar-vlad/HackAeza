import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderRoute, stubApi } from "@/shared/lib/testing";
import { useSession } from "@/shared/model/session";
import { AppLayout } from "./AppLayout";

describe("AppLayout logout", () => {
  it("clears the local session after the server confirms revocation", async () => {
    useSession.getState().signIn("valid", "analyst");
    stubApi({ "POST /api/v1/auth/logout": () => [204] });
    renderRoute(<AppLayout />, "/customers");
    fireEvent.click(screen.getByRole("button", { name: "Выйти из аккаунта" }));
    await waitFor(() => expect(useSession.getState().token).toBeNull());
  });

  it("keeps the session and explains a failed server logout", async () => {
    useSession.getState().signIn("valid", "analyst");
    stubApi({
      "POST /api/v1/auth/logout": () => [
        503,
        {
          error: {
            code: "UNAVAILABLE",
            message: "unavailable",
            requestId: "r",
          },
        },
      ],
    });
    renderRoute(<AppLayout />, "/customers");
    fireEvent.click(screen.getByRole("button", { name: "Выйти из аккаунта" }));
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(useSession.getState().token).toBe("valid");
  });
});
