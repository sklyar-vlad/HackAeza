import type { ReactNode } from "react";
import { Link, useRouteError } from "react-router";
import { ApiError } from "@/shared/api/http";
import { Logo } from "@/shared/ui/Logo";
import { RequestId } from "@/shared/ui/RequestId";
import styles from "./ErrorPage.module.css";

function ErrorScreen(props: {
  code: number;
  title: string;
  text: string;
  note: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.page}>
      <main className={styles.sheet}>
        <section className={styles.content}>
          <Link to="/" aria-label="aéza, на главную">
            <Logo size={32} />
          </Link>
          <div className={styles.body}>
            <span className={styles.code} aria-hidden="true">
              {props.code}
            </span>
            <div>
              <h1 className={styles.title}>{props.title}</h1>
              <p className={styles.text}>{props.text}</p>
            </div>
            {props.children}
          </div>
          <span className={styles.foot}>{props.note}</span>
        </section>
        <div className={styles.art}>
          <img src="/mascot.webp" alt="" />
        </div>
      </main>
    </div>
  );
}

export function NotFoundPage() {
  return (
    <ErrorScreen
      code={404}
      title="страница не найдена"
      text="Такой страницы в панели нет. Возможно, ссылка устарела или клиент пропал из последнего снимка данных."
      note="Ошибка 404 · запрошенный адрес не существует"
    >
      <div className={styles.actions}>
        <Link className="btn" to="/">
          На главную
        </Link>
        <Link className={`btn ${styles.secondary}`} to="/customers">
          К рейтингу клиентов
        </Link>
      </div>
    </ErrorScreen>
  );
}

export function ServerErrorPage() {
  const error = useRouteError();
  const requestId =
    error instanceof ApiError ? error.body?.error.requestId : undefined;
  return (
    <ErrorScreen
      code={500}
      title="что-то сломалось"
      text="Ошибка на нашей стороне, ваши данные не пострадали. Обновите страницу через минуту. Если ошибка повторится, передайте команде код запроса."
      note="Ошибка 500 · внутренняя ошибка приложения"
    >
      {requestId && (
        <div className={styles.request}>
          <RequestId id={requestId} />
        </div>
      )}
      <div className={styles.actions}>
        <button
          type="button"
          className="btn"
          onClick={() => window.location.reload()}
        >
          Обновить страницу
        </button>
        <a className={`btn ${styles.secondary}`} href="/">
          На главную
        </a>
      </div>
    </ErrorScreen>
  );
}
