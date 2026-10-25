import { usePrefs } from "@/shared/model/session";
import styles from "./CookieBanner.module.css";

export function CookieBanner() {
  const { cookiesAccepted, acceptCookies } = usePrefs();
  if (cookiesAccepted) return null;

  return (
    <div
      role="dialog"
      aria-label="Использование cookies"
      className={styles.banner}
    >
      <p>
        Мы обрабатываем локальные данные браузера и используем инструменты
        аналитики в целях улучшения и обеспечения работоспособности сайта,
        статистических исследований и обзоров. Вы можете запретить обработку
        указанных данных в настройках браузера. Пожалуйста, ознакомьтесь с{" "}
        <a
          className="link"
          href="https://aeza.ru/privacy"
          target="_blank"
          rel="noopener"
        >
          условиями их обработки
        </a>
        .
      </p>
      <button type="button" className="btn" onClick={acceptCookies}>
        Принять
      </button>
    </div>
  );
}
