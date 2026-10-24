import type { ChurnPoint } from "@/shared/api/generated/api";
import { shortDay } from "@/shared/lib/format";

const W = 440;
const H = 200;
const PAD = { left: 36, right: 8, top: 12, bottom: 24 };

/** Daily scenario of non-renewals (sum of score/100); days outside the horizon are null and left as gaps. */
export function ChurnChart({ points }: { points: ChurnPoint[] }) {
  const values = points.map((p) => p.forecastNonRenewals);
  const known = values.filter((v): v is number => v != null);
  if (!known.length)
    return <p className="empty">Прогноз появится после первого расчёта</p>;

  const max = Math.max(1, Math.ceil(Math.max(...known)));
  const x = (i: number) =>
    PAD.left +
    (i / Math.max(1, points.length - 1)) * (W - PAD.left - PAD.right);
  const y = (v: number) => PAD.top + (1 - v / max) * (H - PAD.top - PAD.bottom);

  let line = "";
  values.forEach((v, i) => {
    if (v == null) return;
    line += `${line && values[i - 1] != null ? "L" : "M"}${x(i).toFixed(1)} ${y(v).toFixed(1)}`;
  });
  const ticks = [0, max / 2, max];
  const labels = points.filter(
    (_, i) => i % Math.ceil(points.length / 5) === 0,
  );
  const total = known.reduce((a, b) => a + b, 0);

  return (
    <figure style={{ margin: 0 }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        style={{ width: "100%", height: "auto", display: "block" }}
        role="img"
        aria-label={`Ожидается около ${Math.round(total)} непродлений за период`}
      >
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
              {Math.round(t * 10) / 10}
            </text>
          </g>
        ))}
        {labels.map((p) => (
          <text
            key={p.date}
            x={x(points.indexOf(p))}
            y={H - 6}
            textAnchor="middle"
            fontSize="11"
            fill="var(--mute)"
          >
            {shortDay(p.date)}
          </text>
        ))}
        <path
          d={line}
          fill="none"
          stroke="var(--accent)"
          strokeWidth="2.5"
          strokeLinejoin="round"
        />
      </svg>
      <figcaption className="muted" style={{ marginTop: 8 }}>
        Сценарий: около <strong className="num">{Math.round(total)}</strong>{" "}
        непродлений за период
      </figcaption>
    </figure>
  );
}
