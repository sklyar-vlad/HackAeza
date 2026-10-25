import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderRoute, stubApi } from "@/shared/lib/testing";
import { useSession } from "@/shared/model/session";
import { LoginPage } from "./LoginPage";

const fill = (username: string, password: string) => {
  fireEvent.change(screen.getByLabelText("Логин"), {
    target: { value: username },
  });
  fireEvent.change(screen.getByLabelText("Пароль"), {
    target: { value: password },
  });
  fireEvent.click(screen.getByRole("button", { name: "войти" }));
};

describe("LoginPage", () => {
  it("logs in with username and password and opens the panel", async () => {
    const calls = stubApi({
      "POST /api/v1/auth/login": () => [
        200,
        {
          accessToken: "session-1",
          tokenType: "Bearer",
          expiresAt: "2026-10-25T12:00:00Z",
          user: {
            id: "analyst",
            username: "analyst",
            authProvider: "password",
          },
        },
      ],
    });
    renderRoute(<LoginPage />, "/login");

    fill("analyst", "secret");

    expect(await screen.findByText("главная")).toBeTruthy();
    expect(calls.find((c) => c.method === "POST")?.body).toEqual({
      username: "analyst",
      password: "secret",
    });
    expect(useSession.getState()).toMatchObject({
      token: "session-1",
      username: "analyst",
    });
  });

  it("explains a rejected password", async () => {
    stubApi({
      "POST /api/v1/auth/login": () => [
        401,
        { error: { code: "UNAUTHORIZED", message: "x", requestId: "r" } },
      ],
    });
    renderRoute(<LoginPage />, "/login");

    fill("analyst", "wrong");

    expect(await screen.findByText("Неверный логин или пароль")).toBeTruthy();
    expect(useSession.getState().token).toBeNull();
  });

  it("does not call the API with empty fields", async () => {
    const calls = stubApi({});
    renderRoute(<LoginPage />, "/login");

    fill("", "");

    expect(await screen.findByText("Введите логин и пароль")).toBeTruthy();
    expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);
  });

  it("offers sign-in with an aéza account when it is configured", async () => {
    stubApi({
      "GET /api/v1/auth/config": () => [
        200,
        {
          passwordEnabled: true,
          oidc: {
            enabled: true,
            issuer: "https://my.aeza.net",
            clientId: "risk-panel",
            redirectUri: "https://panel.test/login/callback",
            scopes: ["user:read"],
            authorizationEndpoint: "https://my.aeza.net/oauth/authorize",
            responseType: "token",
            provider: "aeza",
          },
        },
      ],
    });
    renderRoute(<LoginPage />, "/login");

    expect(
      await screen.findByRole("button", { name: "войти через аккаунт aéza" }),
    ).toBeTruthy();
  });
});
