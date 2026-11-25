package platform

import "testing"

func TestDetectChinaApps(t *testing.T) {
	installed := []InstalledApp{
		{PackageName: "com.tencent.mm", Label: "微信"},
		{PackageName: "com.xingin.xhs", Label: "小红书"},
		{PackageName: "com.bytedance.customapp", Label: "Custom"},
		{PackageName: "com.example.localtool", Label: "某某本地工具", InstallerPackage: "com.coolapk.market"},
		{PackageName: "com.example.hanapp", Label: "校园一卡通"},
		{PackageName: "com.google.android.youtube", Label: "YouTube"},
		{PackageName: "org.telegram.messenger", Label: "Telegram"},
		{PackageName: "com.zhiliaoapp.musically", Label: "TikTok"},
		{PackageName: "com.android.settings", Label: "设置", IsSystem: true},
	}

	got := DetectChinaApps(installed)
	if len(got) != 5 {
		t.Fatalf("expected 5 detected CN apps, got %d: %+v", len(got), got)
	}
	reasons := map[string]string{}
	for _, d := range got {
		reasons[d.PackageName] = d.Reason
	}
	if reasons["com.tencent.mm"] != "经典中国软件" {
		t.Errorf("WeChat reason mismatch: %q", reasons["com.tencent.mm"])
	}
	if reasons["com.bytedance.customapp"] != "中国厂商包名" {
		t.Errorf("Bytedance reason mismatch: %q", reasons["com.bytedance.customapp"])
	}
	if reasons["com.example.localtool"] != "国内应用商店安装" {
		t.Errorf("Coolapk installer reason mismatch: %q", reasons["com.example.localtool"])
	}
	if reasons["com.example.hanapp"] != "中文应用名称" {
		t.Errorf("CJK label reason mismatch: %q", reasons["com.example.hanapp"])
	}
	if _, bad := reasons["com.zhiliaoapp.musically"]; bad {
		t.Errorf("TikTok Global must not be excluded")
	}
}
