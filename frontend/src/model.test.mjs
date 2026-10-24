import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  calculate,
  defaults,
  money,
  settingsError,
  validateSnapshot,
} from "./model.ts";

const now = Date.parse("2026-10-24T12:00:00.000Z");
const source = () => ({
  accounts: [
    {
      id: 1,
      email: "customer@example.test",
      balance: 0,
      withdrawBalance: 10000,
      bonusBalance: 0,
      bonusState: "locked",
      totalReplenished: 1000,
    },
  ],
  services: [
    {
      id: 1,
      ownerId: 1,
      name: "main-server",
      paymentTerm: "month",
      autoProlong: false,
      createdAt: "2026-10-01T12:00:00.000Z",
      expiresAt: "2026-10-25T12:00:00.000Z",
      status: "active",
      price: 250,
    },
  ],
  transactions: [],
});

test("organizer snapshot passes validation without modifying the original data", () => {
  const snapshot = JSON.parse(
    readFileSync(new URL("../../testdata/data.json", import.meta.url), "utf8"),
  );
  const before = structuredClone(snapshot);
  assert.equal(validateSnapshot(snapshot).errors.length, 0);
  calculate(snapshot, defaults, now);
  assert.deepEqual(snapshot, before);
});

test("uses a shared balance across services, excludes referral balance, and rounds cents half-up", () => {
  const snapshot = source();
  const customer = calculate(snapshot, defaults, now)[0];
  assert.equal(customer.score, 75);
  assert.equal(customer.loss, 188n);
  snapshot.accounts[0].balance = 250;
  snapshot.services.push({ ...snapshot.services[0], id: 2 });
  const shared = calculate(snapshot, defaults, now)[0];
  assert.equal(shared.score, 57.5);
  assert.equal(shared.loss, 288n);
});

test("only unlocked bonuses cover renewals", () => {
  const snapshot = source();
  snapshot.accounts[0].bonusBalance = 250;
  assert.equal(calculate(snapshot, defaults, now)[0].score, 75);
  snapshot.accounts[0].bonusState = "unlocked";
  assert.equal(calculate(snapshot, defaults, now)[0].score, 40);
});

test("stale prolong_wait remains unchanged and produces unknown risk with unassessed revenue", () => {
  const snapshot = source();
  snapshot.services[0].status = "prolong_wait";
  snapshot.services[0].expiresAt = "2026-10-10T12:00:00.000Z";
  const customer = calculate(snapshot, defaults, now)[0];
  assert.equal(customer.score, null);
  assert.equal(customer.level, "unknown");
  assert.equal(customer.loss, null);
  assert.equal(customer.unassessed, 250n);
  assert.equal(snapshot.services[0].status, "prolong_wait");
});

test("valid zero score differs from unknown and zero-priced services do not divide by zero", () => {
  const snapshot = source();
  snapshot.services[0].price = 0;
  snapshot.services[0].autoProlong = true;
  const assessed = calculate(snapshot, defaults, now)[0];
  assert.equal(assessed.score, 0);
  assert.equal(assessed.level, "low");
  assert.equal(assessed.loss, 0n);
  snapshot.services[0].status = "deleted";
  assert.equal(calculate(snapshot, defaults, now)[0].score, null);
});

test("invalid snapshot reports IDs, links, money, dates and statuses instead of dropping records", () => {
  const snapshot = source();
  snapshot.accounts.push({ ...snapshot.accounts[0] });
  snapshot.services[0].ownerId = 999;
  snapshot.services[0].expiresAt = "2026-02-30T12:00:00.000Z";
  snapshot.transactions.push({
    id: 1,
    serviceId: 999,
    amount: 1001,
    status: "not_performed",
    type: "order",
    createdAt: "2026-10-24T12:00:00.000Z",
    performedAt: "2026-10-24T12:01:00.000Z",
  });
  const result = validateSnapshot(snapshot);
  assert.equal(result.snapshot, undefined);
  for (const path of [
    "accounts[1].id",
    "accounts[1].email",
    "services[0].ownerId",
    "services[0].expiresAt",
    "transactions[0].serviceId",
    "transactions[0].amount",
    "transactions[0].performedAt",
  ]) {
    assert.ok(
      result.errors.some((issue) => issue.path === path),
      path,
    );
  }
});

test("zero and 1000 transaction amounts, empty snapshots and case-sensitive unique emails are accepted", () => {
  assert.equal(
    validateSnapshot({ accounts: [], services: [], transactions: [] }).errors
      .length,
    0,
  );
  const snapshot = source();
  snapshot.accounts.push({
    ...snapshot.accounts[0],
    id: 2,
    email: "CUSTOMER@example.test",
  });
  for (const amount of [0, 1000])
    snapshot.transactions.push({
      id: amount + 1,
      serviceId: 1,
      amount,
      status: "not_performed",
      type: "prolong",
      createdAt: "2026-10-24T12:00:00.000Z",
      performedAt: null,
    });
  assert.equal(validateSnapshot(snapshot).errors.length, 0);
});

test("invalid model settings are rejected and monetary formatting preserves exact large integers", () => {
  assert.ok(settingsError({ ...defaults, medium: 70, high: 35 }));
  assert.ok(
    settingsError({
      ...defaults,
      weights: {
        renewalWithoutAutoProlong: 0,
        insufficientRenewalFunds: 0,
        paymentWaiting: 0,
      },
    }),
  );
  assert.equal(
    money(9223372036854775807n).replace(/\s/g, ""),
    "92233720368547758,07€",
  );
});
