import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useState, type CSSProperties } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { JobNotice, useJob } from "@/entities/job";
import { riskClass } from "@/entities/risk";
import {
  updateModelSettings,
  useGetModelSettings,
  useListModelFeatures,
  type ModelSettings,
  type ModelSettingsPatch,
} from "@/shared/api/generated/api";
import { UpdateModelSettingsBody } from "@/shared/api/generated/zod";
import { ApiError } from "@/shared/api/http";
import { date } from "@/shared/lib/format";

const body = UpdateModelSettingsBody.shape;
const Form = z
  .object({
    coefficients: body.coefficients.unwrap(),
    thresholds: body.thresholds
      .unwrap()
      .refine((t) => t.mediumFrom < t.highFrom, {
        message: "Средний порог должен быть ниже высокого",
      }),
    horizonDays: body.horizonDays.unwrap(),
    refreshIntervalSeconds: body.refreshIntervalSeconds.unwrap(),
  })
  .refine((v) => Object.values(v.coefficients).some((c) => c > 0), {
    message: "Хотя бы один признак должен быть включён",
    path: ["coefficients"],
  });
type Form = z.infer<typeof Form>;

const toForm = (s: ModelSettings): Form => ({
  coefficients: s.coefficients,
  thresholds: s.thresholds,
  horizonDays: s.horizonDays,
  refreshIntervalSeconds: s.refreshIntervalSeconds,
});

const conflictText: Record<string, string> = {
  VERSION_CONFLICT: "Настройки уже изменил кто-то другой. Обновите страницу.",
  JOB_IN_PROGRESS: "Уже идёт другой пересчёт, дождитесь его завершения.",
};

export function ModelPage() {
  const { data, isError } = useGetModelSettings();
  const { data: registry, isError: registryError } = useListModelFeatures();
  const [jobId, setJobId] = useState<string | null>(null);
  const { job, busy } = useJob(jobId ?? data?.pendingJobId);

  const form = useForm<Form>({
    resolver: zodResolver(Form),
    values: data && toForm(data.settings),
    resetOptions: { keepDirtyValues: true },
  });
  const { register, watch, setValue, handleSubmit, reset, formState } = form;
  const values = watch();

  const save = useMutation({
    mutationFn: (patch: ModelSettingsPatch) =>
      updateModelSettings(patch, {
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: (accepted) => setJobId(accepted.jobId),
    meta: { inlineError: true },
  });

  if (!data || !registry)
    return (
      <p className="empty">
        {isError || registryError
          ? "Не удалось загрузить настройки модели"
          : "Загрузка…"}
      </p>
    );
  const settings = data.settings;

  const submit = handleSubmit((v) => {
    const dirty = formState.dirtyFields;
    const changed = Object.keys(dirty.coefficients ?? {});
    save.mutate({
      expectedVersion: settings.version,
      coefficients: changed.length
        ? Object.fromEntries(changed.map((k) => [k, v.coefficients[k]]))
        : undefined,
      thresholds: dirty.thresholds ? v.thresholds : undefined,
      horizonDays: dirty.horizonDays ? v.horizonDays : undefined,
      refreshIntervalSeconds: dirty.refreshIntervalSeconds
        ? v.refreshIntervalSeconds
        : undefined,
    });
  });

  const coef = (key: string) => values.coefficients?.[key] ?? 0;
  const setCoef = (key: string, value: number) =>
    setValue(`coefficients.${key}`, Math.round(value * 100) / 100, {
      shouldDirty: true,
      shouldValidate: true,
    });
  const { mediumFrom, highFrom } = values.thresholds ?? settings.thresholds;
  const active = registry.items.filter((f) => coef(f.key) > 0).length;
  const saveError = save.error instanceof ApiError ? save.error : null;

  return (
    <form className="stack rise" onSubmit={submit}>
      <div className="page-head">
        <div>
          <h1>настройки модели</h1>
          <p className="page-sub">
            Веса признаков и пороги уровней. Изменения применяются фоновым
            пересчётом.
          </p>
        </div>
      </div>

      <section className="grid" style={{ "--min": "220px" } as CSSProperties}>
        <div className="card">
          <div className="kpi-label">Версия модели</div>
          <div className="kpi-value">v{settings.version}</div>
          <div className="kpi-sub">обновлена {date(settings.updatedAt)}</div>
        </div>
        <div className="card">
          <div className="kpi-label">Включено признаков</div>
          <div className="kpi-value">{active}</div>
          <div className="kpi-sub">из {registry.items.length}</div>
        </div>
        <div className="card">
          <div className="kpi-label">Версия расчёта</div>
          <div className="kpi-value">{data.calculation.version}</div>
          <div className="kpi-sub">
            от {date(data.calculation.calculatedAt)}
          </div>
        </div>
      </section>

      <div className="grid" style={{ "--min": "380px" } as CSSProperties}>
        <section className="card">
          <div className="card-head">
            <div>
              <h2>Признаки риска</h2>
              <p>
                Ноль отключает признак. Score = 100 × Σ(вес × значение) / Σ веса
              </p>
            </div>
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() =>
                registry.items.forEach((f) =>
                  setCoef(f.key, f.defaultCoefficient),
                )
              }
            >
              По умолчанию
            </button>
          </div>
          <div className="stack" style={{ gap: 0 }}>
            {registry.items.map((f) => (
              <div
                key={f.key}
                style={{
                  padding: "14px 0",
                  borderTop: "1px solid var(--line)",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                  <label htmlFor={f.key} style={{ flex: 1, fontWeight: 600 }}>
                    {f.name}
                  </label>
                  <span className="num">{coef(f.key).toFixed(2)}</span>
                  <button
                    type="button"
                    role="switch"
                    className="switch"
                    aria-checked={coef(f.key) > 0}
                    aria-label={`Признак «${f.name}»`}
                    onClick={() =>
                      setCoef(
                        f.key,
                        coef(f.key) > 0 ? 0 : f.defaultCoefficient || 0.1,
                      )
                    }
                  />
                </div>
                <p className="muted" style={{ margin: "4px 0 8px" }}>
                  {f.description}
                </p>
                <input
                  id={f.key}
                  type="range"
                  className="range"
                  min={0}
                  max={1}
                  step={0.05}
                  value={coef(f.key)}
                  onChange={(e) => setCoef(f.key, Number(e.target.value))}
                  style={
                    {
                      "--p": `${coef(f.key) * 100}%`,
                      "--k": coef(f.key) ? "var(--accent)" : "var(--unknown)",
                    } as CSSProperties
                  }
                />
              </div>
            ))}
          </div>
          {formState.errors.coefficients && (
            <p className="error-text">
              Хотя бы один признак должен быть включён
            </p>
          )}
        </section>

        <div className="stack">
          <section className="card">
            <div className="card-head">
              <div>
                <h2>Пороги уровней риска</h2>
                <p>С какого score начинается уровень</p>
              </div>
            </div>
            {(
              [
                ["high", "Высокий", "thresholds.highFrom", highFrom, 100],
                [
                  "medium",
                  "Средний",
                  "thresholds.mediumFrom",
                  mediumFrom,
                  highFrom,
                ],
              ] as const
            ).map(([level, label, name, from, to]) => (
              <div
                key={level}
                className={riskClass(level)}
                style={{
                  padding: "12px 0",
                  borderTop: "1px solid var(--line)",
                }}
              >
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 10,
                    marginBottom: 6,
                  }}
                >
                  <i className="dot" />
                  <label htmlFor={name} style={{ flex: 1 }}>
                    {label}
                  </label>
                  <span className="num">
                    {from} – {to}
                  </span>
                </div>
                <input
                  id={name}
                  type="range"
                  className="range span"
                  min={0}
                  max={100}
                  {...register(name, { valueAsNumber: true })}
                  style={
                    {
                      "--k": "var(--c)",
                      "--a": `${from}%`,
                      "--b": `${to}%`,
                    } as CSSProperties
                  }
                />
              </div>
            ))}
            <div
              className={riskClass("low")}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 10,
                paddingTop: 12,
                borderTop: "1px solid var(--line)",
              }}
            >
              <i className="dot" />
              <span style={{ flex: 1 }}>Низкий</span>
              <span className="num">0 – {mediumFrom}</span>
            </div>
            {formState.errors.thresholds && (
              <p className="error-text">
                {formState.errors.thresholds.message}
              </p>
            )}
          </section>

          <section className="card stack">
            <div className="card-head" style={{ marginBottom: 0 }}>
              <h2>Расчёт</h2>
            </div>
            <label className="stack" style={{ gap: 8 }}>
              Горизонт оценки потерь, дней
              <input
                type="number"
                min={1}
                className="field"
                {...register("horizonDays", { valueAsNumber: true })}
              />
            </label>
            <label className="stack" style={{ gap: 8 }}>
              Интервал пересчёта по времени, секунд
              <input
                type="number"
                min={1}
                className="field"
                {...register("refreshIntervalSeconds", { valueAsNumber: true })}
              />
            </label>
          </section>
        </div>
      </div>

      <JobNotice job={job} />
      {saveError && (
        <p className="error-text" role="alert">
          {conflictText[saveError.body?.error.code ?? ""] ?? saveError.message}
        </p>
      )}

      <div
        className="card"
        style={{
          display: "flex",
          flexWrap: "wrap",
          justifyContent: "flex-end",
          gap: 12,
        }}
      >
        <button
          type="button"
          className="btn btn-ghost"
          disabled={!formState.isDirty}
          onClick={() => reset(toForm(settings))}
        >
          Отменить изменения
        </button>
        <button
          type="submit"
          className="btn"
          disabled={!formState.isDirty || busy || save.isPending}
        >
          {busy || save.isPending
            ? "Пересчитываем…"
            : "Сохранить и пересчитать"}
        </button>
      </div>
    </form>
  );
}
