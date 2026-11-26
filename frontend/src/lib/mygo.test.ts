import { afterEach, describe, expect, it } from "bun:test";
import { AndroidNetPlugin, AppService, hasMyGoBridge } from "./mygo";

describe("mygo bridge", () => {
  const origWindow = (globalThis as Record<string, unknown>).window;

  afterEach(() => {
    if (origWindow === undefined) {
      delete (globalThis as Record<string, unknown>).window;
    } else {
      (globalThis as Record<string, unknown>).window = origWindow;
    }
  });

  it("reports unavailable when window.mygo is absent", async () => {
    delete (globalThis as Record<string, unknown>).window;
    expect(hasMyGoBridge()).toBe(false);
    await expect(AppService.GetStats()).rejects.toThrow(
      "MyGo runtime bridge unavailable",
    );
  });

  it("forwards AppService and AndroidNetPlugin calls to window.mygo.call", async () => {
    const calls: Array<{ method: string; args: unknown[] }> = [];
    (globalThis as Record<string, unknown>).window = {
      mygo: {
        call: async (method: string, ...args: unknown[]) => {
          calls.push({ method, args });
          if (method === "AppService.ProbeChain") {
            return {
              test_url: "http://connectivitycheck.gstatic.com/generate_204",
              hops: [],
            };
          }
          if (method === "AndroidNetPlugin.openVpnSettings") {
            return { opened: true };
          }
          return { running: true, up_bytes: 10, down_bytes: 20 };
        },
      },
    };

    expect(hasMyGoBridge()).toBe(true);
    const probeRes = await AppService.ProbeChain();
    expect(probeRes.test_url).toContain("generate_204");
    const openRes = await AndroidNetPlugin.openVpnSettings();
    expect(openRes.opened).toBe(true);
    expect(calls.map((c) => c.method)).toEqual([
      "AppService.ProbeChain",
      "AndroidNetPlugin.openVpnSettings",
    ]);
  });
});
