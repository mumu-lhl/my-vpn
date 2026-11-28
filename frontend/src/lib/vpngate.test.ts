import { describe, expect, it } from "bun:test";
import type { VPNGateNode } from "./mygo";
import {
  defaultVPNGateSortDir,
  extractVPNGateCountries,
  filterVPNGateNodes,
  formatVPNGateCountry,
  formatVPNGateSpeed,
  formatVPNGateTraffic,
  sortVPNGateNodes,
} from "./vpngate";

const SAMPLE_NODES: VPNGateNode[] = [
  {
    id: "jp-1",
    host_name: "vpn-jp-1",
    ip: "203.0.113.1",
    score: 2500000,
    ping: 35,
    speed: 120_000_000,
    country_long: "Japan",
    country_short: "JP",
    num_vpn_sessions: 10,
    uptime: 1000,
    total_users: 100,
    total_traffic: 5 * 1024 ** 4,
    server: "203.0.113.1",
    server_port: 1195,
    network: "tcp",
    supports_udp: false,
  },
  {
    id: "us-1",
    host_name: "vpn-us-1",
    ip: "198.51.100.2",
    score: 1800000,
    ping: 15,
    speed: 250_000_000,
    country_long: "United States",
    country_short: "US",
    num_vpn_sessions: 5,
    uptime: 2000,
    total_users: 50,
    total_traffic: 2 * 1024 ** 4,
    server: "198.51.100.2",
    server_port: 1194,
    network: "udp",
    supports_udp: true,
  },
  {
    id: "jp-2",
    host_name: "vpn-jp-2",
    ip: "203.0.113.3",
    score: 3000000,
    ping: 80,
    speed: 50_000_000,
    country_long: "Japan",
    country_short: "JP",
    num_vpn_sessions: 20,
    uptime: 3000,
    total_users: 200,
    total_traffic: 10 * 1024 ** 4,
    server: "203.0.113.3",
    server_port: 443,
    network: "tcp",
    supports_udp: false,
  },
];

describe("vpngate helpers (AC-VPNGATE-03)", () => {
  it("extracts countries with flags and filters by country code", () => {
    const countries = extractVPNGateCountries(SAMPLE_NODES);
    expect(countries).toHaveLength(2);
    expect(countries[0]).toEqual({
      code: "JP",
      name: "日本",
      flag: "🇯🇵",
      count: 2,
    });
    expect(formatVPNGateCountry("US", "United States")).toEqual({
      flag: "🇺🇸",
      name: "美国",
    });

    const onlyJP = filterVPNGateNodes(SAMPLE_NODES, "jp");
    expect(onlyJP.map((n) => n.id)).toEqual(["jp-1", "jp-2"]);
  });

  it("sorts by score, speed, total_traffic, and ping with optimal top-to-bottom defaults", () => {
    expect(defaultVPNGateSortDir("score")).toBe("desc");
    expect(defaultVPNGateSortDir("speed")).toBe("desc");
    expect(defaultVPNGateSortDir("total_traffic")).toBe("desc");
    expect(defaultVPNGateSortDir("ping")).toBe("asc");

    expect(
      sortVPNGateNodes(SAMPLE_NODES, "score", "desc").map((n) => n.id),
    ).toEqual(["jp-2", "jp-1", "us-1"]);
    expect(
      sortVPNGateNodes(SAMPLE_NODES, "speed", "desc").map((n) => n.id),
    ).toEqual(["us-1", "jp-1", "jp-2"]);
    expect(
      sortVPNGateNodes(SAMPLE_NODES, "total_traffic", "desc").map((n) => n.id),
    ).toEqual(["jp-2", "jp-1", "us-1"]);
    expect(
      sortVPNGateNodes(SAMPLE_NODES, "ping", "asc").map((n) => n.id),
    ).toEqual(["us-1", "jp-1", "jp-2"]);
  });

  it("formats speed and total traffic cleanly", () => {
    expect(formatVPNGateSpeed(120_000_000)).toBe("120 Mbps");
    expect(formatVPNGateTraffic(5 * 1024 ** 4)).toBe("5.00 TB");
  });
});
