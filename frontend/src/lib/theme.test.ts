import { describe, expect, it } from "bun:test";
import { resolveTheme } from "./theme";

describe("resolveTheme", () => {
  it("resolves explicit light and dark modes", () => {
    expect(resolveTheme("light", true)).toBe("light");
    expect(resolveTheme("dark", false)).toBe("dark");
  });

  it("follows system preference when mode is system", () => {
    expect(resolveTheme("system", true)).toBe("dark");
    expect(resolveTheme("system", false)).toBe("light");
  });
});
