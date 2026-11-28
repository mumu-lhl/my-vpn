package platform

import (
	"strings"
	"unicode"
)

type InstalledApp struct {
	PackageName      string `json:"package_name"`
	Label            string `json:"label"`
	IsSystem         bool   `json:"is_system"`
	InstallerPackage string `json:"installer_package,omitempty"`
}

type ClassicCNApp struct {
	PackageName string `json:"package_name"`
	Name        string `json:"name"`
	Category    string `json:"category"`
}

type DetectedCNApp struct {
	PackageName string `json:"package_name"`
	Label       string `json:"label"`
	Reason      string `json:"reason"`
}

var ClassicCNApps = []ClassicCNApp{
	{PackageName: "com.tencent.mm", Name: "微信", Category: "社交"},
	{PackageName: "com.tencent.mobileqq", Name: "QQ", Category: "社交"},
	{PackageName: "com.tencent.tim", Name: "TIM", Category: "社交"},
	{PackageName: "com.tencent.wework", Name: "企业微信", Category: "办公"},
	{PackageName: "com.alibaba.android.rimet", Name: "钉钉", Category: "办公"},
	{PackageName: "com.ss.android.lark", Name: "飞书", Category: "办公"},
	{PackageName: "com.sina.weibo", Name: "微博", Category: "社交"},
	{PackageName: "com.xingin.xhs", Name: "小红书", Category: "内容"},
	{PackageName: "com.ss.android.ugc.aweme", Name: "抖音", Category: "视频"},
	{PackageName: "tv.danmaku.bili", Name: "哔哩哔哩", Category: "视频"},
	{PackageName: "com.smile.gifmaker", Name: "快手", Category: "视频"},
	{PackageName: "com.zhihu.android", Name: "知乎", Category: "内容"},
	{PackageName: "com.netease.cloudmusic", Name: "网易云音乐", Category: "音乐"},
	{PackageName: "com.tencent.qqmusic", Name: "QQ 音乐", Category: "音乐"},
	{PackageName: "com.qiyi.video", Name: "爱奇艺", Category: "视频"},
	{PackageName: "com.youku.phone", Name: "优酷", Category: "视频"},
	{PackageName: "com.tencent.qqlive", Name: "腾讯视频", Category: "视频"},
	{PackageName: "com.eg.android.AlipayGphone", Name: "支付宝", Category: "支付"},
	{PackageName: "com.taobao.taobao", Name: "淘宝", Category: "电商"},
	{PackageName: "com.jingdong.app.mall", Name: "京东", Category: "电商"},
	{PackageName: "com.xunmeng.pinduoduo", Name: "拼多多", Category: "电商"},
	{PackageName: "com.sankuai.meituan", Name: "美团", Category: "生活"},
	{PackageName: "me.ele", Name: "饿了么", Category: "生活"},
	{PackageName: "com.autonavi.minimap", Name: "高德地图", Category: "出行"},
	{PackageName: "com.baidu.BaiduMap", Name: "百度地图", Category: "出行"},
	{PackageName: "com.sdu.didi.psnger", Name: "滴滴出行", Category: "出行"},
	{PackageName: "com.MobileTicket", Name: "铁路 12306", Category: "出行"},
	{PackageName: "com.ctrip.ctrip", Name: "携程", Category: "出行"},
	{PackageName: "com.icbc", Name: "中国工商银行", Category: "金融"},
	{PackageName: "com.chinamworld.main", Name: "中国建设银行", Category: "金融"},
	{PackageName: "com.cmbchina.pb", Name: "招商银行", Category: "金融"},
	{PackageName: "com.baidu.netdisk", Name: "百度网盘", Category: "工具"},
	{PackageName: "com.alicloud.databox", Name: "阿里云盘", Category: "工具"},
}

var cnVendorPrefixes = []string{
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
	"com.sohu.",
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
	"com.bankcomm.",
	"com.abchina.",
	"com.boc.",
	"com.sdu.didi.",
	"tv.danmaku.bili",
}

var cnAppStores = map[string]bool{
	"com.xiaomi.market":                true,
	"com.huawei.appmarket":             true,
	"com.hihonor.appmarket":            true,
	"com.heytap.market":                true,
	"com.oppo.market":                  true,
	"com.bbk.appstore":                 true,
	"com.coolapk.market":               true,
	"com.tencent.android.qqdownloader": true,
	"com.wandoujia.phoenix2":           true,
	"com.baidu.appsearch":              true,
}

var internationalNeverExclude = []string{
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
}

func DetectChinaApps(apps []InstalledApp) []DetectedCNApp {
	classicSet := make(map[string]ClassicCNApp, len(ClassicCNApps))
	for _, a := range ClassicCNApps {
		classicSet[strings.ToLower(a.PackageName)] = a
	}

	var out []DetectedCNApp
	for _, app := range apps {
		pkgLower := strings.ToLower(strings.TrimSpace(app.PackageName))
		if pkgLower == "" || isInternationalExcluded(pkgLower) {
			continue
		}
		if c, ok := classicSet[pkgLower]; ok {
			label := app.Label
			if label == "" {
				label = c.Name
			}
			out = append(out, DetectedCNApp{PackageName: app.PackageName, Label: label, Reason: "经典中国软件"})
			continue
		}
		if cnAppStores[strings.ToLower(app.InstallerPackage)] && !app.IsSystem {
			out = append(out, DetectedCNApp{PackageName: app.PackageName, Label: app.Label, Reason: "国内应用商店安装"})
			continue
		}
		if hasCNVendorPrefix(pkgLower) {
			out = append(out, DetectedCNApp{PackageName: app.PackageName, Label: app.Label, Reason: "中国厂商包名"})
			continue
		}
		if !app.IsSystem && containsCJK(app.Label) {
			out = append(out, DetectedCNApp{PackageName: app.PackageName, Label: app.Label, Reason: "中文应用名称"})
		}
	}
	return out
}

func isInternationalExcluded(pkgLower string) bool {
	for _, p := range internationalNeverExclude {
		if pkgLower == p || strings.HasPrefix(pkgLower, p) {
			return true
		}
	}
	return false
}

func hasCNVendorPrefix(pkgLower string) bool {
	if strings.Contains(pkgLower, ".cn.") {
		return true
	}
	for _, p := range cnVendorPrefixes {
		if strings.HasPrefix(pkgLower, p) {
			return true
		}
	}
	return false
}

func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
