import { useState, type CSSProperties } from "react";
import { Link, useNavigate } from "react-router";
import { RISK_LEVELS, RiskPill, riskClass, riskLabel } from "@/entities/risk";
import {
  useGetAnalyticsSummary,
  useGetChurnSeries,
  useListCustomers,
} from "@/shared/api/generated/api";
import { date, int, isoDay, money, percent, score } from "@/shared/lib/format";
import { Dropdown } from "@/shared/ui/Dropdown";
import { ChurnChart } from "./ChurnChart";
import { RiskRing } from "./RiskRing";

const PERIODS = [
  { value: "30", label: "30 дней" },
  { value: "7", label: "7 дней" },
  { value: "90", label: "90 дней" },
] as const;

export function OverviewPage() {
  const navigate = useNavigate();
  const [days, setDays] = useState<(typeof PERIODS)[number]["value"]>("30");
  const { data: summary } = useGetAnalyticsSummary();
  const today = new Date();
  const { data: churn } = useGetChurnSeries({
    from: isoDay(today),
    to: isoDay(new Date(today.getTime() + Number(days) * 86_400_000)),
  });
  const { data: top } = useListCustomers({
    riskLevel: ["high"],
    sortBy: "potentialLoss",
    pageSize: 5,
  });

  const empty = summary?.calculation.version === 0;
  const total = summary?.totalCustomers ?? 0;
  const byRisk = summary?.customersByRisk;

  return (
    <div className="stack rise">
      <div className="page-head">
        <div>
          <div className="home-word" aria-hidden="true">
            home
          </div>
          <h1>обзор рисков</h1>
        </div>
        <span className="muted">
          Расчёт от {date(summary?.calculation.calculatedAt)}
        </span>
      </div>

      {empty && (
        <div className="card">
          <p>
            Данных ещё нет.{" "}
            <Link className="link" to="/source">
              Загрузите снимок
            </Link>
            , чтобы посчитать риски.
          </p>
        </div>
      )}

      <section
        className="grid"
        style={{ "--min": "220px" } as CSSProperties}
        aria-label="Ключевые показатели"
      >
        <div className="card">
          <div className="kpi-label">Всего клиентов</div>
          <div className="kpi-value">{int(total)}</div>
          <div className="kpi-sub">
            оценено {int(summary?.assessedCustomers ?? 0)}
          </div>
        </div>
        <div className="card">
          <div className="kpi-label">Высокий риск</div>
          <div className="kpi-value">{int(byRisk?.high ?? 0)}</div>
          <div className="kpi-sub">
            {percent(byRisk?.high ?? 0, total)} клиентов
          </div>
        </div>
        <div className="card">
          <div className="kpi-label">Выручка под риском</div>
          <div className="kpi-value">{money(summary?.loss.amountCents)}</div>
          <div className="kpi-sub">
            на {summary?.loss.horizonDays ?? "—"} дней
            {summary?.loss.isPartial ? ", оценка частичная" : ""}
          </div>
        </div>
        <div className="card">
          <div className="kpi-label">Услуги под риском</div>
          <div className="kpi-value">{int(summary?.atRiskServices ?? 0)}</div>
          <div className="kpi-sub">из {int(summary?.totalServices ?? 0)}</div>
        </div>
      </section>

      <div className="grid" style={{ "--min": "380px" } as CSSProperties}>
        <section className="card">
          <div className="card-head">
            <h2>Клиенты по уровню риска</h2>
          </div>
          <div
            className="grid"
            style={{ "--min": "180px", alignItems: "center" } as CSSProperties}
          >
            <RiskRing counts={byRisk} total={total} />
            <ul
              className="stack"
              style={{ gap: 12, margin: 0, padding: 0, listStyle: "none" }}
            >
              {RISK_LEVELS.map((level) => (
                <li
                  key={level}
                  className={riskClass(level)}
                  style={{ display: "flex", alignItems: "center", gap: 10 }}
                >
                  <i className="dot" />
                  <span style={{ flex: 1 }}>{riskLabel[level]}</span>
                  <span className="num">{int(byRisk?.[level] ?? 0)}</span>
                  <span
                    className="muted num"
                    style={{ width: 44, textAlign: "right" }}
                  >
                    {percent(byRisk?.[level] ?? 0, total)}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="card">
          <div className="card-head">
            <h2>Прогноз непродлений</h2>
            <Dropdown
              label="Период"
              value={days}
              options={[...PERIODS]}
              onChange={setDays}
            />
          </div>
          <ChurnChart points={churn?.points ?? []} />
        </section>
      </div>

      <section className="card">
        <div className="card-head">
          <h2>Высокий риск, наибольшие потери</h2>
          <Link className="btn" to="/customers?risk=high">
            Все клиенты
          </Link>
        </div>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Email</th>
                <th>Риск</th>
                <th className="right">Score</th>
                <th>Главная причина</th>
                <th className="right">Потенциальные потери</th>
              </tr>
            </thead>
            <tbody>
              {top?.items.map((c) => (
                <tr
                  key={c.id}
                  data-href
                  onClick={() => navigate(`/customers/${c.id}`)}
                >
                  <td>
                    <Link to={`/customers/${c.id}`}>{c.email}</Link>
                  </td>
                  <td>
                    <RiskPill level={c.risk.level} />
                  </td>
                  <td className="right num">{score(c.risk.score)}</td>
                  <td className="muted">{c.risk.reasons[0]?.message ?? "—"}</td>
                  <td className="right num">{money(c.loss.amountCents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {top?.items.length === 0 && (
            <p className="empty">Клиентов с высоким риском нет</p>
          )}
        </div>
      </section>
    </div>
  );
}
