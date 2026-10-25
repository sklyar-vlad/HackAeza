import { keepPreviousData } from "@tanstack/react-query";
import { useDeferredValue } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { RISK_LEVELS, riskClass, riskLabel } from "@/entities/risk";
import {
  useGetAnalyticsSummary,
  useListCustomers,
  type ListCustomersSortBy,
  type RiskLevel,
} from "@/shared/api/generated/api";
import { int, money, score } from "@/shared/lib/format";
import { Dropdown } from "@/shared/ui/Dropdown";
import { Icon } from "@/shared/ui/Icon";

const PAGE_SIZE = 20;

const COLUMNS: { key: ListCustomersSortBy; label: string; right?: boolean }[] =
  [
    { key: "id", label: "ID" },
    { key: "email", label: "Email" },
    { key: "balance", label: "Баланс", right: true },
    { key: "totalReplenished", label: "Пополнения", right: true },
    { key: "score", label: "Score", right: true },
  ];

export function CustomersPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const search = useDeferredValue(params.get("q") ?? "");
  const risk = (params.get("risk") ?? "all") as RiskLevel | "all";
  const sortBy = (params.get("sort") ?? "score") as ListCustomersSortBy;
  const sortOrder = params.get("order") === "asc" ? "asc" : "desc";
  const page = Number(params.get("page") ?? 1);

  const set = (patch: Record<string, string | null>) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      for (const [key, value] of Object.entries(patch))
        value ? next.set(key, value) : next.delete(key);
      if (!("page" in patch)) next.delete("page");
      return next;
    });

  const { data } = useListCustomers(
    {
      page,
      pageSize: PAGE_SIZE,
      search: search || undefined,
      riskLevel: risk === "all" ? undefined : [risk],
      sortBy,
      sortOrder,
    },
    { query: { placeholderData: keepPreviousData } },
  );
  const { data: summary } = useGetAnalyticsSummary();
  const total = data?.pagination.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const ariaSort = (key: ListCustomersSortBy) =>
    sortBy === key
      ? sortOrder === "asc"
        ? "ascending"
        : "descending"
      : undefined;
  const sortHeader = (key: ListCustomersSortBy) => ({
    onClick: () =>
      set({
        sort: key,
        order: sortBy === key && sortOrder === "desc" ? "asc" : "desc",
      }),
  });

  return (
    <div className="stack">
      <div className="page-head">
        <div>
          <h1>клиенты</h1>
          <p className="page-sub">
            Risk-score, балансы и причины риска по каждому аккаунту
          </p>
        </div>
      </div>

      <section className="card">
        <div className="card-head" style={{ justifyContent: "flex-start" }}>
          <div
            style={{ position: "relative", flex: "1 1 280px", maxWidth: 520 }}
          >
            <Icon
              name="search"
              size={18}
              className="muted"
              style={{
                position: "absolute",
                left: 14,
                top: 13,
                pointerEvents: "none",
              }}
            />
            <input
              type="search"
              className="field"
              style={{ paddingLeft: 42 }}
              placeholder="Поиск по email или ID"
              aria-label="Поиск клиентов"
              value={params.get("q") ?? ""}
              onChange={(e) => set({ q: e.target.value })}
            />
          </div>
          <Dropdown
            label="Риск"
            value={risk}
            onChange={(value) => set({ risk: value === "all" ? null : value })}
            options={[
              {
                value: "all",
                label: "Все уровни",
                count: summary?.totalCustomers,
              },
              ...RISK_LEVELS.map((level) => ({
                value: level,
                label: riskLabel[level],
                count: summary?.customersByRisk[level],
                dotClass: riskClass(level),
              })),
            ]}
          />
        </div>

        <div className="table-wrap">
          <table className="table fixed">
            <colgroup>
              <col style={{ width: 70 }} />
              <col style={{ width: "22%" }} />
              <col style={{ width: 120 }} />
              <col style={{ width: 120 }} />
              <col style={{ width: 80 }} />
              <col />
              <col style={{ width: 140 }} />
            </colgroup>
            <thead>
              <tr>
                {COLUMNS.map((c) => (
                  <th
                    key={c.key}
                    className={c.right ? "right" : undefined}
                    aria-sort={ariaSort(c.key)}
                  >
                    <button
                      type="button"
                      className="sort"
                      {...sortHeader(c.key)}
                    >
                      {c.label}
                      {sortBy === c.key && (
                        <Icon
                          name="chevron"
                          size={14}
                          style={{
                            rotate: sortOrder === "asc" ? "180deg" : "0deg",
                          }}
                        />
                      )}
                    </button>
                  </th>
                ))}
                <th>Причины</th>
                <th className="right" aria-sort={ariaSort("potentialLoss")}>
                  <button
                    type="button"
                    className="sort"
                    {...sortHeader("potentialLoss")}
                  >
                    Потенциальные потери
                  </button>
                </th>
              </tr>
            </thead>
            <tbody>
              {data?.items.map((c) => (
                <tr
                  key={c.id}
                  data-href
                  onClick={() => navigate(`/customers/${c.id}`)}
                >
                  <td className="num">{c.id}</td>
                  <td className="clip" title={c.email}>
                    <Link to={`/customers/${c.id}`}>{c.email}</Link>
                  </td>
                  <td className="right num">{money(c.balance)}</td>
                  <td className="right num">{money(c.totalReplenished)}</td>
                  <td className={`right num ${riskClass(c.risk.level)}`}>
                    <i className="dot" style={{ marginRight: 8 }} />
                    {score(c.risk.score)}
                  </td>
                  <td
                    className="muted clip"
                    title={c.risk.reasons.map((r) => r.message).join("; ")}
                  >
                    {c.risk.reasons.map((r) => r.message).join("; ") || "—"}
                  </td>
                  <td className="right num">{money(c.loss.amountCents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {data?.items.length === 0 && (
            <p className="empty">Ничего не найдено</p>
          )}
        </div>

        <div className="pager">
          <span className="num">
            {total
              ? `${int((page - 1) * PAGE_SIZE + 1)}–${int(Math.min(page * PAGE_SIZE, total))} из ${int(total)}`
              : "0"}
          </span>
          <div style={{ display: "flex", gap: 8 }}>
            <button
              type="button"
              className="btn btn-icon"
              aria-label="Предыдущая страница"
              disabled={page <= 1}
              onClick={() => set({ page: String(page - 1) })}
            >
              <Icon name="back" size={18} />
            </button>
            <button
              type="button"
              className="btn btn-icon"
              aria-label="Следующая страница"
              disabled={page >= pages}
              onClick={() => set({ page: String(page + 1) })}
            >
              <Icon name="next" size={18} />
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
