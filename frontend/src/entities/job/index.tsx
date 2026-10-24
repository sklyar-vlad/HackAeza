import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import {
  useGetCalculationJob,
  type CalculationJob,
} from "@/shared/api/generated/api";

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

const statusText = {
  queued: "Задание в очереди",
  running: "Идёт пересчёт",
  succeeded: "Новая версия опубликована",
  failed: "Пересчёт не выполнен, прежние данные сохранены",
};

export function JobNotice({ job }: { job?: CalculationJob }) {
  if (!job) return null;
  const tone =
    job.status === "failed"
      ? "risk-high"
      : job.status === "succeeded"
        ? "risk-low"
        : "risk-medium";
  const progress = job.progress;

  return (
    <div className={`notice ${tone}`} role="status">
      <i className="dot" style={{ marginTop: 6 }} />
      <div className="stack" style={{ gap: 4, flex: 1 }}>
        <strong>{statusText[job.status]}</strong>
        {progress && active(job) && (
          <span className="muted num">
            Обработано {progress.processedCustomers} из{" "}
            {progress.totalCustomers} клиентов
          </span>
        )}
        {job.error && <span>{job.error.message}</span>}
        {job.error?.details?.slice(0, 5).map((d) => (
          <code key={d.path} className="muted">
            {d.path}: {d.message}
          </code>
        ))}
      </div>
    </div>
  );
}
