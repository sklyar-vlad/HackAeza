import { useMutation } from "@tanstack/react-query";
import { useState, type CSSProperties } from "react";
import { JobNotice, useJob } from "@/entities/job";
import {
  replaceSourceSnapshot,
  useGetSourceStatus,
  type SnapshotResult,
  type SourceCounts,
  type SourceSnapshot,
} from "@/shared/api/generated/api";
import { ReplaceSourceSnapshotBody } from "@/shared/api/generated/zod";
import { ApiError } from "@/shared/api/http";
import { date, int } from "@/shared/lib/format";
import { Icon } from "@/shared/ui/Icon";
import styles from "./SourcePage.module.css";

type Picked = {
  name: string;
  size: number;
  snapshot?: SourceSnapshot;
  issues: string[];
};

const ENTITIES: { key: keyof SourceCounts; label: string }[] = [
  { key: "accounts", label: "Аккаунты" },
  { key: "services", label: "Услуги" },
  { key: "transactions", label: "Транзакции" },
];

async function readSnapshot(file: File): Promise<Picked> {
  const picked = { name: file.name, size: file.size, issues: [] as string[] };
  try {
    const parsed = ReplaceSourceSnapshotBody.safeParse(
      JSON.parse(await file.text()),
    );
    if (parsed.success)
      return { ...picked, snapshot: parsed.data as SourceSnapshot };
    return {
      ...picked,
      issues: parsed.error.issues
        .slice(0, 5)
        .map((i) => `${i.path.join(".")}: ${i.message}`),
    };
  } catch {
    return { ...picked, issues: ["Файл не является корректным JSON"] };
  }
}

export function SourcePage() {
  const { data: status } = useGetSourceStatus();
  const [picked, setPicked] = useState<Picked | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const { job, busy } = useJob(jobId ?? status?.pendingJobId);

  const upload = useMutation({
    mutationFn: (snapshot: SourceSnapshot) =>
      replaceSourceSnapshot(snapshot, {
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: (accepted) => setJobId(accepted.jobId),
  });

  const result =
    job?.status === "succeeded" && job.kind === "source_snapshot"
      ? (job.result as SnapshotResult)
      : null;
  const counts = result?.counts ?? status?.counts;
  const uploadError = upload.error instanceof ApiError ? upload.error : null;

  return (
    <div className="stack rise">
      <div className="page-head">
        <div>
          <h1>источник данных</h1>
          <p className="page-sub">
            Полный снимок аккаунтов, услуг и транзакций для пересчёта рисков
          </p>
        </div>
      </div>

      <section className="grid" style={{ "--min": "200px" } as CSSProperties}>
        {ENTITIES.map((e) => (
          <div key={e.key} className="card">
            <div className="kpi-label">{e.label}</div>
            <div className="kpi-value">{int(counts?.[e.key] ?? 0)}</div>
          </div>
        ))}
        <div className="card">
          <div className="kpi-label">Последняя загрузка</div>
          <div className="kpi-value" style={{ fontSize: 20 }}>
            {status?.snapshotPresent ? date(status.receivedAt) : "Нет данных"}
          </div>
          <div className="kpi-sub">снимок от {date(status?.capturedAt)}</div>
        </div>
      </section>

      <section className="card">
        <div className="card-head">
          <div>
            <h2>Загрузить снимок</h2>
            <p>
              JSON с массивами accounts, services и transactions, как
              testdata/data.json
            </p>
          </div>
        </div>
        <div className="grid" style={{ "--min": "300px" } as CSSProperties}>
          <label className={styles.drop}>
            <Icon name="upload" size={28} />
            <span>
              {picked
                ? picked.name
                : "Перетащите файл сюда или нажмите для выбора"}
            </span>
            <span className="muted">
              {picked
                ? `${(picked.size / 1_048_576).toFixed(1)} МБ`
                : "Формат .json"}
            </span>
            <input
              type="file"
              accept=".json,application/json"
              className="sr-only"
              onChange={async (e) => {
                const file = e.target.files?.[0];
                upload.reset();
                setPicked(file ? await readSnapshot(file) : null);
              }}
            />
          </label>
          <div className="stack">
            {picked?.issues.map((issue) => (
              <code key={issue} className="error-text">
                {issue}
              </code>
            ))}
            {picked?.snapshot && (
              <div className="notice">
                В файле: {int(picked.snapshot.accounts.length)} аккаунтов,{" "}
                {int(picked.snapshot.services.length)} услуг,{" "}
                {int(picked.snapshot.transactions.length)} транзакций
              </div>
            )}
            <div className="notice risk-medium">
              <i className="dot" style={{ marginTop: 6 }} />
              Текущие данные будут полностью заменены. Пустые массивы удалят
              соответствующие записи.
            </div>
            <button
              type="button"
              className="btn"
              disabled={!picked?.snapshot || busy || upload.isPending}
              onClick={() => picked?.snapshot && upload.mutate(picked.snapshot)}
            >
              {busy || upload.isPending
                ? "Загружаем и пересчитываем…"
                : "Загрузить и пересчитать"}
            </button>
            {uploadError && (
              <p className="error-text" role="alert">
                {uploadError.message}
                {uploadError.body?.error.details
                  ?.slice(0, 5)
                  .map((d) => ` · ${d.path}: ${d.message}`)}
              </p>
            )}
          </div>
        </div>
      </section>

      {job && (
        <section className="card">
          <div className="card-head">
            <h2>Результат загрузки</h2>
          </div>
          <div className="stack">
            <JobNotice job={job} />
            {result && (
              <div
                className="grid"
                style={{ "--min": "320px" } as CSSProperties}
              >
                <div>
                  {ENTITIES.map((e) => {
                    const sent = picked?.snapshot?.[e.key].length;
                    const ok = result.counts[e.key];
                    return (
                      <div key={e.key} className={styles.entity}>
                        <span>{e.label}</span>
                        <span className="num">
                          <strong>{int(ok)}</strong>
                          {sent !== undefined && (
                            <span className="muted"> из {int(sent)}</span>
                          )}
                        </span>
                      </div>
                    );
                  })}
                </div>
                <div>
                  <h3 className={styles.subhead}>
                    Предупреждения: {result.warnings.length}
                  </h3>
                  {result.warnings.map((w) => (
                    <div key={w.path + w.code} className={styles.warning}>
                      <code>{w.path}</code>
                      <span>{w.message}</span>
                    </div>
                  ))}
                  {!result.warnings.length && (
                    <p className="muted">Замечаний нет</p>
                  )}
                </div>
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  );
}
