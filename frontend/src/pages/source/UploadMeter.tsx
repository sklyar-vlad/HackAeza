import type { UploadProgress } from "@/shared/api/http";
import { bytes } from "@/shared/lib/format";
import styles from "./SourcePage.module.css";

const seconds = (s: number) =>
  s < 60
    ? `${Math.ceil(s)} с`
    : `${Math.floor(s / 60)} мин ${Math.ceil(s % 60)} с`;

export function UploadMeter({
  progress,
  startedAt,
}: {
  progress: UploadProgress;
  startedAt: number;
}) {
  const { loaded, total } = progress;
  const share = total ? Math.min(100, (loaded / total) * 100) : 0;
  const sent = loaded >= total;
  const speed = loaded / Math.max((Date.now() - startedAt) / 1000, 0.001);
  const left = speed ? (total - loaded) / speed : 0;

  return (
    <div className={styles.meter} role="status" aria-live="polite">
      <span className={styles.stage}>
        {sent ? "Сервер проверяет снимок" : "Отправляем файл"}
      </span>
      <strong className={`${styles.percent} num`}>{Math.floor(share)}%</strong>
      <div
        className={styles.track}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.floor(share)}
      >
        <i style={{ width: `${share}%` }} />
      </div>
      <span className={`${styles.stats} num`}>
        {sent
          ? `${bytes(total)} получено, ждём подтверждения`
          : `${bytes(loaded)} из ${bytes(total)} · ${bytes(speed)}/с · осталось ${seconds(left)}`}
      </span>
    </div>
  );
}
