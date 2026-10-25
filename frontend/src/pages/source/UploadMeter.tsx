import type { UploadProgress } from "@/shared/api/http";
import { bytes } from "@/shared/lib/format";
import styles from "./SourcePage.module.css";

export type Sending = {
  stage: "receiving" | "processing";
  progress: UploadProgress;
  startedAt: number;
};

const seconds = (s: number) =>
  s < 60
    ? `${Math.ceil(s)} с`
    : `${Math.floor(s / 60)} мин ${Math.ceil(s % 60)} с`;

export function UploadMeter({ stage, progress, startedAt }: Sending) {
  const { loaded, total } = progress;
  const share = total ? Math.min(100, (loaded / total) * 100) : 0;
  const done = loaded >= total;
  const speed = loaded / Math.max((Date.now() - startedAt) / 1000, 0.001);

  const title =
    stage === "receiving"
      ? "Шаг 1 из 2 · отправляем файл"
      : done
        ? "Шаг 2 из 2 · сохраняем снимок"
        : "Шаг 2 из 2 · сервер разбирает снимок";
  const stats =
    stage === "receiving"
      ? `${bytes(loaded)} из ${bytes(total)} · ${bytes(speed)}/с · осталось ${seconds((total - loaded) / speed)}`
      : done
        ? "Проверка пройдена, записываем данные в базу"
        : `${bytes(loaded)} из ${bytes(total)} разобрано`;

  return (
    <div className={styles.meter} role="status" aria-live="polite">
      <span className={styles.stage}>{title}</span>
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
      <span className={styles.stats}>{stats}</span>
    </div>
  );
}
