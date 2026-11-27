import { describe, expect, it } from "bun:test";
import { analyzeChain, formatBytes } from "./chain";

describe("analyzeChain (AC-04)", () => {
  it("auto-switches WARP to MASQUE H2 when hop1 is Psiphon", () => {
    const res = analyzeChain({
      two_hop: true,
      hop1: { kind: "psiphon" },
      hop2: { kind: "warp" },
    });
    expect(res.autoSwitchWarpMasqueH2).toBe(true);
    expect(res.warning).toContain("MASQUE");
    expect(res.label).toContain("赛风");
    expect(res.label).toContain("WARP");
  });

  it("keeps standard mode when hop1 is Geph5", () => {
    const res = analyzeChain({
      two_hop: true,
      hop1: { kind: "geph" },
      hop2: { kind: "warp" },
    });
    expect(res.autoSwitchWarpMasqueH2).toBe(false);
    expect(res.warning).toBeNull();
  });

  it("formats traffic bytes cleanly", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2.0 KB");
    expect(formatBytes(3 * 1024 * 1024)).toBe("3.00 MB");
  });

  it("formats serverless direct_bypass as single-hop without warnings", () => {
    const res = analyzeChain({
      two_hop: true,
      hop1: { kind: "direct_bypass" },
      hop2: { kind: "warp" },
    });
    expect(res.label).toBe("直连绕过（无服务器）");
    expect(res.autoSwitchWarpMasqueH2).toBe(false);
    expect(res.warning).toBeNull();
  });
});
