import { describe, expect, it } from "bun:test";
import {
  BUILTIN_PROBE_URLS,
  countryCodeToFlag,
  formatLatency,
  validateProbeUrl,
} from "./probe";

describe("probe helpers (AC-19, AC-20)", () => {
  it("exposes Hiddify-style builtin connectivity test URLs", () => {
    expect(BUILTIN_PROBE_URLS.length).toBeGreaterThanOrEqual(4);
    expect(BUILTIN_PROBE_URLS[0].url).toContain("generate_204");
  });

  it("converts ISO-2 country codes to flag emoji", () => {
    expect(countryCodeToFlag("JP")).toBe("🇯🇵");
    expect(countryCodeToFlag("us")).toBe("🇺🇸");
    expect(countryCodeToFlag("NL")).toBe("🇳🇱");
    expect(countryCodeToFlag("")).toBe("🌐");
    expect(countryCodeToFlag("123")).toBe("🌐");
  });

  it("validates custom connectivity URLs", () => {
    expect(validateProbeUrl("https://cp.cloudflare.com/generate_204")).toBe(
      true,
    );
    expect(validateProbeUrl("http://1.1.1.1")).toBe(true);
    expect(validateProbeUrl("ftp://example.com")).toBe(false);
    expect(validateProbeUrl("not-a-url")).toBe(false);
  });

  it("formats latency and unreachable errors cleanly", () => {
    expect(
      formatLatency({
        url: "http://1.1.1.1",
        reachable: true,
        latency_ms: 48,
      }),
    ).toBe("48 ms");
    expect(
      formatLatency({
        url: "http://1.1.1.1",
        reachable: false,
        latency_ms: 0,
        error: "timeout",
      }),
    ).toBe("失败 (timeout)");
  });
});
