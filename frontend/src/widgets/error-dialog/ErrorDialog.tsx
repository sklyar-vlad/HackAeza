import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { create } from "zustand";
import { ApiError } from "@/shared/api/http";
import { Icon } from "@/shared/ui/Icon";
import { RequestId } from "@/shared/ui/RequestId";
import styles from "./ErrorDialog.module.css";

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: { inlineError?: boolean };
    mutationMeta: { inlineError?: boolean };
  }
}

type Action = "close" | "retry" | "login" | "home" | "customers";

type Content = {
  code: string;
  title: string;
  text: string;
  primary: [string, Action];
  secondary?: [string, Action];
};

export const useErrorDialog = create<{
  error: unknown;
  show: (error: unknown) => void;
  close: () => void;
}>((set, get) => ({
  error: null,
  show: (error) => get().error === null && set({ error }),
  close: () => set({ error: null }),
}));

// Pages that render their own errors opt out with meta.inlineError; an expired session is always shown.
export const reportError = (
  error: unknown,
  meta?: { inlineError?: boolean },
) => {
  const expired = error instanceof ApiError && error.sessionExpired;
  if (meta?.inlineError && !expired) return;
  if (error instanceof ApiError || error instanceof TypeError)
    useErrorDialog.getState().show(error);
};

const conflicts: Record<string, Content> = {
  VERSION_CONFLICT: {
    code: "409",
    title: "данные уже изменились",
    text: "Пока вы работали, кто-то сохранил новую версию. Загрузите свежие данные и внесите правки заново.",
    primary: ["Загрузить свежие", "retry"],
    secondary: ["Отмена", "close"],
  },
  JOB_IN_PROGRESS: {
    code: "409",
    title: "пересчёт уже идёт",
    text: "Сейчас выполняется другое задание. Новое можно запустить, когда оно завершится.",
    primary: ["Понятно", "close"],
  },
  IDEMPOTENCY_CONFLICT: {
    code: "409",
    title: "запрос уже отправлен",
    text: "Этот запрос уже принят с другими данными. Обновите страницу и отправьте изменения ещё раз.",
    primary: ["Обновить данные", "retry"],
    secondary: ["Закрыть", "close"],
  },
};

const byStatus: Record<number, Content> = {
  400: {
    code: "400",
    title: "некорректный запрос",
    text: "Сервер не принял параметры запроса. Исправьте их и повторите.",
    primary: ["Понятно", "close"],
  },
  401: {
    code: "401",
    title: "сессия истекла",
    text: "Токен входа больше не действует. Войдите снова, чтобы продолжить работу.",
    primary: ["Войти снова", "login"],
  },
  403: {
    code: "403",
    title: "нет доступа",
    text: "У вашей учётной записи нет прав на это действие. Обратитесь к администратору панели.",
    primary: ["Понятно", "close"],
    secondary: ["На главную", "home"],
  },
  404: {
    code: "404",
    title: "не найдено",
    text: "Запрошенных данных нет в текущей версии. Возможно, они пропали из последнего снимка.",
    primary: ["К рейтингу клиентов", "customers"],
    secondary: ["Закрыть", "close"],
  },
  409: {
    code: "409",
    title: "конфликт изменений",
    text: "Запрос конфликтует с текущим состоянием данных. Обновите страницу и попробуйте снова.",
    primary: ["Обновить данные", "retry"],
    secondary: ["Закрыть", "close"],
  },
  413: {
    code: "413",
    title: "файл слишком большой",
    text: "Запрос больше, чем допускает сервер. Разбейте выгрузку или сожмите файл и загрузите снова.",
    primary: ["Понятно", "close"],
  },
  422: {
    code: "422",
    title: "данные не прошли проверку",
    text: "Ничего не изменилось, текущая версия осталась прежней. Исправьте записи:",
    primary: ["Понятно", "close"],
  },
  429: {
    code: "429",
    title: "слишком много запросов",
    text: "Сервер временно ограничил запросы. Подождите немного и повторите.",
    primary: ["Понятно", "close"],
  },
  500: {
    code: "500",
    title: "что-то сломалось",
    text: "Ошибка на нашей стороне, данные не пострадали. Если повторится, передайте команде код запроса.",
    primary: ["Повторить", "retry"],
    secondary: ["Закрыть", "close"],
  },
  503: {
    code: "503",
    title: "сервис недоступен",
    text: "Сервер временно не отвечает. Повторите запрос через минуту.",
    primary: ["Повторить", "retry"],
    secondary: ["Закрыть", "close"],
  },
};

const offline: Content = {
  code: "сеть",
  title: "нет соединения",
  text: "Браузер не может достучаться до сервера. Проверьте интернет и повторите запрос.",
  primary: ["Повторить", "retry"],
};

function describe(error: ApiError): Content {
  const known =
    conflicts[error.body?.error.code ?? ""] ?? byStatus[error.status];
  if (known) return known;
  return {
    ...byStatus[error.status >= 500 ? 500 : 400],
    code: String(error.status),
    text: error.message,
  };
}

function useCountdown(seconds?: number) {
  const [left, setLeft] = useState(seconds ?? 0);
  useEffect(() => {
    setLeft(seconds ?? 0);
    if (!seconds) return;
    const timer = setInterval(() => setLeft((n) => Math.max(0, n - 1)), 1000);
    return () => clearInterval(timer);
  }, [seconds]);
  return left;
}

export function ErrorDialog() {
  const { error, close } = useErrorDialog();
  const ref = useRef<HTMLDialogElement>(null);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const api = error instanceof ApiError ? error : null;
  const wait = useCountdown(api?.retryAfter);

  useEffect(() => {
    if (error) ref.current?.showModal();
    else ref.current?.close();
  }, [error]);

  if (!error) return <dialog ref={ref} className={styles.dialog} />;

  const content = api ? describe(api) : offline;
  const details = api?.body?.error.details ?? [];
  const requestId = api?.body?.error.requestId;
  const run = (action: Action) => {
    close();
    if (action === "retry") queryClient.refetchQueries({ type: "active" });
    if (action === "login") navigate("/login");
    if (action === "home") navigate("/");
    if (action === "customers") navigate("/customers");
  };

  return (
    <dialog
      ref={ref}
      className={styles.dialog}
      aria-labelledby="error-dialog-title"
      onClose={close}
    >
      <div className={styles.frame}>
        <img className={styles.mascot} src="/mascot.webp" alt="" />
        <section className={styles.card}>
          <div className={styles.head}>
            <span className={styles.chip}>{content.code}</span>
            <button
              type="button"
              className="btn btn-ghost"
              aria-label="Закрыть"
              onClick={close}
            >
              <Icon name="x" />
            </button>
          </div>
          <div>
            <h2 id="error-dialog-title" className={styles.title}>
              {content.title}
            </h2>
            <p className={styles.text}>
              {content.text}
              {wait > 0 && ` Повторить можно через ${wait} с.`}
            </p>
          </div>
          {details.length > 0 && (
            <ul className={styles.details}>
              {details.slice(0, 5).map((d) => (
                <li key={d.path}>
                  <code>{d.path}</code>
                  <span>{d.message}</span>
                </li>
              ))}
            </ul>
          )}
          <div className={styles.actions}>
            <button
              type="button"
              className="btn"
              onClick={() => run(content.primary[1])}
            >
              {content.primary[0]}
            </button>
            {content.secondary && (
              <button
                type="button"
                className={`btn ${styles.secondary}`}
                onClick={() => run(content.secondary![1])}
              >
                {content.secondary[0]}
              </button>
            )}
          </div>
          <p className={styles.request}>
            {requestId ? (
              <RequestId id={requestId} />
            ) : (
              "Ответа от сервера нет, кода запроса нет."
            )}
          </p>
        </section>
      </div>
    </dialog>
  );
}
