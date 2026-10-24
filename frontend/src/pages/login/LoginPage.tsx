import { zodResolver } from "@hookform/resolvers/zod";
import { motion } from "motion/react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Navigate, useNavigate } from "react-router";
import type { z } from "zod";
import { useLogin } from "@/shared/api/generated/api";
import { LoginBody } from "@/shared/api/generated/zod";
import { ApiError } from "@/shared/api/http";
import { usePrefs, useSession } from "@/shared/model/session";
import { Icon } from "@/shared/ui/Icon";
import { Logo, Mark } from "@/shared/ui/Logo";
import styles from "./LoginPage.module.css";

type Form = z.infer<typeof LoginBody>;

const errorText = (error: unknown) => {
  if (!(error instanceof ApiError))
    return "Сервер недоступен, попробуйте позже";
  if (error.status === 401) return "Неверный логин или пароль";
  if (error.status === 429) return "Слишком много попыток, подождите немного";
  return error.message;
};

export function LoginPage() {
  const { token, signIn } = useSession();
  const { theme, toggleTheme } = usePrefs();
  const navigate = useNavigate();
  const [showPassword, setShowPassword] = useState(false);
  const login = useLogin();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<Form>({ resolver: zodResolver(LoginBody) });

  if (token) return <Navigate to="/" replace />;

  const submit = handleSubmit((data) =>
    login.mutate(
      { data },
      {
        onSuccess: (res) => {
          signIn(res.accessToken, res.user.username);
          navigate("/", { replace: true });
        },
      },
    ),
  );

  return (
    <div className={styles.page}>
      <div className={styles.phoneLogo}>
        <Logo size={32} />
      </div>
      <motion.div
        className={styles.cards}
        initial={{ opacity: 0, y: 100 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 1, ease: [0.65, 0, 0.35, 1] }}
      >
        <section className={styles.card}>
          <div>
            <h1 className={styles.title}>вход в риск-панель</h1>
            <p className={styles.sub}>логин и пароль сотрудника aéza</p>
          </div>
          <div className={styles.footer}>
            <span className={styles.mark}>
              <Mark />
            </span>
            <button
              type="button"
              className="btn btn-icon"
              onClick={toggleTheme}
              aria-label="Сменить тему"
            >
              <Icon name={theme === "dark" ? "sun" : "moon"} size={18} />
            </button>
          </div>
        </section>

        <section className={styles.card} aria-label="Форма входа">
          <form className={styles.form} onSubmit={submit} noValidate>
            <label className="sr-only" htmlFor="username">
              Логин
            </label>
            <input
              id="username"
              className={`field ${styles.input}`}
              placeholder="логин"
              autoComplete="username"
              aria-invalid={Boolean(errors.username)}
              {...register("username")}
            />
            <label className="sr-only" htmlFor="password">
              Пароль
            </label>
            <div className={styles.password}>
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                className={`field ${styles.input}`}
                placeholder="пароль"
                autoComplete="current-password"
                aria-invalid={Boolean(errors.password)}
                {...register("password")}
              />
              <button
                type="button"
                className={styles.eye}
                onClick={() => setShowPassword(!showPassword)}
                aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}
              >
                <Icon name={showPassword ? "eyeOff" : "eye"} />
              </button>
            </div>
            {(errors.username || errors.password) && (
              <p className="error-text">Введите логин и пароль</p>
            )}
            {login.isError && (
              <p className="error-text">{errorText(login.error)}</p>
            )}
            <button
              type="submit"
              className={`btn ${styles.submit}`}
              disabled={login.isPending}
            >
              {login.isPending ? "входим…" : "войти"}
            </button>
          </form>

          <div className={styles.links}>
            <a href="/api/v1/docs" target="_blank" rel="noopener">
              документация api
              <Icon name="external" />
            </a>
          </div>
        </section>
      </motion.div>
    </div>
  );
}
