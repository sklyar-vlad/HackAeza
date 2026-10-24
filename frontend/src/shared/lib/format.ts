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
const day = new Intl.DateTimeFormat("ru-RU", {
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});

/** Contract money is integer euro cents; null means "no estimate", never zero. */
export const money = (cents: number | null | undefined) =>
  cents == null ? "—" : `€ ${decimal.format(cents / 100)}`;

export const int = (n: number) => number.format(n);

export const score = (n: number | null | undefined) =>
  n == null ? "—" : String(Math.round(n));

export const percent = (part: number, total: number) =>
  total ? `${Math.round((part / total) * 100)}%` : "0%";

export const date = (iso: string | null | undefined) =>
  iso ? dateTime.format(new Date(iso)) : "—";

export const shortDay = (isoDate: string) => day.format(new Date(isoDate));

/** YYYY-MM-DD in UTC, as the churn endpoint expects. */
export const isoDay = (d: Date) => d.toISOString().slice(0, 10);
