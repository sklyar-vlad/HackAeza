import { useEffect, useMemo, useRef, useState } from "react";
import {
  Activity,
  ArrowDown,
  ArrowDownUp,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  BookOpen,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleHelp,
  Copy,
  Database,
  Download,
  FileJson,
  Home,
  Info,
  LoaderCircle,
  LogOut,
  Menu,
  Monitor,
  Moon,
  RefreshCw,
  Search,
  Settings2,
  ShieldAlert,
  SlidersHorizontal,
  Sun,
  UploadCloud,
  Users,
  Wallet,
  X,
  AlertTriangle,
  CalendarDays,
  Clock3,
  Layers3,
  FileCheck2,
  Eye,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import fixture from "../../testdata/data.json";
import {
  calculate,
  date,
  defaults,
  factors,
  levelLabels,
  money,
  settingsError,
  snapshotWarnings,
  statusLabels,
  termLabels,
  validateSnapshot,
} from "./model";
import type {
  Customer,
  Issue,
  Level,
  ServiceResult,
  Settings,
  Snapshot,
} from "./model";

type Route = "overview" | "customers" | "model" | "source" | "settings";
const titles: Record<Route, string> = {
  overview: "Обзор рисков",
  customers: "Клиенты",
  model: "Настройки модели",
  source: "Источник данных",
  settings: "Настройки",
};
const navItems: { id: Route; label: string; icon: LucideIcon }[] = [
  { id: "overview", label: "Обзор", icon: Home },
  { id: "customers", label: "Клиенты", icon: Users },
  { id: "model", label: "Настройки модели", icon: SlidersHorizontal },
  { id: "source", label: "Источник данных", icon: Database },
];
const levels: Level[] = ["high", "medium", "low", "unknown"];
const sumLoss = (customers: Customer[]) => {
  const known = customers.filter((c) => c.loss !== null);
  return known.length || customers.length === 0
    ? known.reduce((sum, c) => sum + c.loss!, 0n)
    : null;
};
const riskCount = (customers: Customer[], risk: Level) =>
  customers.filter((c) => c.level === risk).length;
const navigate = (page: string) => {
  window.location.hash = page;
};
const readRoute = () => window.location.hash.slice(1) || "overview";
const integer = (n: number) => n.toLocaleString("ru-RU");
const scoreText = (score: number | null) =>
  score === null
    ? "—"
    : score.toLocaleString("ru-RU", { maximumFractionDigits: 2 });

function Badge({ level }: { level: Level }) {
  return (
    <span className={`badge ${level}`}>
      <span className="dot" />
      {levelLabels[level]}
    </span>
  );
}
function Empty({ title, description }: { title: string; description: string }) {
  return (
    <div className="empty">
      <div className="empty-icon">
        <Layers3 size={24} />
      </div>
      <strong>{title}</strong>
      <p>{description}</p>
    </div>
  );
}
function Stat({
  icon: Icon,
  label,
  value,
  note,
  danger = false,
}: {
  icon: LucideIcon;
  label: string;
  value: React.ReactNode;
  note: string;
  danger?: boolean;
}) {
  return (
    <div className="stat card">
      <span className={`stat-icon ${danger ? "danger" : ""}`}>
        <Icon size={23} strokeWidth={1.6} />
      </span>
      <div className="stat-content">
        <div className="stat-label">{label}</div>
        <strong className="stat-value">{value}</strong>
        <div className="stat-note">{note}</div>
      </div>
    </div>
  );
}
function Notice({
  kind = "info",
  children,
}: {
  kind?: "info" | "warning" | "success" | "error";
  children: React.ReactNode;
}) {
  const Icon =
    kind === "success"
      ? CheckCircle2
      : kind === "error" || kind === "warning"
        ? AlertTriangle
        : Info;
  return (
    <div
      className={`notice ${kind}`}
      role={kind === "error" ? "alert" : undefined}
    >
      <Icon size={18} />
      <div>{children}</div>
    </div>
  );
}
function SectionHeading({
  title,
  note,
  action,
}: {
  title: string;
  note?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="section-heading">
      <div>
        <h2>{title}</h2>
        {note && <p>{note}</p>}
      </div>
      {action}
    </div>
  );
}
function Contributions({
  customer,
  service,
}: {
  customer: Customer;
  service?: ServiceResult;
}) {
  const contributions = service?.contributions ?? customer.contributions;
  return (
    <div className="contribution-list">
      {factors.map((f, i) => (
        <div className="contribution" key={f.key}>
          <div>
            <span>{f.label}</span>
            <strong>
              {scoreText(contributions[i])}
              <small> п.</small>
            </strong>
          </div>
          <div className="bar-track">
            <span style={{ width: `${contributions[i]}%` }} />
          </div>
        </div>
      ))}
      <p className="footnote">
        Вклад в score, в пунктах из 100. Округлённые значения могут
        незначительно отличаться от итога.
      </p>
    </div>
  );
}

function Overview({
  customers,
  settings,
}: {
  customers: Customer[];
  settings: Settings;
}) {
  const total = customers.length;
  let end = 0;
  const colors = {
    high: "#ff3b30",
    medium: "#f5aa13",
    low: "#06ba76",
    unknown: "#b5bac5",
  };
  const gradient = levels
    .map((l) => {
      const start = end;
      end += total ? (riskCount(customers, l) / total) * 100 : 0;
      return `${colors[l]} ${start}% ${end}%`;
    })
    .join(", ");
  const priority = [...customers]
    .filter((c) => c.level === "high" || c.level === "medium")
    .sort((a, b) =>
      a.loss === b.loss
        ? (b.score ?? 0) - (a.score ?? 0)
        : (a.loss ?? 0n) > (b.loss ?? 0n)
          ? -1
          : 1,
    )
    .slice(0, 5);
  const upcoming = customers
    .flatMap((c) =>
      c.services
        .filter((s) => s.eligible)
        .map((s) => ({ ...s, email: c.email })),
    )
    .sort((a, b) => Date.parse(a.expiresAt) - Date.parse(b.expiresAt));
  return (
    <>
      <div className="stats-grid">
        <Stat
          icon={Users}
          label="Всего клиентов"
          value={integer(total)}
          note="В текущем снимке данных"
        />
        <Stat
          icon={ShieldAlert}
          label="Высокий риск"
          value={integer(riskCount(customers, "high"))}
          note={`Score от ${settings.high} до 100`}
          danger
        />
        <Stat
          icon={Wallet}
          label="Выручка под риском"
          value={money(sumLoss(customers))}
          note={`Ближайшие оплаты за ${settings.horizon} дн.`}
        />
        <Stat
          icon={CircleHelp}
          label="Без оценки"
          value={integer(riskCount(customers, "unknown"))}
          note="Нет продлений или недостаточно данных"
        />
      </div>
      <div className="overview-grid">
        <section className="card panel distribution">
          <SectionHeading
            title="Распределение по уровню риска"
            note="Клиенты текущей версии расчёта"
          />
          <div className="distribution-body">
            <div
              className="donut"
              role="img"
              aria-label={levels
                .map((l) => `${levelLabels[l]}: ${riskCount(customers, l)}`)
                .join(", ")}
              style={{
                background: total
                  ? `conic-gradient(${gradient})`
                  : "var(--track)",
              }}
            >
              <div>
                <strong>{integer(total)}</strong>
                <span>клиентов</span>
              </div>
            </div>
            <div className="legend">
              {levels.map((l) => (
                <div key={l}>
                  <span className={`dot ${l}`} />
                  <span>{levelLabels[l]}</span>
                  <strong>{riskCount(customers, l)}</strong>
                  <small>
                    {total
                      ? Math.round((riskCount(customers, l) / total) * 100)
                      : 0}
                    %
                  </small>
                </div>
              ))}
            </div>
          </div>
        </section>
        <section className="card panel renewals">
          <SectionHeading
            title="Ближайшие продления"
            note={`Одна оплата на услугу · горизонт ${settings.horizon} дн.`}
            action={
              <span className="icon-muted">
                <CalendarDays size={19} />
              </span>
            }
          />
          {upcoming.length ? (
            <div className="renewal-list">
              {upcoming.slice(0, 4).map((s) => (
                <button
                  className="renewal-row"
                  key={s.id}
                  onClick={() => navigate(`customers/${s.ownerId}`)}
                >
                  <span className="renewal-calendar">
                    <CalendarDays size={18} />
                  </span>
                  <span>
                    <strong>{s.name}</strong>
                    <small>{date(s.expiresAt, false)}</small>
                  </span>
                  <span className="renewal-price">
                    <strong>{money(s.price)}</strong>
                    <small>под риском {money(s.loss)}</small>
                  </span>
                  <ChevronRight size={16} />
                </button>
              ))}
            </div>
          ) : (
            <Empty
              title="Нет ближайших продлений"
              description="В выбранном горизонте нет подходящих услуг."
            />
          )}
          <p className="footnote">
            Сценарная оценка по score. Это не доказанный прогноз убытков и не
            вся выручка за период.
          </p>
        </section>
        <section className="card panel">
          <SectionHeading
            title="Основные причины риска"
            note="Доля всех клиентов с выявленной причиной"
          />
          <div className="reason-list">
            {factors.map((f) => {
              const count = customers.filter((c) =>
                c.reasons.includes(f.label),
              ).length;
              const percent = total ? Math.round((count / total) * 100) : 0;
              return (
                <div className="reason" key={f.key}>
                  <div>
                    <span>{f.label}</span>
                    <strong>{percent}%</strong>
                  </div>
                  <div className="bar-track">
                    <span style={{ width: `${percent}%` }} />
                  </div>
                  <small>
                    {count} из {total} клиентов
                  </small>
                </div>
              );
            })}
          </div>
          <p className="footnote">
            У клиента может быть несколько причин. Их доли не обязаны
            складываться в 100%.
          </p>
        </section>
      </div>
      {customers.some((c) => c.partial) && (
        <Notice kind="warning">
          Часть услуг требует проверки исходных данных. Денежная оценка
          частичная: неоценённые оплаты —{" "}
          <strong>
            {money(customers.reduce((sum, c) => sum + c.unassessed, 0n))}
          </strong>
          .{" "}
          <button className="text-button" onClick={() => navigate("source")}>
            Посмотреть предупреждения <ArrowRight size={13} />
          </button>
        </Notice>
      )}
      <section className="card panel priority">
        <SectionHeading
          title="Клиенты, требующие внимания"
          note="Высокий и средний риск · приоритет по выручке под риском"
          action={
            <button
              className="button primary small"
              onClick={() => navigate("customers")}
            >
              Все клиенты <ArrowRight size={16} />
            </button>
          }
        />
        {priority.length ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Email</th>
                  <th>Уровень риска</th>
                  <th>Основные причины</th>
                  <th className="numeric">Выручка под риском</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {priority.map((c) => (
                  <tr key={c.id}>
                    <td className="muted">{c.id}</td>
                    <td>
                      <button
                        className="customer-link"
                        onClick={() => navigate(`customers/${c.id}`)}
                      >
                        {c.email}
                      </button>
                    </td>
                    <td>
                      <Badge level={c.level} />
                    </td>
                    <td className="reason-cell">
                      {c.reasons.join(", ") || "Повышающие факторы не выявлены"}
                    </td>
                    <td className="numeric strong">{money(c.loss)}</td>
                    <td>
                      <button
                        className="icon-button"
                        aria-label={`Открыть клиента ${c.id}`}
                        onClick={() => navigate(`customers/${c.id}`)}
                      >
                        <ArrowRight size={16} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title="Клиентов с повышенным риском нет"
            description="При текущих настройках никто не попал в категории среднего и высокого риска."
          />
        )}
      </section>
    </>
  );
}

type SortField =
  "id" | "email" | "balance" | "totalReplenished" | "score" | "loss";
function Customers({ customers }: { customers: Customer[] }) {
  const [search, setSearch] = useState("");
  const [risk, setRisk] = useState("all");
  const [sort, setSort] = useState<SortField>("score");
  const [descending, setDescending] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const filtered = useMemo(
    () =>
      customers
        .filter(
          (c) =>
            (!search ||
              c.email.toLowerCase().includes(search.trim().toLowerCase()) ||
              String(c.id) === search.trim()) &&
            (risk === "all" || c.level === risk),
        )
        .sort((a, b) => {
          const av = a[sort],
            bv = b[sort];
          if (av === null || bv === null)
            return av === bv ? a.id - b.id : av === null ? 1 : -1;
          const comparison =
            typeof av === "string" && typeof bv === "string"
              ? av.localeCompare(bv)
              : typeof av !== "string" && typeof bv !== "string"
                ? av < bv
                  ? -1
                  : av > bv
                    ? 1
                    : 0
                : 0;
          return comparison === 0
            ? a.id - b.id
            : comparison * (descending ? -1 : 1);
        }),
    [customers, search, risk, sort, descending],
  );
  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const current = Math.min(page, pages);
  const visible = filtered.slice((current - 1) * pageSize, current * pageSize);
  const header = (field: SortField, label: string) => (
    <th
      className={
        ["balance", "totalReplenished", "loss"].includes(field) ? "numeric" : ""
      }
      aria-sort={
        sort === field ? (descending ? "descending" : "ascending") : "none"
      }
    >
      <button
        className="sort-button"
        onClick={() => {
          if (field === sort) setDescending(!descending);
          else {
            setSort(field);
            setDescending(false);
          }
          setPage(1);
        }}
      >
        {label}
        {field === sort ? (
          descending ? (
            <ArrowDown size={13} />
          ) : (
            <ArrowUp size={13} />
          )
        ) : (
          <ArrowDownUp size={12} />
        )}
      </button>
    </th>
  );
  return (
    <section className="card customer-table">
      <div className="table-toolbar">
        <label className="search">
          <Search size={19} />
          <input
            aria-label="Поиск клиентов"
            placeholder="Поиск по email или ID"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
          />
          {search && (
            <button
              className="icon-button"
              aria-label="Очистить поиск"
              onClick={() => setSearch("")}
            >
              <X size={14} />
            </button>
          )}
        </label>
        <select
          aria-label="Уровень риска"
          value={risk}
          onChange={(e) => {
            setRisk(e.target.value);
            setPage(1);
          }}
        >
          <option value="all">Все уровни риска</option>
          {levels.map((l) => (
            <option value={l} key={l}>
              {levelLabels[l]}
            </option>
          ))}
        </select>
        {(search || risk !== "all") && (
          <button
            className="text-button"
            onClick={() => {
              setSearch("");
              setRisk("all");
              setPage(1);
            }}
          >
            Сбросить
          </button>
        )}
        <span className="result-count">
          {integer(filtered.length)} клиентов
        </span>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {header("id", "ID")}
              {header("email", "Email")}
              {header("balance", "Баланс")}
              {header("totalReplenished", "Всего пополнено")}
              {header("score", "Risk-score")}
              {header("loss", "Выручка под риском")}
              <th>Причины риска</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {visible.map((c) => (
              <tr key={c.id}>
                <td className="muted">{c.id}</td>
                <td>
                  <button
                    className="customer-link"
                    onClick={() => navigate(`customers/${c.id}`)}
                  >
                    {c.email}
                  </button>
                </td>
                <td className="numeric">{money(c.balance)}</td>
                <td className="numeric">{money(c.totalReplenished)}</td>
                <td>
                  <div className="table-score">
                    <span className={`dot ${c.level}`} />
                    <strong>{scoreText(c.score)}</strong>
                    <span className="score-label">{levelLabels[c.level]}</span>
                  </div>
                </td>
                <td className="numeric strong">
                  {money(c.loss)}
                  {c.partial && (
                    <span title="Частичная оценка" className="partial-marker">
                      *
                    </span>
                  )}
                </td>
                <td className="reason-cell">{c.reasons.join(", ") || "—"}</td>
                <td>
                  <button
                    className="icon-button"
                    aria-label={`Открыть клиента ${c.id}`}
                    onClick={() => navigate(`customers/${c.id}`)}
                  >
                    <ChevronRight size={17} />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {!visible.length && (
        <Empty
          title="Клиенты не найдены"
          description="Измените запрос или выбранный уровень риска."
        />
      )}
      <div className="pagination">
        <span>
          Показано {filtered.length ? (current - 1) * pageSize + 1 : 0}–
          {Math.min(current * pageSize, filtered.length)} из {filtered.length}
        </span>
        <label className="page-size">
          Строк на странице{" "}
          <select
            value={pageSize}
            onChange={(e) => {
              setPageSize(Number(e.target.value));
              setPage(1);
            }}
          >
            {[10, 20, 50].map((n) => (
              <option key={n}>{n}</option>
            ))}
          </select>
        </label>
        <div className="page-buttons">
          <button
            className="icon-button"
            aria-label="Предыдущая страница"
            disabled={current === 1}
            onClick={() => setPage(current - 1)}
          >
            <ChevronLeft size={18} />
          </button>
          <span className="current-page">{current}</span>
          <span className="muted">из {pages}</span>
          <button
            className="icon-button"
            aria-label="Следующая страница"
            disabled={current === pages}
            onClick={() => setPage(current + 1)}
          >
            <ChevronRight size={18} />
          </button>
        </div>
      </div>
    </section>
  );
}

function CustomerDetails({
  customer,
  snapshot,
  notify,
}: {
  customer: Customer;
  snapshot: Snapshot;
  notify: (s: string) => void;
}) {
  const [tab, setTab] = useState("services");
  const [serviceId, setServiceId] = useState<number | null>(null);
  const [status, setStatus] = useState("all");
  const [txPage, setTxPage] = useState(1);
  const selectedService = customer.services.find((s) => s.id === serviceId);
  const serviceIds = new Set(customer.services.map((s) => s.id));
  const transactions = snapshot.transactions
    .filter(
      (t) =>
        serviceIds.has(t.serviceId) &&
        (status === "all" || t.status === status),
    )
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt) || a.id - b.id);
  const txPages = Math.max(1, Math.ceil(transactions.length / 5));
  const currentPage = Math.min(txPage, txPages);
  const copy = async (s: string) => {
    try {
      await navigator.clipboard.writeText(s);
      notify("Скопировано");
    } catch {
      notify("Копирование недоступно в этом браузере");
    }
  };
  return (
    <>
      <div className="breadcrumb">
        <button onClick={() => navigate("customers")}>
          <ArrowLeft size={15} />
          Клиенты
        </button>
        <span>/</span>
        <span>{customer.id}</span>
      </div>
      <div className="customer-heading">
        <div>
          <h1>{customer.email}</h1>
          <span className="muted">
            ID: {customer.id}{" "}
            <button
              className="icon-button inline"
              aria-label="Скопировать ID"
              onClick={() => copy(String(customer.id))}
            >
              <Copy size={14} />
            </button>
          </span>
        </div>
        <Badge level={customer.level} />
      </div>
      <div className="stats-grid">
        <Stat
          icon={Wallet}
          label="Основной баланс"
          value={money(customer.balance)}
          note="Средства на оплату услуг"
        />
        <Stat
          icon={ArrowDownUp}
          label="Всего пополнено"
          value={money(customer.totalReplenished)}
          note="Сумма из исходного снимка"
        />
        <Stat
          icon={ShieldAlert}
          label="Risk-score"
          value={
            <>
              {scoreText(customer.score)}
              <small className="out-of"> / 100</small>
            </>
          }
          note={
            customer.score === null
              ? "Оценка клиента недоступна"
              : levelLabels[customer.level] + " риск"
          }
          danger={customer.level === "high"}
        />
        <Stat
          icon={Activity}
          label="Выручка под риском"
          value={money(customer.loss)}
          note={
            customer.partial
              ? "Частичная оценка по известным услугам"
              : "Стоимость ближайших непродлений"
          }
        />
      </div>
      <div className="detail-grid">
        <section className="card panel">
          <SectionHeading
            title="Финансы клиента"
            note="Данные аккаунта без восстановления истории"
          />
          <dl className="account-info">
            <div>
              <dt>Email</dt>
              <dd>
                {customer.email}
                <button
                  className="icon-button inline"
                  aria-label="Скопировать email"
                  onClick={() => copy(customer.email)}
                >
                  <Copy size={14} />
                </button>
              </dd>
            </div>
            <div>
              <dt>Бонусный баланс</dt>
              <dd>
                {money(customer.bonusBalance)}
                <span
                  className={`tag ${customer.bonusState === "unlocked" ? "green" : ""}`}
                >
                  {customer.bonusState === "unlocked"
                    ? "Доступен"
                    : "Заблокирован"}
                </span>
              </dd>
            </div>
            <div>
              <dt>Реферальный баланс</dt>
              <dd>{money(customer.withdrawBalance)}</dd>
            </div>
            <div>
              <dt>Количество услуг</dt>
              <dd>{customer.services.length}</dd>
            </div>
            <div>
              <dt>Неоценённые оплаты</dt>
              <dd>{money(customer.unassessed)}</dd>
            </div>
          </dl>
          <p className="footnote">
            Реферальный баланс не используется для продления автоматически.
            Бонусный учитывается только в доступном состоянии.
          </p>
        </section>
        <section className="card panel">
          <SectionHeading
            title="Почему такой риск"
            note={
              customer.score === null
                ? "Известные факторы по оценённым услугам"
                : "Вклад факторов в итоговую оценку"
            }
          />
          {customer.services.some((s) => s.score !== null) && (
            <Contributions customer={customer} />
          )}
          {customer.score === null && (
            <Notice kind="warning">{customer.reasons.join(". ")}</Notice>
          )}
        </section>
      </div>
      <section className="card panel">
        <div className="tabs">
          <button
            className={tab === "services" ? "active" : ""}
            onClick={() => setTab("services")}
          >
            Услуги <span>{customer.services.length}</span>
          </button>
          <button
            className={tab === "transactions" ? "active" : ""}
            onClick={() => setTab("transactions")}
          >
            Транзакции{" "}
            <span>
              {
                snapshot.transactions.filter((t) => serviceIds.has(t.serviceId))
                  .length
              }
            </span>
          </button>
        </div>
        {tab === "services" ? (
          <>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Услуга</th>
                    <th>Статус</th>
                    <th>Период</th>
                    <th className="numeric">Цена за период</th>
                    <th>Окончание</th>
                    <th>Автопродление</th>
                    <th>Risk-score</th>
                    <th className="numeric">Под риском</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {customer.services.map((s) => (
                    <tr
                      key={s.id}
                      className={serviceId === s.id ? "selected-row" : ""}
                    >
                      <td>
                        <strong>{s.name}</strong>
                        <small className="cell-sub">ID {s.id}</small>
                      </td>
                      <td>
                        <span
                          className={`tag ${s.status === "active" ? "green" : s.status === "prolong_wait" ? "amber" : ""}`}
                        >
                          {statusLabels[s.status]}
                        </span>
                      </td>
                      <td>{termLabels[s.paymentTerm]}</td>
                      <td className="numeric">{money(s.price)}</td>
                      <td>{date(s.expiresAt, false)}</td>
                      <td>
                        {s.autoProlong ? (
                          <span className="check-text">
                            <Check size={14} />
                            Включено
                          </span>
                        ) : (
                          "Выключено"
                        )}
                      </td>
                      <td>
                        <span className="table-score">
                          <span className={`dot ${s.level}`} />
                          {scoreText(s.score)}
                        </span>
                      </td>
                      <td className="numeric">{money(s.loss)}</td>
                      <td>
                        <button
                          className="icon-button"
                          aria-label={`Показать расчёт ${s.name}`}
                          onClick={() =>
                            setServiceId(serviceId === s.id ? null : s.id)
                          }
                        >
                          <ChevronDown size={16} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {!customer.services.length && (
              <Empty
                title="У клиента нет услуг"
                description="В загруженном снимке услуги отсутствуют."
              />
            )}
            {selectedService && (
              <div className="service-calculation">
                <SectionHeading
                  title={`Расчёт: ${selectedService.name}`}
                  note={selectedService.reason}
                  action={
                    <button
                      className="icon-button"
                      aria-label="Закрыть расчёт услуги"
                      onClick={() => setServiceId(null)}
                    >
                      <X size={16} />
                    </button>
                  }
                />
                {selectedService.score !== null ? (
                  <Contributions
                    customer={customer}
                    service={selectedService}
                  />
                ) : (
                  <Notice>
                    {selectedService.reason}. Отсутствие оценки не означает
                    низкий риск.
                  </Notice>
                )}
              </div>
            )}
          </>
        ) : (
          <>
            <div className="transactions-toolbar">
              <p className="footnote">
                Заказы и продления услуг. Это не полная история пополнений.
              </p>
              <select
                aria-label="Статус транзакции"
                value={status}
                onChange={(e) => {
                  setStatus(e.target.value);
                  setTxPage(1);
                }}
              >
                <option value="all">Все статусы</option>
                <option value="performed">Выполнено</option>
                <option value="not_performed">Не выполнено</option>
              </select>
            </div>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Создана</th>
                    <th>Тип</th>
                    <th>Услуга</th>
                    <th className="numeric">Сумма</th>
                    <th>Статус</th>
                    <th>Выполнена</th>
                  </tr>
                </thead>
                <tbody>
                  {transactions
                    .slice((currentPage - 1) * 5, currentPage * 5)
                    .map((t) => (
                      <tr key={t.id}>
                        <td>{t.id}</td>
                        <td>{date(t.createdAt)}</td>
                        <td>{t.type === "order" ? "Заказ" : "Продление"}</td>
                        <td>
                          {
                            customer.services.find((s) => s.id === t.serviceId)
                              ?.name
                          }
                        </td>
                        <td className="numeric">{money(t.amount)}</td>
                        <td>
                          <span
                            className={`tag ${t.status === "performed" ? "green" : ""}`}
                          >
                            {t.status === "performed"
                              ? "Выполнено"
                              : "Не выполнено"}
                          </span>
                        </td>
                        <td>{t.performedAt ? date(t.performedAt) : "—"}</td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
            {!transactions.length && (
              <Empty
                title="Транзакций нет"
                description="Нет записей с выбранным статусом."
              />
            )}
            <div className="pagination">
              <span>{transactions.length} транзакций</span>
              <div className="page-buttons">
                <button
                  className="icon-button"
                  aria-label="Предыдущие транзакции"
                  disabled={currentPage === 1}
                  onClick={() => setTxPage(currentPage - 1)}
                >
                  <ChevronLeft size={18} />
                </button>
                <span>
                  {currentPage} из {txPages}
                </span>
                <button
                  className="icon-button"
                  aria-label="Следующие транзакции"
                  disabled={currentPage === txPages}
                  onClick={() => setTxPage(currentPage + 1)}
                >
                  <ChevronRight size={18} />
                </button>
              </div>
            </div>
          </>
        )}
      </section>
    </>
  );
}

function ModelSettings({
  settings,
  onSave,
  version,
  snapshot,
  customers,
  calculatedAt,
}: {
  settings: Settings;
  onSave: (s: Settings) => void;
  version: number;
  snapshot: Snapshot;
  customers: Customer[];
  calculatedAt: string;
}) {
  const [draft, setDraft] = useState<Settings>(() => structuredClone(settings));
  const [preview, setPreview] = useState<Customer[] | null>(null);
  const error = settingsError(draft);
  const dirty = JSON.stringify(draft) !== JSON.stringify(settings);
  const change = (next: Settings) => {
    setDraft(next);
    setPreview(null);
  };
  const active = Object.values(draft.weights).filter((w) => w > 0).length;
  return (
    <>
      <div className="stats-grid three">
        <Stat
          icon={Layers3}
          label="Версия настроек"
          value={`№ ${version}`}
          note="Версия текущей модели в демосеансе"
        />
        <Stat
          icon={SlidersHorizontal}
          label="Факторы риска"
          value={`${active} из ${factors.length}`}
          note="Коэффициент 0 отключает фактор"
        />
        <Stat
          icon={CalendarDays}
          label="Горизонт расчёта"
          value={`${settings.horizon} дней`}
          note="Одна ближайшая оплата на услугу"
        />
      </div>
      <div className="model-grid">
        <section className="card panel">
          <SectionHeading
            title="Факторы риска"
            note="Относительные веса. Сумма не обязана равняться 100."
          />
          <div className="factor-list">
            {factors.map((f) => (
              <div className="factor-row" key={f.key}>
                <div>
                  <strong>{f.label}</strong>
                  <p>{f.description}</p>
                </div>
                <div className="factor-controls">
                  <input
                    aria-label={`${f.label}: ползунок`}
                    type="range"
                    min="0"
                    max="100"
                    step="1"
                    value={Math.min(100, draft.weights[f.key])}
                    onChange={(e) =>
                      change({
                        ...draft,
                        weights: {
                          ...draft.weights,
                          [f.key]: Number(e.target.value),
                        },
                      })
                    }
                  />
                  <input
                    className="weight-input"
                    aria-label={`${f.label}: коэффициент`}
                    type="number"
                    min="0"
                    max="1000"
                    step="0.01"
                    value={draft.weights[f.key]}
                    onChange={(e) =>
                      change({
                        ...draft,
                        weights: {
                          ...draft.weights,
                          [f.key]: Number(e.target.value),
                        },
                      })
                    }
                  />
                  <button
                    className={`switch ${draft.weights[f.key] > 0 ? "on" : ""}`}
                    role="switch"
                    aria-checked={draft.weights[f.key] > 0}
                    aria-label={`Включить фактор: ${f.label}`}
                    onClick={() =>
                      change({
                        ...draft,
                        weights: {
                          ...draft.weights,
                          [f.key]: draft.weights[f.key] > 0 ? 0 : f.weight,
                        },
                      })
                    }
                  >
                    <span />
                  </button>
                </div>
              </div>
            ))}
          </div>
          <div className="formula">
            <span className="eyebrow">Формула модели</span>
            <p>Score = 100 × Σ(вес × признак) / Σ(весов)</p>
            <small>
              Это объяснимый индекс риска, а не вероятность оттока. Веса не
              обучены на исторических данных.
            </small>
          </div>
        </section>
        <div className="model-side">
          <section className="card panel">
            <SectionHeading
              title="Пороги уровней риска"
              note="Границы задаются двумя числами"
            />
            <label className="threshold">
              <span>
                <span className="dot high" />
                Высокий риск от
              </span>
              <input
                aria-label="Порог высокого риска"
                type="number"
                min="0"
                max="100"
                value={draft.high}
                onChange={(e) =>
                  change({ ...draft, high: Number(e.target.value) })
                }
              />
            </label>
            <label className="threshold">
              <span>
                <span className="dot medium" />
                Средний риск от
              </span>
              <input
                aria-label="Порог среднего риска"
                type="number"
                min="0"
                max="100"
                value={draft.medium}
                onChange={(e) =>
                  change({ ...draft, medium: Number(e.target.value) })
                }
              />
            </label>
            <div className="threshold">
              <span>
                <span className="dot low" />
                Низкий риск
              </span>
              <strong>меньше {draft.medium}</strong>
            </div>
            <div className="threshold">
              <span>
                <span className="dot unknown" />
                Без оценки
              </span>
              <span className="muted">Недостаточно данных</span>
            </div>
          </section>
          <section className="card panel">
            <SectionHeading title="Горизонт прогнозирования" />
            <label className="threshold">
              <span>Ближайшие продления за</span>
              <span className="input-with-unit">
                <input
                  aria-label="Горизонт в днях"
                  type="number"
                  min="1"
                  max="365"
                  value={draft.horizon}
                  onChange={(e) =>
                    change({ ...draft, horizon: Number(e.target.value) })
                  }
                />
                дн.
              </span>
            </label>
            <p className="footnote">
              Период оплаты уже включён в цену услуги. Оценка не суммирует все
              повторные списания за горизонт.
            </p>
          </section>
        </div>
      </div>
      {error && <Notice kind="error">{error}</Notice>}
      {preview && (
        <section className="card panel">
          <SectionHeading
            title="Предварительный эффект"
            note="Локальный what-if: настройки и опубликованные результаты ещё не изменены"
          />
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Показатель</th>
                  <th>Сейчас</th>
                  <th>После изменения</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Клиенты с высоким риском</td>
                  <td>{riskCount(customers, "high")}</td>
                  <td>{riskCount(preview, "high")}</td>
                </tr>
                <tr>
                  <td>Выручка под риском</td>
                  <td>{money(sumLoss(customers))}</td>
                  <td>{money(sumLoss(preview))}</td>
                </tr>
                <tr>
                  <td>Клиенты без оценки</td>
                  <td>{riskCount(customers, "unknown")}</td>
                  <td>{riskCount(preview, "unknown")}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      )}
      <div className="save-bar card">
        <button
          className="text-button muted"
          onClick={() => change(structuredClone(defaults))}
        >
          <RefreshCw size={16} />
          Значения по умолчанию
        </button>
        <div>
          <button
            className="button"
            disabled={!dirty}
            onClick={() => {
              change(structuredClone(settings));
            }}
          >
            Отменить
          </button>
          <button
            className="button"
            disabled={!!error}
            onClick={() =>
              setPreview(calculate(snapshot, draft, Date.parse(calculatedAt)))
            }
          >
            <Eye size={16} />
            Предпросмотр
          </button>
          <button
            className="button primary"
            disabled={!!error || !dirty}
            onClick={() => {
              onSave(structuredClone(draft));
              setPreview(null);
            }}
          >
            <Check size={16} />
            Сохранить и пересчитать
          </button>
        </div>
      </div>
    </>
  );
}

function IssueTable({ issues }: { issues: Issue[] }) {
  return (
    <div className="table-wrap">
      <table className="issue-table">
        <thead>
          <tr>
            <th>Описание</th>
            <th>Запись</th>
          </tr>
        </thead>
        <tbody>
          {issues.slice(0, 20).map((issue, i) => (
            <tr key={`${issue.path}-${i}`}>
              <td>{issue.message}</td>
              <td>
                <code>{issue.path}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {issues.length > 20 && (
        <p className="footnote">Показано 20 из {issues.length} сообщений.</p>
      )}
    </div>
  );
}

function Source({
  snapshot,
  importedAt,
  version,
  warnings,
  onImport,
}: {
  snapshot: Snapshot;
  importedAt: string | null;
  version: number;
  warnings: Issue[];
  onImport: (s: Snapshot) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [errors, setErrors] = useState<Issue[]>([]);
  const [dragging, setDragging] = useState(false);
  const [busy, setBusy] = useState(false);
  const choose = (next: File | null) => {
    setFile(next);
    setErrors([]);
  };
  const upload = async () => {
    if (!file || busy) return;
    setBusy(true);
    setErrors([]);
    try {
      if (file.size > 10 * 1024 * 1024) {
        setErrors([
          { path: file.name, message: "Лимит файла в макете — 10 МБ." },
        ]);
        return;
      }
      if (!file.name.toLowerCase().endsWith(".json")) {
        setErrors([
          { path: file.name, message: "Выберите файл формата .json." },
        ]);
        return;
      }
      const text = await file.text();
      let parsed: unknown;
      try {
        parsed = JSON.parse(text);
      } catch {
        setErrors([{ path: "$", message: "Файл содержит некорректный JSON." }]);
        return;
      }
      const validation = validateSnapshot(parsed);
      if (!validation.snapshot) {
        setErrors(validation.errors);
        return;
      }
      onImport(validation.snapshot);
      choose(null);
      if (input.current) input.current.value = "";
    } catch {
      setErrors([
        {
          path: "$",
          message: "Не удалось обработать снимок. Предыдущие данные сохранены.",
        },
      ]);
    } finally {
      setBusy(false);
    }
  };
  const download = () => {
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(fixture, null, 2)], {
        type: "application/json",
      }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = "data.json";
    a.click();
    URL.revokeObjectURL(url);
  };
  return (
    <>
      <div className="stats-grid">
        <Stat
          icon={Users}
          label="Аккаунты"
          value={snapshot.accounts.length}
          note="Из текущего снимка"
        />
        <Stat
          icon={Layers3}
          label="Услуги"
          value={snapshot.services.length}
          note="Из текущего снимка"
        />
        <Stat
          icon={ArrowDownUp}
          label="Транзакции"
          value={snapshot.transactions.length}
          note="Заказы и продления услуг"
        />
        <Stat
          icon={Clock3}
          label="Последняя загрузка"
          value={importedAt ? date(importedAt, false) : "Тестовый набор"}
          note={
            importedAt ? date(importedAt) : "Начальный комплект организаторов"
          }
        />
      </div>
      <section className="card panel">
        <SectionHeading
          title="Загрузить снимок данных"
          note="JSON с массивами accounts, services и transactions"
          action={
            <button className="text-button" onClick={download}>
              <Download size={15} />
              Скачать тестовый JSON
            </button>
          }
        />
        <div className="upload-layout">
          <div
            className={`dropzone ${dragging ? "dragging" : ""}`}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              choose(e.dataTransfer.files[0] ?? null);
            }}
          >
            <input
              ref={input}
              id="snapshot-file"
              aria-label="Выбрать JSON снимок"
              type="file"
              accept=".json,application/json"
              onChange={(e) => choose(e.target.files?.[0] ?? null)}
            />
            <label htmlFor="snapshot-file">
              <span className="upload-icon">
                <UploadCloud size={30} strokeWidth={1.4} />
              </span>
              <strong>
                Перетащите файл сюда или <span>выберите на устройстве</span>
              </strong>
              <small>Формат .json · до 10 МБ в макете</small>
            </label>
          </div>
          <div className="upload-actions">
            {file ? (
              <div className="selected-file">
                <FileJson size={25} />
                <div>
                  <strong>{file.name}</strong>
                  <small>
                    {new Intl.NumberFormat("ru-RU", {
                      maximumFractionDigits: 1,
                    }).format(file.size / 1024)}{" "}
                    КБ
                  </small>
                </div>
                <button
                  className="icon-button"
                  aria-label="Убрать файл"
                  onClick={() => {
                    choose(null);
                    if (input.current) input.current.value = "";
                  }}
                >
                  <X size={16} />
                </button>
              </div>
            ) : (
              <div className="file-placeholder">
                <FileJson size={22} />
                <span>Файл ещё не выбран</span>
              </div>
            )}
            <button
              className="button primary"
              disabled={!file || busy}
              onClick={upload}
            >
              {busy ? (
                <>
                  <LoaderCircle className="spin" size={16} />
                  Проверка и пересчёт…
                </>
              ) : (
                <>
                  <UploadCloud size={16} />
                  Загрузить и пересчитать
                </>
              )}
            </button>
            <Notice kind="warning">
              Текущие данные будут полностью заменены. Пустые массивы удалят
              соответствующие записи.
            </Notice>
          </div>
        </div>
      </section>
      {errors.length > 0 && (
        <section className="card panel">
          <Notice kind="error">
            <strong>Снимок не загружен.</strong> Предыдущие данные сохранены.
            Исправьте ошибки и попробуйте снова.
          </Notice>
          <IssueTable issues={errors} />
        </section>
      )}
      <section className="card panel">
        <SectionHeading
          title="Текущий снимок"
          note="Сведения о данных и последнем опубликованном расчёте"
        />
        <div className="source-result-grid">
          <div>
            <Notice kind="success">
              <strong>
                {importedAt
                  ? "Загрузка успешно завершена"
                  : "Комплект организаторов подключён к макету"}
              </strong>
              <br />
              Данные доступны, риски рассчитаны локально.
            </Notice>
            <dl className="account-info">
              <div>
                <dt>Загружено в приложение</dt>
                <dd>
                  {importedAt ? date(importedAt) : "Начальный тестовый набор"}
                </dd>
              </div>
              <div>
                <dt>Дата снимка источника</dt>
                <dd>{date(snapshot.capturedAt ?? null)}</dd>
              </div>
              <div>
                <dt>Принято аккаунтов / услуг / транзакций</dt>
                <dd>
                  {snapshot.accounts.length} / {snapshot.services.length} /{" "}
                  {snapshot.transactions.length}
                </dd>
              </div>
              <div>
                <dt>Версия расчёта</dt>
                <dd>№ {version}</dd>
              </div>
            </dl>
            <p className="footnote">
              Импорт в этом макете действует до перезагрузки страницы. Серверное
              хранение будет подключено вместе с Go API.
            </p>
          </div>
          <div>
            {warnings.length ? (
              <>
                <Notice kind="warning">
                  <strong>Предупреждения: {warnings.length}</strong>
                  <br />
                  Данные сохранены без исправлений. Аномальные услуги не
                  получают фиктивный score.
                </Notice>
                <IssueTable issues={warnings} />
              </>
            ) : (
              <Notice kind="success">
                Предупреждений по статусам и датам услуг нет.
              </Notice>
            )}
          </div>
        </div>
      </section>
    </>
  );
}

export default function App() {
  const [route, setRoute] = useState(readRoute);
  const [snapshot, setSnapshot] = useState<Snapshot>(fixture as Snapshot);
  const [settings, setSettings] = useState<Settings>(() =>
    structuredClone(defaults),
  );
  const [calculatedAt, setCalculatedAt] = useState(() =>
    new Date().toISOString(),
  );
  const [importedAt, setImportedAt] = useState<string | null>(null);
  const [version, setVersion] = useState(1);
  const [modelVersion, setModelVersion] = useState(1);
  const [toast, setToast] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [session, setSession] = useState(true);
  const [theme, setTheme] = useState(() => {
    try {
      return localStorage.getItem("hackaeza-theme") ?? "light";
    } catch {
      return "light";
    }
  });
  const customers = useMemo(
    () => calculate(snapshot, settings, Date.parse(calculatedAt)),
    [snapshot, settings, calculatedAt],
  );
  const warnings = useMemo(
    () => snapshotWarnings(snapshot, Date.parse(calculatedAt)),
    [snapshot, calculatedAt],
  );
  const detailId = route.startsWith("customers/")
    ? Number(route.split("/")[1])
    : null;
  const page: Route = route.startsWith("customers")
    ? "customers"
    : Object.hasOwn(titles, route)
      ? (route as Route)
      : "overview";
  const customer = customers.find((c) => c.id === detailId);
  useEffect(() => {
    const listener = () => {
      setRoute(readRoute());
      setSidebarOpen(false);
      window.scrollTo(0, 0);
    };
    window.addEventListener("hashchange", listener);
    return () => window.removeEventListener("hashchange", listener);
  }, []);
  useEffect(() => {
    document.title = `${detailId ? "Карточка клиента" : titles[page]} — HackAeza`;
  }, [page, detailId]);
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      document.documentElement.dataset.theme =
        theme === "system" ? (media.matches ? "dark" : "light") : theme;
    };
    apply();
    media.addEventListener("change", apply);
    try {
      localStorage.setItem("hackaeza-theme", theme);
    } catch {
      /* Theme remains available without browser storage. */
    }
    return () => media.removeEventListener("change", apply);
  }, [theme]);
  useEffect(() => {
    if (!toast) return;
    const timeout = window.setTimeout(() => setToast(""), 3500);
    return () => window.clearTimeout(timeout);
  }, [toast]);
  const saveSettings = (next: Settings) => {
    setSettings(next);
    setModelVersion((v) => v + 1);
    setVersion((v) => v + 1);
    setCalculatedAt(new Date().toISOString());
    setToast("Настройки сохранены, показатели пересчитаны");
  };
  const importSnapshot = (next: Snapshot) => {
    calculate(next, settings, Date.now());
    setSnapshot(next);
    const now = new Date().toISOString();
    setImportedAt(now);
    setCalculatedAt(now);
    setVersion((v) => v + 1);
    setToast("Снимок загружен, риски пересчитаны");
  };
  if (!session)
    return (
      <div className="demo-entry">
        <div className="card panel">
          <span className="brand-word">
            aéza<span> / risk</span>
          </span>
          <h1>
            Анализ рисков
            <br />
            непродления
          </h1>
          <p>Интерактивный макет на данных организаторов.</p>
          <Notice>
            В этом макете нет серверной авторизации. Вход по паролю и OIDC будут
            подключены вместе с API.
          </Notice>
          <button
            className="button primary"
            onClick={() => {
              setSession(true);
              navigate("overview");
            }}
          >
            Открыть демосеанс <ArrowRight size={16} />
          </button>
        </div>
      </div>
    );
  return (
    <div className="app">
      <button
        className="mobile-menu icon-button"
        aria-label="Открыть меню"
        onClick={() => setSidebarOpen(!sidebarOpen)}
      >
        <Menu size={22} />
      </button>
      {sidebarOpen && (
        <button
          className="sidebar-overlay"
          aria-label="Закрыть меню"
          onClick={() => setSidebarOpen(false)}
        />
      )}
      <aside className={`sidebar ${sidebarOpen ? "open" : ""}`}>
        <button
          className="brand"
          aria-label="HackAeza — обзор"
          onClick={() => navigate("overview")}
        >
          <span className="brand-word">
            aéza<span> / risk</span>
          </span>
          <span className="brand-caption">CUSTOMER INTELLIGENCE</span>
        </button>
        <span className="nav-caption">РАБОЧЕЕ ПРОСТРАНСТВО</span>
        <nav aria-label="Основная навигация">
          {navItems.map(({ id, label, icon: Icon }) => (
            <a
              key={id}
              href={`#${id}`}
              className={`nav-item ${page === id ? "active" : ""}`}
              aria-current={page === id ? "page" : undefined}
            >
              <Icon size={19} strokeWidth={1.65} />
              <span>{label}</span>
              {page === id && <span className="nav-indicator" />}
            </a>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <a
            className="nav-item"
            href="/docs/"
            target="_blank"
            rel="noreferrer"
          >
            <BookOpen size={18} strokeWidth={1.6} />
            <span>База знаний</span>
            <ArrowUp size={13} className="external-arrow" />
          </a>
          <a
            className={`nav-item ${page === "settings" ? "active" : ""}`}
            href="#settings"
          >
            <Settings2 size={18} strokeWidth={1.6} />
            <span>Настройки</span>
          </a>
          <div className="sidebar-footer">
            <div className="mini-logo">é</div>
            <div>
              <strong>HackAeza</strong>
              <span>Интерактивный макет</span>
            </div>
            <span className="live-dot" />
          </div>
        </div>
      </aside>
      <main className="main">
        <div className="topbar">
          <span className="demo-pill">
            <span className="dot" />
            Демонстрационный режим
          </span>
          <div className="topbar-right">
            <button
              className="refresh"
              onClick={() => {
                setCalculatedAt(new Date().toISOString());
                setVersion((v) => v + 1);
                setToast("Показатели обновлены");
              }}
              title="Пересчитать на текущее время"
            >
              <RefreshCw size={15} />
              <span>Расчёт {date(calculatedAt)}</span>
            </button>
            <span className="topbar-divider" />
            <button
              className="profile"
              onClick={() => navigate("settings")}
              aria-label="Настройки профиля"
            >
              <span>Д</span>
              <span>Демосеанс</span>
              <ChevronDown size={13} />
            </button>
          </div>
        </div>
        {!detailId && (
          <header className="page-header">
            <div className="eyebrow">
              {page === "overview"
                ? "АНАЛИТИКА"
                : page === "customers"
                  ? "КЛИЕНТСКАЯ БАЗА"
                  : page === "source"
                    ? "ДАННЫЕ"
                    : "КОНФИГУРАЦИЯ"}
            </div>
            <h1>{titles[page]}</h1>
            <p>
              {page === "overview"
                ? "Кто может не продлить услуги, почему и сколько выручки под риском."
                : page === "customers"
                  ? "Рейтинг клиентов с объяснимой оценкой риска непродления."
                  : page === "model"
                    ? "Настройте факторы риска и посмотрите, как изменятся результаты."
                    : page === "source"
                      ? "Загрузите снимок аккаунтов, услуг и транзакций для пересчёта рисков."
                      : "Оформление интерфейса и управление демосеансом."}
            </p>
          </header>
        )}
        <div className="page-content">
          {page === "overview" && (
            <Overview customers={customers} settings={settings} />
          )}
          {page === "customers" &&
            (detailId ? (
              customer ? (
                <CustomerDetails
                  key={customer.id}
                  customer={customer}
                  snapshot={snapshot}
                  notify={setToast}
                />
              ) : (
                <section className="card panel">
                  <Empty
                    title="Клиент не найден"
                    description="В текущем снимке нет аккаунта с таким ID."
                  />
                  <button
                    className="button"
                    onClick={() => navigate("customers")}
                  >
                    <ArrowLeft size={15} />К списку клиентов
                  </button>
                </section>
              )
            ) : (
              <Customers customers={customers} />
            ))}
          {page === "model" && (
            <ModelSettings
              key={modelVersion}
              settings={settings}
              onSave={saveSettings}
              version={modelVersion}
              snapshot={snapshot}
              customers={customers}
              calculatedAt={calculatedAt}
            />
          )}
          {page === "source" && (
            <Source
              snapshot={snapshot}
              importedAt={importedAt}
              version={version}
              warnings={warnings}
              onImport={importSnapshot}
            />
          )}
          {page === "settings" && (
            <div className="settings-grid">
              <section className="card panel">
                <SectionHeading
                  title="Оформление"
                  note="Выберите удобную тему интерфейса"
                />
                <div className="theme-options">
                  {[
                    { id: "light", label: "Светлая", icon: Sun },
                    { id: "dark", label: "Тёмная", icon: Moon },
                    { id: "system", label: "Системная", icon: Monitor },
                  ].map(({ id, label, icon: Icon }) => (
                    <button
                      key={id}
                      className={`theme-option ${theme === id ? "selected" : ""}`}
                      onClick={() => setTheme(id)}
                    >
                      <Icon size={26} strokeWidth={1.5} />
                      <span>{label}</span>
                      {theme === id && <CheckCircle2 size={16} />}
                    </button>
                  ))}
                </div>
                <p className="footnote">
                  Выбор темы сохраняется в этом браузере.
                </p>
              </section>
              <section className="card panel">
                <SectionHeading title="Профиль" note="Демонстрационный сеанс" />
                <div className="profile-card">
                  <span className="avatar">Д</span>
                  <div>
                    <strong>Демосеанс</strong>
                    <p>Предпросмотр интерфейса HackAeza</p>
                  </div>
                </div>
                <Notice>
                  Импорт и настройки модели действуют до перезагрузки. Серверная
                  авторизация в этот макет не включена.
                </Notice>
                <button
                  className="button danger-outline"
                  onClick={() => setSession(false)}
                >
                  <LogOut size={16} />
                  Завершить демосеанс
                </button>
              </section>
            </div>
          )}
        </div>
        <footer className="page-footer">
          <span>
            HackAeza <span className="muted">/</span> Анализ рисков непродления
          </span>
          <span>
            Версия расчёта № {version} <span className="muted">·</span> Модель №{" "}
            {modelVersion}
          </span>
        </footer>
      </main>
      {toast && (
        <div className="toast" role="status">
          <FileCheck2 size={18} />
          {toast}
          <button
            className="icon-button"
            aria-label="Закрыть уведомление"
            onClick={() => setToast("")}
          >
            <X size={14} />
          </button>
        </div>
      )}
    </div>
  );
}
