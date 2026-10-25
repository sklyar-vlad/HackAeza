const number = new Intl.NumberFormat("ru-RU");
const decimal = new Intl.NumberFormat("ru-RU", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});
const dateTime = new Intl.DateTimeFormat("ru-RU", {
  day: "numeric",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});
const dayYear = new Intl.DateTimeFormat("ru-RU", {
  day: "numeric",
  month: "short",
  year: "numeric",
});
const clock = new Intl.DateTimeFormat("ru-RU", {
  hour: "2-digit",
  minute: "2-digit",
});
const day = new Intl.DateTimeFormat("ru-RU", {
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});

/** Contract money is integer euro cents; null means "no estimate", never zero. */
export const money = (cents: number | null | undefined) =>
  cents == null ? "—" : `€ ${decimal.format(cents / 100)}`;

export const int = (n: number) => number.format(n);

export const bytes = (n: number) =>
  n < 1024
    ? `${n} Б`
    : n < 1_048_576
      ? `${(n / 1024).toFixed(1)} КБ`
      : `${(n / 1_048_576).toFixed(1)} МБ`;

export const score = (n: number | null | undefined) =>
  n == null ? "—" : String(Math.round(n));

export const percent = (part: number, total: number) =>
  total ? `${Math.round((part / total) * 100)}%` : "0%";

export const date = (iso: string | null | undefined) =>
  iso ? dateTime.format(new Date(iso)) : "—";

export const dateOnly = (iso: string | null | undefined) =>
  iso ? dayYear.format(new Date(iso)) : "—";

export const time = (iso: string | null | undefined) =>
  iso ? clock.format(new Date(iso)) : "—";

export const shortDay = (isoDate: string) => day.format(new Date(isoDate));

/** YYYY-MM-DD in UTC, as the churn endpoint expects. */
export const isoDay = (d: Date) => d.toISOString().slice(0, 10);
