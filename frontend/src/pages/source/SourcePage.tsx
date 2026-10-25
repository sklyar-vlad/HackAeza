import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import { JobNotice, useJob } from "@/entities/job";
import {
  useGetSourceStatus,
  type JobAccepted,
  type SnapshotResult,
  type SourceCounts,
  type SourceSnapshot,
} from "@/shared/api/generated/api";
import { ReplaceSourceSnapshotBody } from "@/shared/api/generated/zod";
import { ApiError, upload, type UploadProgress } from "@/shared/api/http";
import { bytes, date, dateOnly, int, time } from "@/shared/lib/format";
import { Icon } from "@/shared/ui/Icon";
import styles from "./SourcePage.module.css";
import { UploadMeter } from "./UploadMeter";

type Picked = {
  key: string;
  name: string;
  size: number;
  file?: File;
  snapshot?: SourceSnapshot;
  issues: string[];
};

const MAX_BYTES = 1 << 30;
const PREVIEW_BYTES = 10 << 20;

const ENTITIES: { key: keyof SourceCounts; label: string }[] = [
  { key: "accounts", label: "Аккаунты" },
  { key: "services", label: "Услуги" },
  { key: "transactions", label: "Транзакции" },
];

async function readSnapshot(file: File): Promise<Picked> {
  const picked = {
    key: crypto.randomUUID(),
    name: file.name,
    size: file.size,
    issues: [] as string[],
  };
  if (file.size > MAX_BYTES)
    return { ...picked, issues: ["Файл больше 1 ГиБ, сервер его не примет"] };
  if (file.size > PREVIEW_BYTES) return { ...picked, file };
  try {
    const parsed = ReplaceSourceSnapshotBody.safeParse(
      JSON.parse(await file.text()),
    );
    if (parsed.success)
      return { ...picked, file, snapshot: parsed.data as SourceSnapshot };
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
  const [dragging, setDragging] = useState(false);
  const [sending, setSending] = useState<{
    progress: UploadProgress;
    startedAt: number;
  } | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const { job, busy } = useJob(jobId ?? status?.pendingJobId);

  const send = useMutation({
    mutationFn: ({ file, key }: { file: File; key: string }) => {
      setSending({
        progress: { loaded: 0, total: file.size },
        startedAt: Date.now(),
      });
      return upload<JobAccepted>(
        "PUT",
        "/api/v1/source/snapshot",
        file,
        {
          "Content-Type": "application/json",
          "Idempotency-Key": key,
        },
        (progress) => setSending((s) => s && { ...s, progress }),
      );
    },
    onSettled: () => setSending(null),
    onSuccess: (accepted) => setJobId(accepted.jobId),
    meta: { inlineError: true },
  });

  const result =
    job?.status === "succeeded" && job.kind === "source_snapshot"
      ? (job.result as SnapshotResult)
      : null;
  const counts = result?.counts ?? status?.counts;
  const uploadError = send.error instanceof ApiError ? send.error : null;
  const pick = async (file: File | null | undefined) => {
    if (sending) return;
    send.reset();
    if (file === null)
      return setPicked({
        key: crypto.randomUUID(),
        name: "Файл не получен",
        size: 0,
        issues: [
          "Приложение, из которого перетащили файл, не передало его браузеру. Сохраните файл и перетащите из Finder или выберите кнопкой.",
        ],
      });
    setPicked(file ? await readSnapshot(file) : null);
  };
  const pickRef = useRef(pick);
  pickRef.current = pick;

  useEffect(() => {
    const over = (e: DragEvent) => {
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = "copy";
      setDragging(true);
    };
    const leave = (e: DragEvent) => {
      if (!e.relatedTarget) setDragging(false);
    };
    const drop = (e: DragEvent) => {
      e.preventDefault();
      setDragging(false);
      const items = [...(e.dataTransfer?.items ?? [])];
      void pickRef.current(
        e.dataTransfer?.files[0] ??
          items.find((i) => i.kind === "file")?.getAsFile() ??
          null,
      );
    };
    window.addEventListener("dragover", over, true);
    window.addEventListener("dragleave", leave, true);
    window.addEventListener("drop", drop, true);
    return () => {
      window.removeEventListener("dragover", over, true);
      window.removeEventListener("dragleave", leave, true);
      window.removeEventListener("drop", drop, true);
    };
  }, []);

  return (
    <div className="stack">
      <div className="page-head">
        <div>
          <h1>источник данных</h1>
          <p className="page-sub">
            Полный снимок аккаунтов, услуг и транзакций для пересчёта рисков
          </p>
        </div>
      </div>

      <section className={styles.kpis}>
        {ENTITIES.map((e) => (
          <div key={e.key} className="card">
            <div className="kpi-label">{e.label}</div>
            <div className="kpi-value">{int(counts?.[e.key] ?? 0)}</div>
          </div>
        ))}
        <div className="card">
          <div className="kpi-label">Последняя загрузка</div>
          <div className="kpi-value">
            {status?.snapshotPresent ? time(status.receivedAt) : "—"}
          </div>
          <div className="kpi-sub">
            {status?.snapshotPresent
              ? dateOnly(status.receivedAt)
              : "Нет данных"}
            {status?.capturedAt && ` · снимок от ${date(status.capturedAt)}`}
          </div>
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
          <label
            className={styles.drop}
            data-dragging={dragging || undefined}
            data-busy={sending ? true : undefined}
          >
            {sending ? (
              <UploadMeter {...sending} />
            ) : (
              <>
                <Icon name="upload" size={28} />
                <span>
                  {picked
                    ? picked.name
                    : "Перетащите файл сюда или нажмите для выбора"}
                </span>
                <span className="muted">
                  {picked?.size ? bytes(picked.size) : "Формат .json, до 1 ГБ"}
                </span>
              </>
            )}
            <input
              disabled={!!sending}
              type="file"
              accept=".json,application/json"
              className={styles.file}
              onChange={(e) => {
                void pick(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
          <div className="stack">
            {picked?.issues.map((issue) => (
              <code key={issue} className="error-text">
                {issue}
              </code>
            ))}
            {picked?.file && !picked.snapshot && (
              <p className={styles.hint}>
                <Icon name="info" size={16} />
                Файл больше 10 МБ, поэтому браузер не разбирает его заранее.
                Структуру и связи проверит сервер после загрузки.
              </p>
            )}
            {picked?.snapshot && (
              <div>
                <h3 className={styles.subhead}>В файле</h3>
                {ENTITIES.map((e) => (
                  <div key={e.key} className={styles.entity}>
                    <span>{e.label}</span>
                    <strong className="num">
                      {int(picked.snapshot![e.key].length)}
                    </strong>
                  </div>
                ))}
              </div>
            )}
            <button
              type="button"
              className="btn"
              disabled={
                !picked?.file ||
                picked.issues.length > 0 ||
                busy ||
                send.isPending
              }
              onClick={() =>
                picked?.file &&
                send.mutate({ file: picked.file, key: picked.key })
              }
            >
              {busy || send.isPending
                ? "Загружаем и пересчитываем…"
                : "Загрузить и пересчитать"}
            </button>
            <p className={styles.hint}>
              <Icon name="info" size={16} />
              Текущие данные заменятся целиком. Пустой массив удалит все записи
              этого типа.
            </p>
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
