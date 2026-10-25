import { RISK_LEVELS, riskLabel } from "@/entities/risk";
import type { RiskCounts } from "@/shared/api/generated/api";
import { int } from "@/shared/lib/format";

const R = 70;
const C = 2 * Math.PI * R;
const STROKE = 36;

export function RiskRing({
  counts,
  total,
}: {
  counts?: RiskCounts;
  total: number;
}) {
  const filled = RISK_LEVELS.filter((l) => (counts?.[l] ?? 0) > 0).length;
  const gap = filled > 1 ? 2 : 0;
  let offset = 0;
  const segments = RISK_LEVELS.map((level) => {
    const share = total ? (counts?.[level] ?? 0) / total : 0;
    const start = offset;
    offset += share;
    return { level, share, start };
  }).filter((s) => s.share > 0);
  const label = RISK_LEVELS.map(
    (l) => `${riskLabel[l]}: ${counts?.[l] ?? 0}`,
  ).join(", ");

  return (
    <div
      style={{
        position: "relative",
        width: 220,
        height: 220,
        flex: "none",
        margin: "0 auto",
      }}
    >
      <svg
        width="220"
        height="220"
        viewBox="-15 -15 230 230"
        shapeRendering="geometricPrecision"
        role="img"
        aria-label={label}
      >
        {segments.length === 0 && (
          <circle
            cx="100"
            cy="100"
            r={R}
            fill="none"
            stroke="var(--track)"
            strokeWidth={STROKE}
          />
        )}
        <g transform="rotate(-90 100 100)">
          {segments.map(({ level, share, start }) => {
            const length = Math.max(share * C - gap, 1);
            return (
              <circle
                key={level}
                cx="100"
                cy="100"
                r={R}
                fill="none"
                stroke={`var(--${level})`}
                strokeWidth={STROKE}
                strokeDasharray={`${length} ${C - length}`}
                strokeDashoffset={-start * C}
              />
            );
          })}
        </g>
        {segments
          .filter(({ share }) => share >= 0.01)
          .map(({ level, share, start }) => {
            const angle = (start + share / 2) * 2 * Math.PI - Math.PI / 2;
            const inside = share >= 0.06;
            const r = inside ? R : R + STROKE / 2 + 10;
            return (
              <text
                key={level}
                x={100 + r * Math.cos(angle)}
                y={100 + r * Math.sin(angle) + 4}
                fontSize="11"
                fontWeight={inside ? 700 : 400}
                fill={inside ? "#000" : "var(--mute)"}
                textAnchor="middle"
              >
                {Math.round(share * 100)}%
              </text>
            );
          })}
      </svg>
      <div
        style={{
          position: "absolute",
          inset: 0,
          display: "grid",
          placeContent: "center",
          textAlign: "center",
          pointerEvents: "none",
        }}
      >
        <strong style={{ fontSize: 20, lineHeight: 1.1 }} className="num">
          {int(total)}
        </strong>
        <span className="muted">клиентов</span>
      </div>
    </div>
  );
}
