import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router";
import { completeOidcLogin } from "@/shared/api/generated/api";
import { ApiError } from "@/shared/api/http";
import { useSession } from "@/shared/model/session";
import { Logo } from "@/shared/ui/Logo";
import { readCallback } from "./oidc";
import styles from "./LoginPage.module.css";

const failure = (error: unknown) => {
  if (!(error instanceof ApiError))
    return "Сервер недоступен, попробуйте позже";
  if (error.status === 403)
    return "Этому аккаунту aéza не открыт доступ к панели";
  if (error.status === 401)
    return "aéza не подтвердила вход, попробуйте ещё раз";
  return error.message;
};

export function OidcCallbackPage() {
  const signIn = useSession((s) => s.signIn);
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const result = readCallback(window.location);
    if (!result.body) return setError(result.error);
    let active = true;
    completeOidcLogin(result.body).then(
      (res) => {
        if (!active) return;
        signIn(res.accessToken, res.user.username);
        navigate("/", { replace: true });
      },
      (e: unknown) => active && setError(failure(e)),
    );
    return () => {
      active = false;
    };
  }, [navigate, signIn]);

  return (
    <div className={styles.page}>
      <section className={styles.card} aria-live="polite">
        <div>
          <h1 className={styles.title}>
            {error ? "вход не выполнен" : "входим через aéza"}
          </h1>
          <p className={styles.sub}>
            {error ?? "Проверяем аккаунт и открываем панель…"}
          </p>
        </div>
        <div className={styles.footer}>
          <Logo size={28} />
          {error && (
            <Link className="btn" to="/login" replace>
              Вернуться ко входу
            </Link>
          )}
        </div>
      </section>
    </div>
  );
}
