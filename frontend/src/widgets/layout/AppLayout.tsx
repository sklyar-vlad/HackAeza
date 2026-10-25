import { useQueryClient } from "@tanstack/react-query";
import { Link, NavLink, Outlet } from "react-router";
import { useLogout } from "@/shared/api/generated/api";
import { usePrefs, useSession } from "@/shared/model/session";
import { Icon, type IconName } from "@/shared/ui/Icon";
import { Logo } from "@/shared/ui/Logo";
import styles from "./AppLayout.module.css";

const NAV: { to: string; label: string; icon: IconName }[] = [
  { to: "/", label: "Обзор", icon: "home" },
  { to: "/customers", label: "Клиенты", icon: "users" },
  { to: "/model", label: "Настройки модели", icon: "trend" },
  { to: "/source", label: "Источник данных", icon: "doc" },
];

export function AppLayout() {
  const { username, signOut } = useSession();
  const { theme, toggleTheme } = usePrefs();
  const queryClient = useQueryClient();
  const logout = useLogout({ mutation: { meta: { inlineError: true } } });

  const exit = () =>
    logout.mutate(undefined, {
      onSuccess: () => {
        signOut();
        queryClient.clear();
      },
    });

  return (
    <div className={styles.shell}>
      <aside className={styles.side}>
        <div className={styles.group}>
          <Link to="/" className={styles.brand} aria-label="На главную">
            <Logo size={30} />
          </Link>
          <nav className={styles.nav} aria-label="Основная навигация">
            {NAV.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={styles.item}
                title={item.label}
              >
                <span className={styles.label}>{item.label}</span>
                <Icon name={item.icon} />
              </NavLink>
            ))}
          </nav>
        </div>

        <div className={styles.group}>
          <a
            className={styles.item}
            href="/api/v1/docs"
            target="_blank"
            rel="noopener"
            title="База знаний: Swagger"
          >
            <span className={styles.label}>База знаний</span>
            <Icon name="bookmark" />
          </a>
          <button
            type="button"
            className={styles.item}
            onClick={toggleTheme}
            title="Сменить тему"
          >
            <span className={styles.label}>
              {theme === "dark" ? "Светлая тема" : "Тёмная тема"}
            </span>
            <Icon name={theme === "dark" ? "sun" : "moon"} />
          </button>
          <div className={styles.account}>
            <span className={styles.label} title={username ?? undefined}>
              {username}
            </span>
            <button
              type="button"
              className="btn btn-icon"
              onClick={exit}
              disabled={logout.isPending}
              aria-label="Выйти из аккаунта"
              title="Выйти"
            >
              <Icon name="logout" size={18} />
            </button>
          </div>
        </div>
      </aside>

      <main className={styles.sheet}>
        {logout.isError && (
          <p role="alert" className="error-text">
            Не удалось завершить сессию на сервере. Попробуйте выйти ещё раз.
          </p>
        )}
        <Outlet />
      </main>
    </div>
  );
}
