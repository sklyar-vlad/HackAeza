import type { AuthConfigOidc } from "@/shared/api/generated/api";

const STATE_KEY = "aeza-oauth-state";

export function startOidc(oidc: AuthConfigOidc) {
  const state = crypto.randomUUID();
  sessionStorage.setItem(STATE_KEY, state);
  const url = new URL(oidc.authorizationEndpoint ?? "");
  url.search = new URLSearchParams({
    client_id: oidc.clientId ?? "",
    response_type: oidc.responseType ?? "token",
    redirect_uri: oidc.redirectUri ?? "",
    scope: (oidc.scopes ?? []).join(" "),
    state,
  }).toString();
  window.location.assign(url.toString());
}

export function readCallback(location: Location) {
  const params = new URLSearchParams(location.search);
  new URLSearchParams(location.hash.slice(1)).forEach((value, key) =>
    params.set(key, value),
  );
  const expected = sessionStorage.getItem(STATE_KEY);
  sessionStorage.removeItem(STATE_KEY);
  if (params.get("error"))
    return {
      error: params.get("error_description") ?? "Вход в aéza отменён",
    };
  if (!expected || params.get("state") !== expected)
    return {
      error: "Ответ aéza не совпал с начатым входом. Попробуйте ещё раз.",
    };
  const accessToken = params.get("access_token");
  const code = params.get("code");
  if (accessToken) return { body: { accessToken } };
  if (code) return { body: { code } };
  return { error: "aéza не вернула данные для входа" };
}
