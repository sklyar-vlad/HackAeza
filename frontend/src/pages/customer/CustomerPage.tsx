import { useState, type CSSProperties } from "react";
import { Link, useParams } from "react-router";
import { RiskPill, riskClass } from "@/entities/risk";
import {
  useGetCustomer,
  useListCustomerTransactions,
  useListModelFeatures,
  type ListCustomerTransactionsStatus,
  type ListCustomerTransactionsType,
  type ServiceStatus,
} from "@/shared/api/generated/api";
import { ApiError } from "@/shared/api/http";
import { date, money, score } from "@/shared/lib/format";
import { Dropdown } from "@/shared/ui/Dropdown";
import { Icon } from "@/shared/ui/Icon";

const serviceStatus: Record<ServiceStatus, string> = {
  active: "Активна",
  suspended: "Приостановлена",
  prolong_wait: "Ждёт оплаты",
  deleted: "Удалена",
  blocked: "Заблокирована",
};

const term: Record<string, string> = {
  hour: "час",
  week: "неделя",
  month: "месяц",
  quarter_year: "квартал",
  half_year: "полгода",
  year: "год",
};

export function CustomerPage() {
  const id = Number(useParams().id);
  const { data, error } = useGetCustomer(id, {
    query: { meta: { inlineError: true } },
  });
  const { data: registry } = useListModelFeatures();
  const [type, setType] = useState<ListCustomerTransactionsType | "all">("all");
  const [status, setStatus] = useState<ListCustomerTransactionsStatus | "all">(
    "all",
  );
  const { data: tx } = useListCustomerTransactions(id, {
    type: type === "all" ? undefined : type,
    status: status === "all" ? undefined : status,
  });

  if (error instanceof ApiError && error.status === 404)
    return <p className="empty">Клиент не найден</p>;
  const c = data?.customer;
  if (!c)
    return (
      <p className="empty">
        {error ? "Не удалось загрузить клиента" : "Загрузка…"}
      </p>
    );

  const featureName = (key: string) =>
    registry?.items.find((f) => f.key === key)?.name ?? key;
  const maxContribution = Math.max(
    1,
    ...c.features.map((f) => f.contribution ?? 0),
  );

  return (
    <div className="stack rise">
      <div className="page-head">
        <div>
          <Link
            to="/customers"
            className="muted"
            style={{ display: "inline-flex", gap: 6, textDecoration: "none" }}
          >
            <Icon name="back" size={18} />
            Все клиенты
          </Link>
          <h1 style={{ marginTop: 10, wordBreak: "break-all" }}>{c.email}</h1>
          <p className="page-sub num">ID {c.id}</p>
        </div>
      </div>

      <section className="grid" style={{ "--min": "220px" } as CSSProperties}>
        <div className="card">
          <div className="kpi-label">Баланс</div>
          <div className="kpi-value">{money(c.balance)}</div>
          <div className="kpi-sub">
            бонусы {money(c.bonusBalance)} (
            {c.bonusState === "unlocked" ? "доступны" : "заблокированы"})
          </div>
        </div>
        <div className="card">
          <div className="kpi-label">Всего пополнений</div>
          <div className="kpi-value">{money(c.totalReplenished)}</div>
          <div className="kpi-sub">к выводу {money(c.withdrawBalance)}</div>
        </div>
        <div className="card">
          <div className="kpi-label">Risk score</div>
          <div
            className="kpi-value"
            style={{ display: "flex", alignItems: "center", gap: 12 }}
          >
            {score(c.risk.score)}
            <RiskPill level={c.risk.level} />
          </div>
          <div className="kpi-sub">индекс риска, не вероятность</div>
        </div>
        <div className="card">
          <div className="kpi-label">Потенциальные потери</div>
          <div className="kpi-value">{money(c.loss.amountCents)}</div>
          <div className="kpi-sub">на {c.loss.horizonDays} дней</div>
        </div>
      </section>

      <div className="grid" style={{ "--min": "380px" } as CSSProperties}>
        <section className="card">
          <div className="card-head">
            <h2>Причины риска</h2>
          </div>
          {c.risk.reasons.length ? (
            <ul
              className="stack"
              style={{ gap: 10, margin: 0, paddingLeft: 18 }}
            >
              {c.risk.reasons.map((r) => (
                <li key={r.code + r.serviceIds.join()}>{r.message}</li>
              ))}
            </ul>
          ) : (
            <p className="muted">Причин риска нет</p>
          )}
        </section>

        <section className="card">
          <div className="card-head">
            <h2>Вклад признаков</h2>
          </div>
          <div className="stack" style={{ gap: 14 }}>
            {c.features.map((f) => (
              <div key={f.key} title={f.explanation}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 12,
                  }}
                >
                  <span className={f.applicable ? undefined : "muted"}>
                    {featureName(f.key)}
                  </span>
                  <span className="num">
                    {f.contribution == null ? "—" : f.contribution.toFixed(1)}
                  </span>
                </div>
                <div className="bar" style={{ marginTop: 8 }}>
                  <i
                    style={{
                      width: `${((f.contribution ?? 0) / maxContribution) * 100}%`,
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        </section>
      </div>

      <section className="card">
        <div className="card-head">
          <h2>Услуги</h2>
        </div>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Услуга</th>
                <th>Статус</th>
                <th>Оплата</th>
                <th className="right">Цена</th>
                <th>Действует до</th>
                <th>Автопродление</th>
                <th>Риск</th>
                <th className="right">Потери</th>
              </tr>
            </thead>
            <tbody>
              {c.services.map((s) => (
                <tr key={s.id}>
                  <td>{s.name}</td>
                  <td>{serviceStatus[s.status]}</td>
                  <td>{term[s.paymentTerm]}</td>
                  <td className="right num">{money(s.price)}</td>
                  <td className="num">{date(s.expiresAt)}</td>
                  <td>{s.autoProlong ? "Да" : "Нет"}</td>
                  <td className={riskClass(s.risk.level)}>
                    <RiskPill level={s.risk.level} />
                  </td>
                  <td className="right num">{money(s.loss.amountCents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <div className="card-head">
          <h2>Транзакции услуг</h2>
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: 12,
              flex: "1 1 400px",
              justifyContent: "flex-end",
            }}
          >
            <Dropdown
              label="Тип"
              value={type}
              onChange={setType}
              options={[
                { value: "all", label: "Все" },
                { value: "order", label: "Заказ" },
                { value: "prolong", label: "Продление" },
              ]}
            />
            <Dropdown
              label="Статус"
              value={status}
              onChange={setStatus}
              options={[
                { value: "all", label: "Все" },
                { value: "performed", label: "Проведена" },
                { value: "not_performed", label: "Не проведена" },
              ]}
            />
          </div>
        </div>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Создана</th>
                <th>Тип</th>
                <th className="right">Сумма</th>
                <th>Статус</th>
                <th>Проведена</th>
              </tr>
            </thead>
            <tbody>
              {tx?.items.map((t) => (
                <tr key={t.id}>
                  <td className="num">{t.id}</td>
                  <td className="num">{date(t.createdAt)}</td>
                  <td>{t.type === "order" ? "Заказ" : "Продление"}</td>
                  <td className="right num">{money(t.amount)}</td>
                  <td
                    className={
                      t.status === "performed" ? "risk-low" : "risk-unknown"
                    }
                  >
                    <span className="pill">
                      <i className="dot" />
                      {t.status === "performed" ? "Проведена" : "Не проведена"}
                    </span>
                  </td>
                  <td className="num">{date(t.performedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {tx?.items.length === 0 && <p className="empty">Транзакций нет</p>}
        </div>
      </section>
    </div>
  );
}
