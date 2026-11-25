declare global {
  interface Window {
    mygo?: {
      call: <T = unknown>(method: string, ...args: unknown[]) => Promise<T>;
    };
  }
}

async function callGo<T>(method: string, ...args: unknown[]): Promise<T> {
  if (typeof window !== "undefined" && window.mygo?.call) {
    return window.mygo.call<T>(method, ...args);
  }
  throw new Error(`MyGo runtime bridge unavailable for ${method}`);
}

export const AppService = {
  GetConfig: () => callGo<Record<string, any>>("AppService.GetConfig"),
  SaveConfig: (cfg: Record<string, any>) => callGo<void>("AppService.SaveConfig", cfg),
  Connect: () => callGo<{ warnings?: Array<{ code: string; message: string }> }>("AppService.Connect"),
  Disconnect: () => callGo<void>("AppService.Disconnect"),
  GetStats: () => callGo<{ running: boolean; up_bytes: number; down_bytes: number; mode: string; chain_desc: string }>("AppService.GetStats"),
  GetLANInfo: () => callGo<{ ip: string; socks_port: number; http_port: number; allow_lan: boolean }>("AppService.GetLANInfo"),
  ImportSubscription: (name: string, url: string, intervalHours: number) =>
    callGo<any[]>("AppService.ImportSubscription", name, url, intervalHours),
  TestAllNodes: () => callGo<Array<{ node_id: string; latency_ms: number; err?: string }>>("AppService.TestAllNodes"),
  FetchGephExits: () => callGo<Array<{ country: string; city: string; load?: number }>>("AppService.FetchGephExits"),
};
