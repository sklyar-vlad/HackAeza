import type { RiskLevel } from "@/shared/api/generated/api";

export const RISK_LEVELS = [
  "high",
  "medium",
  "low",
  "unknown",
] as const satisfies RiskLevel[];

export const riskLabel: Record<RiskLevel, string> = {
  high: "Высокий",
  medium: "Средний",
  low: "Низкий",
  unknown: "Без оценки",
};

export const riskClass = (level: RiskLevel) => `risk-${level}`;

export function RiskPill({ level }: { level: RiskLevel }) {
  return (
    <span className={`pill ${riskClass(level)}`}>
      <i className="dot" />
      {riskLabel[level]}
    </span>
  );
}
