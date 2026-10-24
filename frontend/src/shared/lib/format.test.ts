import { describe, expect, it } from "vitest";
import { money, percent, score } from "./format";

describe("format", () => {
  it("renders cents as euros and keeps a missing estimate visible", () => {
    expect(money(249050)).toBe("€ 2 490,50");
    expect(money(0)).toBe("€ 0,00");
    expect(money(null)).toBe("—");
  });

  it("rounds scores and marks unassessed customers", () => {
    expect(score(71.6)).toBe("72");
    expect(score(null)).toBe("—");
  });

  it("guards against an empty total", () => {
    expect(percent(1, 4)).toBe("25%");
    expect(percent(0, 0)).toBe("0%");
  });
});
