import { describe, expect, it } from "vitest";
import { stubApi } from "@/shared/lib/testing";
import { useSession } from "@/shared/model/session";
import { ApiError, http } from "./http";

describe("http", () => {
  it("sends the session as a Bearer token", async () => {
    useSession.setState({ token: "t-1", username: "analyst" });
    const calls = stubApi({
      "GET /api/v1/auth/me": () => [200, { id: "analyst" }],
    });

    await expect(http("/api/v1/auth/me")).resolves.toEqual({ id: "analyst" });
    expect(calls[0].headers.get("Authorization")).toBe("Bearer t-1");
  });

  it("drops the session when the token is rejected", async () => {
    useSession.setState({ token: "expired", username: "analyst" });
    stubApi({
      "GET /api/v1/auth/me": () => [
        401,
        {
          error: {
            code: "UNAUTHORIZED",
            message: "Токен истёк",
            requestId: "r1",
          },
        },
      ],
    });

    await expect(http("/api/v1/auth/me")).rejects.toMatchObject({
      status: 401,
      message: "Токен истёк",
    });
    expect(useSession.getState().token).toBeNull();
  });

  it("exposes contract validation details", async () => {
    const details = [
      {
        path: "services[0].ownerId",
        message: "Аккаунт с таким ID отсутствует в снимке.",
      },
    ];
    stubApi({
      "PUT /api/v1/source/snapshot": () => [
        422,
        {
          error: {
            code: "VALIDATION_ERROR",
            message: "Снимок не прошёл проверку",
            requestId: "r2",
            details,
          },
        },
      ],
    });

    const error = await http("/api/v1/source/snapshot", {
      method: "PUT",
    }).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).body?.error.details).toEqual(details);
  });

  it("treats 204 as an empty success", async () => {
    stubApi({ "POST /api/v1/auth/logout": () => [204] });
    await expect(
      http("/api/v1/auth/logout", { method: "POST" }),
    ).resolves.toBeNull();
  });
});
