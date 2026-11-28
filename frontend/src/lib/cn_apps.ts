export interface InstalledApp {
  package_name: string;
  label: string;
  is_system?: boolean;
  installer_package?: string;
}

export interface ClassicCNApp {
  package_name: string;
  name: string;
  category: string;
}

export interface DetectedCNApp {
  package_name: string;
  label: string;
  reason: string;
}

export const CLASSIC_CN_APPS: ClassicCNApp[] = [
  { package_name: "com.tencent.mm", name: "微信", category: "社交" },
  { package_name: "com.tencent.mobileqq", name: "QQ", category: "社交" },
  { package_name: "com.tencent.tim", name: "TIM", category: "社交" },
  { package_name: "com.tencent.wework", name: "企业微信", category: "办公" },
  { package_name: "com.alibaba.android.rimet", name: "钉钉", category: "办公" },
  { package_name: "com.ss.android.lark", name: "飞书", category: "办公" },
  { package_name: "com.sina.weibo", name: "微博", category: "社交" },
  { package_name: "com.xingin.xhs", name: "小红书", category: "内容" },
  { package_name: "com.ss.android.ugc.aweme", name: "抖音", category: "视频" },
  { package_name: "tv.danmaku.bili", name: "哔哩哔哩", category: "视频" },
  { package_name: "com.smile.gifmaker", name: "快手", category: "视频" },
  { package_name: "com.zhihu.android", name: "知乎", category: "内容" },
  {
    package_name: "com.netease.cloudmusic",
    name: "网易云音乐",
    category: "音乐",
  },
  { package_name: "com.tencent.qqmusic", name: "QQ 音乐", category: "音乐" },
  {
    package_name: "com.eg.android.AlipayGphone",
    name: "支付宝",
    category: "支付",
  },
  { package_name: "com.taobao.taobao", name: "淘宝", category: "电商" },
  { package_name: "com.jingdong.app.mall", name: "京东", category: "电商" },
  { package_name: "com.xunmeng.pinduoduo", name: "拼多多", category: "电商" },
  { package_name: "com.sankuai.meituan", name: "美团", category: "生活" },
  { package_name: "me.ele", name: "饿了么", category: "生活" },
  { package_name: "com.autonavi.minimap", name: "高德地图", category: "出行" },
  { package_name: "com.baidu.BaiduMap", name: "百度地图", category: "出行" },
  { package_name: "com.sdu.didi.psnger", name: "滴滴出行", category: "出行" },
  { package_name: "com.MobileTicket", name: "铁路 12306", category: "出行" },
  { package_name: "com.baidu.netdisk", name: "百度网盘", category: "工具" },
  { package_name: "com.alicloud.databox", name: "阿里云盘", category: "工具" },
];

const CN_VENDOR_PREFIXES = [
  "cn.",
  "com.tencent.",
  "com.alibaba.",
  "com.taobao.",
  "com.alipay.",
  "com.eg.android.alipay",
  "com.bytedance.",
  "com.ss.android.",
  "com.baidu.",
  "com.netease.",
  "com.jd.",
  "com.jingdong.",
  "com.sankuai.",
  "com.meituan.",
  "com.xiaomi.",
  "com.miui.",
  "com.huawei.",
  "com.hihonor.",
  "com.heytap.",
  "com.oppo.",
  "com.vivo.",
  "com.bbk.",
  "com.sina.",
  "com.ctrip.",
  "com.xunmeng.",
  "com.xingin.",
  "com.kuaishou.",
  "com.smile.gifmaker",
  "com.zhihu.",
  "com.unionpay.",
  "com.cmbchina.",
  "com.icbc",
  "com.chinamworld.",
  "tv.danmaku.bili",
];

const CN_APP_STORES = new Set([
  "com.xiaomi.market",
  "com.huawei.appmarket",
  "com.hihonor.appmarket",
  "com.heytap.market",
  "com.oppo.market",
  "com.bbk.appstore",
  "com.coolapk.market",
  "com.tencent.android.qqdownloader",
  "com.wandoujia.phoenix2",
  "com.baidu.appsearch",
]);

const INTERNATIONAL_KEEP = [
  "com.google.",
  "org.telegram.",
  "com.whatsapp",
  "com.twitter.",
  "com.github.",
  "com.discord",
  "com.openai.",
  "com.anthropic.",
  "com.zhiliaoapp.musically",
  "com.ss.android.ugc.trill",
  "com.tencent.ig",
];

const CJK_RE = /[\u4e00-\u9fff]/;

export function detectChinaApps(apps: InstalledApp[]): DetectedCNApp[] {
  const classicMap = new Map(
    CLASSIC_CN_APPS.map((a) => [a.package_name.toLowerCase(), a]),
  );
  const out: DetectedCNApp[] = [];

  for (const app of apps) {
    const pkg = app.package_name.trim().toLowerCase();
    if (
      !pkg ||
      INTERNATIONAL_KEEP.some((p) => pkg === p || pkg.startsWith(p))
    ) {
      continue;
    }
    const classic = classicMap.get(pkg);
    if (classic) {
      out.push({
        package_name: app.package_name,
        label: app.label || classic.name,
        reason: "经典中国软件",
      });
      continue;
    }
    if (
      !app.is_system &&
      app.installer_package &&
      CN_APP_STORES.has(app.installer_package.toLowerCase())
    ) {
      out.push({
        package_name: app.package_name,
        label: app.label,
        reason: "国内应用商店安装",
      });
      continue;
    }
    if (
      pkg.includes(".cn.") ||
      CN_VENDOR_PREFIXES.some((p) => pkg.startsWith(p))
    ) {
      out.push({
        package_name: app.package_name,
        label: app.label,
        reason: "中国厂商包名",
      });
      continue;
    }
    if (!app.is_system && CJK_RE.test(app.label)) {
      out.push({
        package_name: app.package_name,
        label: app.label,
        reason: "中文应用名称",
      });
    }
  }
  return out;
}

export function mergeUniquePackages(
  existing: string[],
  additions: string[],
): string[] {
  return Array.from(new Set([...existing, ...additions]));
}
