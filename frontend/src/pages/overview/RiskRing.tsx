import { RISK_LEVELS, riskLabel } from "@/entities/risk";
import type { RiskCounts } from "@/shared/api/generated/api";
import { int } from "@/shared/lib/format";

const R = 70;
const C = 2 * Math.PI * R;

export function RiskRing({
  counts,
  total,
}: {
  counts?: RiskCounts;
  total: number;
}) {
  let offset = 0;
  const label = RISK_LEVELS.map(
    (l) => `${riskLabel[l]}: ${counts?.[l] ?? 0}`,
  ).join(", ");

  return (
    <div
      style={{
        position: "relative",
        width: 200,
        height: 200,
        margin: "0 auto",
      }}
    >
      <svg
        width="200"
        height="200"
        viewBox="0 0 200 200"
        role="img"
        aria-label={label}
      >
        <circle
          cx="100"
          cy="100"
          r={R}
          fill="none"
          stroke="var(--track)"
          strokeWidth="18"
        />
        {total > 0 &&
          RISK_LEVELS.map((level) => {
            const length = ((counts?.[level] ?? 0) / total) * C;
            const segment = (
              <circle
                key={level}
                cx="100"
                cy="100"
                r={R}
                fill="none"
                stroke={`var(--${level})`}
                strokeWidth="18"
                strokeDasharray={`${length} ${C - length}`}
                strokeDashoffset={-offset}
                transform="rotate(-90 100 100)"
                style={{ transition: "stroke-dasharray 0.6s var(--ease)" }}
              />
            );
            offset += length;
            return segment;
          })}
      </svg>
      <div
        style={{
          position: "absolute",
          inset: 0,
          display: "grid",
          placeContent: "center",
          textAlign: "center",
        }}
      >
        <strong style={{ fontSize: 24 }} className="num">
          {int(total)}
        </strong>
        <span className="muted">клиентов</span>
      </div>
    </div>
  );
}
