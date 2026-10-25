import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { int, time } from "@/shared/lib/format";
import { Icon } from "@/shared/ui/Icon";
import {
  useGetCalculationJob,
  type CalculationJob,
} from "@/shared/api/generated/api";
import styles from "./JobNotice.module.css";

const active = (job?: CalculationJob) =>
  job?.status === "queued" || job?.status === "running";

/** Polls GET /jobs/{id} until succeeded/failed, then refreshes every published view. */
export function useJob(jobId: string | null | undefined) {
  const queryClient = useQueryClient();
  const { data: job } = useGetCalculationJob(jobId ?? "", {
    query: {
      enabled: Boolean(jobId),
      refetchInterval: (q) =>
        !q.state.data || active(q.state.data) ? 2000 : false,
    },
  });

  const finished = job?.status === "succeeded" || job?.status === "failed";
  useEffect(() => {
    if (finished)
      queryClient.invalidateQueries({
        predicate: (q) => !String(q.queryKey[0]).includes("/jobs/"),
      });
  }, [finished, queryClient]);

  return { job, busy: Boolean(jobId) && !finished };
}

const title = {
  queued: "В очереди на пересчёт",
  running: "Идёт пересчёт",
  succeeded: "Новая версия опубликована",
  failed: "Пересчёт не выполнен, прежние данные сохранены",
};

const scheduledTitle = {
  queued: "Плановый пересчёт по времени в очереди",
  running: "Идёт плановый пересчёт по времени",
  succeeded: "Плановый пересчёт по времени завершён",
  failed: "Плановый пересчёт не выполнен, прежние данные сохранены",
};

const icon = {
  queued: "clock",
  running: "clock",
  succeeded: "check",
  failed: "x",
} as const;

export function JobNotice({ job }: { job?: CalculationJob }) {
  if (!job) return null;
  const progress = job.progress;
  const share = progress?.totalCustomers
    ? (progress.processedCustomers / progress.totalCustomers) * 100
    : 0;

  return (
    <div className={styles.job} data-status={job.status} role="status">
      <span className={styles.icon}>
        <Icon name={icon[job.status]} size={18} />
      </span>
      <div className={styles.body}>
        <strong>
          {
            (job.kind === "time_recalculation" ? scheduledTitle : title)[
              job.status
            ]
          }
        </strong>
        <span className={styles.meta}>
          {job.status === "queued" &&
            `Ждёт обработчика с ${time(job.createdAt)}`}
          {job.status === "running" &&
            (progress
              ? `Обработано ${int(progress.processedCustomers)} из ${int(progress.totalCustomers)} клиентов`
              : `Начат в ${time(job.startedAt)}`)}
          {job.status === "succeeded" &&
            (progress
              ? `Пересчитано ${int(progress.processedCustomers)} из ${int(progress.totalCustomers)} клиентов · опубликовано в ${time(job.finishedAt)}`
              : `Опубликовано в ${time(job.finishedAt)}`)}
          {job.status === "failed" && job.error?.message}
        </span>
        {active(job) && progress && (
          <div className={`bar ${styles.progress}`}>
            <i style={{ width: `${share}%` }} />
          </div>
        )}
        {job.error?.details?.slice(0, 5).map((d) => (
          <code key={d.path} className={styles.meta}>
            {d.path}: {d.message}
          </code>
        ))}
      </div>
    </div>
  );
}
