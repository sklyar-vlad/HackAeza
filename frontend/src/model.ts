export type Level = "low" | "medium" | "high" | "unknown";
export type Account = {
  id: number;
  email: string;
  balance: number;
  withdrawBalance: number;
  bonusBalance: number;
  bonusState: "locked" | "unlocked";
  totalReplenished: number;
};
export type Service = {
  id: number;
  ownerId: number;
  name: string;
  paymentTerm:
    "hour" | "week" | "month" | "quarter_year" | "half_year" | "year";
  autoProlong: boolean;
  createdAt: string;
  expiresAt: string;
  status: "active" | "suspended" | "prolong_wait" | "deleted" | "blocked";
  price: number;
};
export type Transaction = {
  id: number;
  serviceId: number;
  amount: number;
  status: "performed" | "not_performed";
  type: "order" | "prolong";
  createdAt: string;
  performedAt: string | null;
};
export type Snapshot = {
  accounts: Account[];
  services: Service[];
  transactions: Transaction[];
  capturedAt?: string;
};
export const factors = [
  {
    key: "renewalWithoutAutoProlong",
    label: "Отключено автопродление",
    description: "Ближайшая оплата на горизонте при выключенном автопродлении.",
    weight: 40,
  },
  {
    key: "insufficientRenewalFunds",
    label: "Недостаточно средств",
    description:
      "Доля стоимости ближайших продлений, не покрытая доступным балансом.",
    weight: 35,
  },
  {
    key: "paymentWaiting",
    label: "Ожидание продления",
    description:
      "Услуга находится в prolong_wait в пределах пяти дней после окончания.",
    weight: 25,
  },
] as const;
export type FactorKey = (typeof factors)[number]["key"];
export type Settings = {
  weights: Record<FactorKey, number>;
  medium: number;
  high: number;
  horizon: number;
};
export const defaults: Settings = {
  weights: {
    renewalWithoutAutoProlong: 40,
    insufficientRenewalFunds: 35,
    paymentWaiting: 25,
  },
  medium: 35,
  high: 70,
  horizon: 7,
};
export const levelLabels: Record<Level, string> = {
  low: "Низкий",
  medium: "Средний",
  high: "Высокий",
  unknown: "Без оценки",
};
export const statusLabels: Record<Service["status"], string> = {
  active: "Активна",
  suspended: "Приостановлена",
  prolong_wait: "Ожидает продления",
  blocked: "Заблокирована",
  deleted: "Удалена",
};
export const termLabels: Record<Service["paymentTerm"], string> = {
  hour: "час",
  week: "неделя",
  month: "месяц",
  quarter_year: "квартал",
  half_year: "полгода",
  year: "год",
};
export type Issue = { path: string; message: string };
export type ServiceResult = Service & {
  score: number | null;
  level: Level;
  eligible: boolean;
  uncertain: boolean;
  reason: string;
  contributions: number[];
  values: number[];
  loss: bigint | null;
};
export type Customer = Account & {
  services: ServiceResult[];
  score: number | null;
  level: Level;
  loss: bigint | null;
  revenue: bigint;
  unassessed: bigint;
  partial: boolean;
  reasons: string[];
  contributions: number[];
};
const DAY = 86_400_000;
const safe = (v: unknown): v is number =>
  typeof v === "number" && Number.isSafeInteger(v);
const round = (num: bigint, den: bigint) => (num * 2n + den) / (den * 2n);
export function money(value: number | bigint | null): string {
  if (value === null) return "—";
  const n = BigInt(value);
  return `${new Intl.NumberFormat("ru-RU").format(n / 100n)},${(n % 100n).toString().padStart(2, "0")} €`;
}
export const date = (s: string | null, time = true) =>
  s
    ? new Intl.DateTimeFormat("ru-RU", {
        day: "numeric",
        month: "short",
        year: "numeric",
        ...(time ? { hour: "2-digit", minute: "2-digit" } : {}),
      }).format(new Date(s))
    : "Не указана";
export function level(score: number | null, settings: Settings): Level {
  return score === null
    ? "unknown"
    : score >= settings.high
      ? "high"
      : score >= settings.medium
        ? "medium"
        : "low";
}
export function settingsError(s: Settings): string | null {
  if (
    !Object.values(s.weights).every(
      (x) => Number.isFinite(x) && x >= 0 && x <= 1000,
    )
  )
    return "Коэффициенты должны быть от 0 до 1000.";
  if (
    Object.values(s.weights).some(
      (x) => Math.abs(x * 100 - Math.round(x * 100)) > 0.000001,
    )
  )
    return "В макете коэффициенты задаются с точностью до двух знаков после запятой.";
  if (!Object.values(s.weights).some((x) => x > 0))
    return "Включите хотя бы один фактор.";
  if (!(
    Number.isFinite(s.medium) &&
    Number.isFinite(s.high) &&
    s.medium >= 0 &&
    s.medium < s.high &&
    s.high <= 100
  ))
    return "Пороги: 0 ≤ средний < высокий ≤ 100.";
  if (!Number.isInteger(s.horizon) || s.horizon < 1 || s.horizon > 365)
    return "В макете горизонт — целое число от 1 до 365 дней.";
  return null;
}

// Browser-only demonstrator. The production engine will run in Go.
export function calculate(
  snapshot: Snapshot,
  settings: Settings,
  now: number,
): Customer[] {
  const grouped = new Map<number, Service[]>();
  for (const service of snapshot.services) {
    const bucket = grouped.get(service.ownerId);
    if (bucket) bucket.push(service);
    else grouped.set(service.ownerId, [service]);
  }
  const weights = factors.map((f) =>
    BigInt(Math.round(settings.weights[f.key] * 1000)),
  );
  const sumWeights = weights.reduce((a, b) => a + b, 0n);
  return snapshot.accounts.map((account) => {
    const services = grouped.get(account.id) ?? [];
    const classify = (s: Service) => {
      if (s.status === "blocked" || s.status === "deleted") return "excluded";
      const expiry = Date.parse(s.expiresAt);
      if (s.status === "prolong_wait")
        return expiry <= now && expiry >= now - 5 * DAY
          ? "eligible"
          : "uncertain";
      if (expiry < now) return "uncertain";
      return expiry <= now + settings.horizon * DAY ? "eligible" : "outside";
    };
    const upcoming = services.filter((s) => classify(s) === "eligible");
    const revenue = upcoming.reduce((sum, s) => sum + BigInt(s.price), 0n);
    const available =
      BigInt(account.balance) +
      (account.bonusState === "unlocked" ? BigInt(account.bonusBalance) : 0n);
    const deficit =
      revenue > available && revenue > 0n
        ? Number(round((revenue - available) * 10000n, revenue))
        : 0;
    const results: ServiceResult[] = services.map((s) => {
      const kind = classify(s);
      const eligible = kind === "eligible";
      const values = [
        s.autoProlong ? 0 : 10000,
        deficit,
        s.status === "prolong_wait" ? 10000 : 0,
      ];
      const contributions = values.map((v, i) =>
        eligible && sumWeights > 0n
          ? Number(round(BigInt(v) * weights[i], sumWeights)) / 100
          : 0,
      );
      const score =
        eligible && sumWeights > 0n
          ? Number(
              round(
                values.reduce((n, v, i) => n + BigInt(v) * weights[i], 0n),
                sumWeights,
              ),
            ) / 100
          : null;
      const reason =
        kind === "uncertain"
          ? "Статус и дата окончания требуют проверки"
          : kind === "excluded"
            ? "Не участвует в будущих продлениях"
            : kind === "outside"
              ? "Продление за пределами горизонта"
              : "Оценка ближайшего продления";
      return {
        ...s,
        eligible,
        uncertain: kind === "uncertain",
        score,
        level: level(score, settings),
        values: values.map((v) => v / 10000),
        contributions,
        reason,
        loss:
          score === null
            ? null
            : round(BigInt(s.price) * BigInt(Math.round(score * 100)), 10000n),
      };
    });
    const assessed = results.filter((s) => s.score !== null);
    const uncertain = results.filter((s) => s.uncertain);
    const partial = uncertain.length > 0;
    const score =
      assessed.length === 0 || partial
        ? null
        : revenue === 0n
          ? Math.round(
              (assessed.reduce((sum, s) => sum + s.score!, 0) /
                assessed.length) *
                100,
            ) / 100
          : Number(
              round(
                assessed.reduce(
                  (sum, s) =>
                    sum + BigInt(s.price) * BigInt(Math.round(s.score! * 100)),
                  0n,
                ),
                revenue,
              ),
            ) / 100;
    const contributions = factors.map((_, i) =>
      assessed.length === 0
        ? 0
        : revenue === 0n
          ? assessed.reduce((sum, s) => sum + s.contributions[i], 0) /
            assessed.length
          : Number(
              round(
                assessed.reduce(
                  (sum, s) =>
                    sum +
                    BigInt(s.price) *
                      BigInt(Math.round(s.contributions[i] * 100)),
                  0n,
                ),
                revenue,
              ),
            ) / 100,
    );
    const reasons: string[] = factors
      .filter((_, i) => assessed.some((s) => s.contributions[i] > 0))
      .map((f) => f.label);
    if (partial) reasons.push("Есть аномалии исходных данных");
    if (assessed.length === 0 && !partial)
      reasons.push("Нет продлений на выбранном горизонте");
    const loss = assessed.length
      ? assessed.reduce((sum, s) => sum + (s.loss ?? 0n), 0n)
      : partial
        ? null
        : 0n;
    return {
      ...account,
      services: results,
      score,
      level: level(score, settings),
      loss,
      revenue,
      unassessed: uncertain.reduce((sum, s) => sum + BigInt(s.price), 0n),
      partial,
      reasons,
      contributions,
    };
  });
}

export function validateSnapshot(input: unknown): {
  snapshot?: Snapshot;
  errors: Issue[];
} {
  const errors: Issue[] = [];
  const add = (path: string, message: string) => {
    if (errors.length < 100) errors.push({ path, message });
  };
  if (!input || typeof input !== "object" || Array.isArray(input))
    return {
      errors: [
        {
          path: "$",
          message:
            "Ожидается объект с массивами accounts, services, transactions.",
        },
      ],
    };
  const root = input as Record<string, unknown>;
  for (const key of Object.keys(root))
    if (!["accounts", "services", "transactions", "capturedAt"].includes(key))
      add(key, "Неизвестное поле снимка.");
  if (
    root.capturedAt !== undefined &&
    (typeof root.capturedAt !== "string" ||
      !/(Z|[+-]\d{2}:\d{2})$/.test(root.capturedAt) ||
      !Number.isFinite(Date.parse(root.capturedAt)))
  )
    add("capturedAt", "Ожидается дата со временем и часовым поясом.");
  const ids: Record<string, Set<number>> = {
    accounts: new Set(),
    services: new Set(),
    transactions: new Set(),
  };
  const emailSet = new Set<string>();
  const fields = {
    accounts: [
      "id",
      "email",
      "balance",
      "withdrawBalance",
      "bonusBalance",
      "bonusState",
      "totalReplenished",
    ],
    services: [
      "id",
      "ownerId",
      "name",
      "paymentTerm",
      "autoProlong",
      "createdAt",
      "expiresAt",
      "status",
      "price",
    ],
    transactions: [
      "id",
      "serviceId",
      "amount",
      "status",
      "type",
      "createdAt",
      "performedAt",
    ],
  };
  const timestamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;
  for (const entity of ["accounts", "services", "transactions"] as const) {
    const rows = root[entity];
    if (!Array.isArray(rows)) {
      add(entity, "Обязательный массив отсутствует.");
      continue;
    }
    if (rows.length > 100000) {
      add(entity, "В макете поддерживается до 100 000 записей каждого типа.");
      continue;
    }
    rows.forEach((raw: unknown, index: number) => {
      const path = `${entity}[${index}]`;
      if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
        add(path, "Ожидается объект.");
        return;
      }
      const row = raw as Record<string, unknown>;
      for (const field of fields[entity])
        if (!(field in row))
          add(`${path}.${field}`, "Обязательное поле отсутствует.");
      if (!safe(row.id) || row.id < 1)
        add(
          `${path}.id`,
          "Нужен положительный целый ID, точно представимый в браузере.",
        );
      else if (ids[entity].has(row.id)) add(`${path}.id`, "Повторяющийся ID.");
      else ids[entity].add(row.id);
      const moneyFields =
        entity === "accounts"
          ? ["balance", "withdrawBalance", "bonusBalance", "totalReplenished"]
          : entity === "services"
            ? ["price"]
            : ["amount"];
      for (const field of moneyFields)
        if (
          !safe(row[field]) ||
          (row[field] as number) < 0 ||
          (field === "amount" && (row[field] as number) > 1000)
        )
          add(
            `${path}.${field}`,
            field === "amount"
              ? "Сумма должна быть целой, от 0 до 1000 евроцентов."
              : "Нужны неотрицательные целые евроценты, точно представимые в браузере.",
          );
      if (entity === "accounts") {
        if (
          typeof row.email !== "string" ||
          !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(row.email)
        )
          add(`${path}.email`, "Некорректный email.");
        else if (emailSet.has(row.email))
          add(
            `${path}.email`,
            "Email должен быть уникальным с учётом регистра.",
          );
        else emailSet.add(row.email);
        if (!["locked", "unlocked"].includes(String(row.bonusState)))
          add(`${path}.bonusState`, "Допустимы locked и unlocked.");
      } else {
        for (const field of entity === "services"
          ? ["createdAt", "expiresAt"]
          : ["createdAt", "performedAt"]) {
          const value = row[field];
          if (field === "performedAt" && value === null) continue;
          if (
            typeof value !== "string" ||
            !timestamp.test(value) ||
            !Number.isFinite(Date.parse(value)) ||
            new Date(value).toISOString() !== value
          )
            add(
              `${path}.${field}`,
              "Ожидается корректная UTC-дата вида 2026-10-19T15:00:00.000Z.",
            );
        }
        if (entity === "services") {
          if (typeof row.name !== "string" || !/^[A-Za-z0-9-]+$/.test(row.name))
            add(`${path}.name`, "Допустимы латинские буквы, цифры и дефис.");
          if (!Object.hasOwn(termLabels, String(row.paymentTerm)))
            add(`${path}.paymentTerm`, "Неизвестный период оплаты.");
          if (!Object.hasOwn(statusLabels, String(row.status)))
            add(`${path}.status`, "Неизвестный статус услуги.");
          if (typeof row.autoProlong !== "boolean")
            add(`${path}.autoProlong`, "Ожидается true или false.");
          if (
            Date.parse(String(row.expiresAt)) <
            Date.parse(String(row.createdAt))
          )
            add(`${path}.expiresAt`, "Дата окончания раньше даты создания.");
        } else {
          if (!["order", "prolong"].includes(String(row.type)))
            add(`${path}.type`, "Допустимы order и prolong.");
          if (!["performed", "not_performed"].includes(String(row.status)))
            add(`${path}.status`, "Допустимы performed и not_performed.");
          if (
            row.status === "performed" &&
            (row.performedAt === null ||
              Date.parse(String(row.performedAt)) <
                Date.parse(String(row.createdAt)))
          )
            add(
              `${path}.performedAt`,
              "Для performed нужна дата не раньше createdAt.",
            );
          if (row.status === "not_performed" && row.performedAt !== null)
            add(`${path}.performedAt`, "Для not_performed требуется null.");
        }
      }
    });
  }
  for (const [entity, field, target] of [
    ["services", "ownerId", "accounts"],
    ["transactions", "serviceId", "services"],
  ] as const) {
    if (Array.isArray(root[entity]))
      root[entity].forEach((row: Record<string, unknown>, i: number) => {
        if (
          row &&
          typeof row === "object" &&
          (!safe(row[field]) || !ids[target].has(row[field] as number))
        )
          add(
            `${entity}[${i}].${field}`,
            "Ссылка на отсутствующую запись этого снимка.",
          );
      });
  }
  return errors.length ? { errors } : { snapshot: input as Snapshot, errors };
}

export function snapshotWarnings(snapshot: Snapshot, now: number): Issue[] {
  return snapshot.services.flatMap((s, i) => {
    const expiry = Date.parse(s.expiresAt);
    const suspicious =
      s.status === "prolong_wait"
        ? expiry > now || expiry < now - 5 * DAY
        : ["active", "suspended"].includes(s.status) && expiry < now;
    return suspicious
      ? [
          {
            path: `services[${i}]`,
            message: `${s.name}: статус «${statusLabels[s.status]}» не согласуется с датой окончания. Риск услуги не оценён.`,
          },
        ]
      : [];
  });
}
