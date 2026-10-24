#!/usr/bin/env python3
"""Check real password login, protected requests and session revocation. No tokens are printed."""

import argparse
import getpass
import json
import sys
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--username", required=True)
    parser.add_argument("--base-url", default="http://localhost:3000")
    parser.add_argument("--password-stdin", action="store_true")
    args = parser.parse_args()
    password = sys.stdin.readline().rstrip("\r\n") if args.password_stdin else getpass.getpass("Пароль: ")
    origin = args.base_url.rstrip("/")
    if not password:
        raise RuntimeError("Пароль не указан")

    def call(method, path, status, body=None, token=None):
        headers = {"Accept": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        if body is not None:
            headers["Content-Type"] = "application/json"
        request = urllib.request.Request(
            origin + path,
            data=json.dumps(body).encode() if body is not None else None,
            headers=headers,
            method=method,
        )
        try:
            response = urllib.request.urlopen(request, timeout=15)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            if response.status != status:
                raise RuntimeError(f"{method} {path}: ожидался {status}, получен {response.status}")
            result = json.loads(raw) if raw else None
            if path.startswith("/api/v1/auth/") and response.headers.get("Cache-Control") != "no-store":
                raise RuntimeError("Ответ авторизации может кешироваться")
            if status == 401 and not result.get("error", {}).get("requestId"):
                raise RuntimeError("В ошибке отсутствует requestId")
            print(f"OK {method} {path} -> {status}")
            return result

    config = call("GET", "/api/v1/auth/config", 200)
    if not config["passwordEnabled"] or config["oidc"]["enabled"]:
        raise RuntimeError("Некорректная конфигурация способов входа")
    call("GET", "/api/v1/auth/me", 401)
    call("GET", "/api/v1/customers", 401)
    call("POST", "/api/v1/auth/login", 400, {})
    call("POST", "/api/v1/auth/login", 401, {"username": args.username, "password": "wrong-" + password})
    sessions = []
    try:
        for _ in range(2):
            session = call("POST", "/api/v1/auth/login", 200, {"username": args.username, "password": password})
            sessions.append(session["accessToken"])
            if session["tokenType"] != "Bearer" or not session["expiresAt"] or session["user"]["username"] != args.username:
                raise RuntimeError("Ответ входа не соответствует контракту")
        if sessions[0] == sessions[1]:
            raise RuntimeError("Повторный вход переиспользовал токен")
        user = call("GET", "/api/v1/auth/me", 200, token=sessions[0])
        if user["username"] != args.username:
            raise RuntimeError("Получен другой пользователь")
        call("POST", "/api/v1/auth/logout", 204, token=sessions[0])
        call("GET", "/api/v1/auth/me", 401, token=sessions[0])
        call("GET", "/api/v1/auth/me", 200, token=sessions[1])
        call("POST", "/api/v1/auth/logout", 204, token=sessions[1])
        call("GET", "/api/v1/auth/me", 401, token=sessions[1])
    finally:
        # Best-effort cleanup if a check fails after creating a session.
        for token in sessions:
            request = urllib.request.Request(origin + "/api/v1/auth/logout", method="POST", headers={"Authorization": "Bearer " + token})
            try:
                urllib.request.urlopen(request, timeout=5).close()
            except OSError:
                pass
    print("Авторизация работает: вход, Bearer-запросы и отзыв сессий проверены.")


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, ValueError, KeyError) as error:
        print(f"FAIL: {error}", file=sys.stderr)
        sys.exit(1)
