import { useState } from "react";
import type { ChurnPoint } from "@/shared/api/generated/api";
import { int, shortDay } from "@/shared/lib/format";

const W = 440;
const H = 218;
const PAD = { left: 52, right: 12, top: 12, bottom: 30 };

const niceMax = (v: number) => {
  const step = 10 ** Math.floor(Math.log10(Math.max(v, 1)));
  return Math.ceil(v / step) * step || 1;
};

/** Daily scenario of non-renewals (sum of score/100); days outside the horizon are null and left as gaps. */
export function ChurnChart({ points }: { points: ChurnPoint[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const values = points.map((p) => p.forecastNonRenewals);
  const known = values.filter((v): v is number => v != null);
  if (!known.length)
    return <p className="empty">Нет данных для прогноза на выбранный период</p>;

  const max = niceMax(Math.max(...known));
  const x = (i: number) =>
    PAD.left +
    (i / Math.max(1, points.length - 1)) * (W - PAD.left - PAD.right);
  const y = (v: number) => PAD.top + (1 - v / max) * (H - PAD.top - PAD.bottom);
  const bottom = y(0);

  const runs: { i: number; v: number }[][] = [];
  values.forEach((v, i) => {
    if (v == null) return;
    if (values[i - 1] == null) runs.push([]);
    runs[runs.length - 1].push({ i, v });
  });
  const curve = (run: { i: number; v: number }[]) =>
    run
      .map(({ i, v }, k) => {
        if (!k) return `M${x(i)} ${y(v)}`;
        const prev = run[k - 1];
        const mid = (x(prev.i) + x(i)) / 2;
        return `C${mid} ${y(prev.v)} ${mid} ${y(v)} ${x(i)} ${y(v)}`;
      })
      .join("");
  const line = runs.map(curve).join("");
  const area = runs
    .map(
      (run) =>
        `${curve(run)}L${x(run[run.length - 1].i)} ${bottom}L${x(run[0].i)} ${bottom}Z`,
    )
    .join("");

  const ticks = [0, 0.25, 0.5, 0.75, 1].map((t) => t * max);
  const step = Math.max(1, Math.ceil(points.length / 5));
  const labels = points
    .map((p, i) => ({ p, i }))
    .filter(({ i }) => i % step === 0 || i === points.length - 1);
  const total = known.reduce((a, b) => a + b, 0);
  const peak = values.indexOf(Math.max(...known));

  const point = hover != null && values[hover] != null ? hover : null;
  const tipX =
    point == null
      ? 0
      : Math.min(Math.max(x(point) - 49, PAD.left + 4), W - PAD.right - 98);

  return (
    <figure style={{ margin: 0 }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        style={{ width: "100%", height: "auto", display: "block" }}
        role="img"
        aria-label={`Ожидается около ${int(Math.round(total))} непродлений за период`}
        onPointerMove={(e) => {
          const box = e.currentTarget.getBoundingClientRect();
          const px = ((e.clientX - box.left) / box.width) * W;
          const i = Math.round(
            ((px - PAD.left) / (W - PAD.left - PAD.right)) *
              (points.length - 1),
          );
          setHover(Math.min(points.length - 1, Math.max(0, i)));
        }}
        onPointerLeave={() => setHover(null)}
      >
        <defs>
          <linearGradient id="churnArea" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="var(--accent)" stopOpacity="0.28" />
            <stop offset="1" stopColor="var(--accent)" stopOpacity="0" />
          </linearGradient>
        </defs>
        {ticks.map((t) => (
          <g key={t}>
            <line
              x1={PAD.left}
              x2={W - PAD.right}
              y1={y(t)}
              y2={y(t)}
              stroke="var(--line)"
            />
            <text
              x={PAD.left - 8}
              y={y(t) + 4}
              textAnchor="end"
              fontSize="11"
              fill="var(--mute)"
            >
              {int(Math.round(t))}
            </text>
          </g>
        ))}
        {labels.map(({ p, i }) => (
          <text
            key={p.date}
            x={x(i)}
            y={H - 8}
            textAnchor={
              i === 0 ? "start" : i === points.length - 1 ? "end" : "middle"
            }
            fontSize="11"
            fill="var(--mute)"
          >
            {shortDay(p.date)}
          </text>
        ))}
        <path d={area} fill="url(#churnArea)" />
        <path
          d={line}
          fill="none"
          stroke="var(--accent)"
          strokeWidth="2.5"
          strokeLinejoin="round"
          strokeLinecap="round"
        />
        {point != null && (
          <g pointerEvents="none">
            <path
              d={`M${x(point)} ${y(values[point]!)} V${bottom}`}
              stroke="var(--text)"
              strokeDasharray="3 3"
            />
            <circle
              cx={x(point)}
              cy={y(values[point]!)}
              r="5"
              fill="var(--card)"
              stroke="var(--text)"
              strokeWidth="2.5"
            />
            <rect
              x={tipX}
              y="4"
              width="98"
              height="44"
              rx="10"
              fill="var(--soft)"
            />
            <text x={tipX + 10} y="21" fontSize="11" fill="var(--mute)">
              {shortDay(points[point].date)}
            </text>
            <text
              x={tipX + 10}
              y="39"
              fontSize="12"
              fontWeight="700"
              fill="var(--text)"
            >
              {int(Math.round(values[point]!))}
            </text>
          </g>
        )}
      </svg>
      <dl
        style={{
          margin: "16px 0 0",
          display: "grid",
          gridTemplateColumns: "repeat(3, minmax(0, 1fr))",
          gap: 12,
        }}
      >
        {[
          ["За период", int(Math.round(total))],
          [
            "Пик",
            `${int(Math.round(known.length ? values[peak]! : 0))} · ${shortDay(points[peak].date)}`,
          ],
          ["В среднем за день", int(Math.round(total / known.length))],
        ].map(([label, value]) => (
          <div
            key={label}
            style={{
              minWidth: 0,
              padding: 12,
              borderRadius: 12,
              background: "var(--sheet)",
            }}
          >
            <dt className="muted" style={{ fontSize: 12 }}>
              {label}
            </dt>
            <dd
              className="num"
              style={{
                margin: "4px 0 0",
                fontSize: 16,
                fontWeight: 700,
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </figure>
  );
}
