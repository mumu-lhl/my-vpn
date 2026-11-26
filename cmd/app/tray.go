package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/mumu-lhl/my-vpn/internal/config"
)

const (
	trayMenuIDStatus = "looma.status"
	trayMenuIDToggle = "looma.toggle"
	trayMenuIDProxy  = "looma.mode.proxy"
	trayMenuIDTUN    = "looma.mode.tun"
	trayMenuIDOpen   = "looma.open"
	trayMenuIDQuit   = "looma.quit"
)

// ShouldKeepAliveInTray decides whether closing the main window should keep
// Looma running in the background. When tray initialization fails, closing the
// window exits instead of leaving an unreachable background process.
func ShouldKeepAliveInTray(cfg *config.AppConfig, trayActive bool) bool {
	if !trayActive || cfg == nil {
		return false
	}
	return cfg.Linux.MinimizeToTray
}

// TrayStateSnapshot captures the UI-visible state mirrored onto the SNI tray.
type TrayStateSnapshot struct {
	Running bool
	Mode    config.InboundMode
	Chain   string
}

func snapshotFromService(svc *AppService) TrayStateSnapshot {
	cfg := svc.GetConfig()
	stats := svc.GetStats()
	mode := cfg.Inbound.Mode
	if mode == "" {
		mode = config.InboundProxy
	}
	return TrayStateSnapshot{
		Running: stats.Running,
		Mode:    mode,
		Chain:   stats.ChainDesc,
	}
}

func formatTrayStatusLabel(s TrayStateSnapshot) string {
	modeLabel := "本地代理"
	if s.Mode == config.InboundTUN {
		modeLabel = "VPN (TUN)"
	}
	if s.Running {
		if s.Chain != "" {
			return fmt.Sprintf("状态：已连接 · %s (%s)", modeLabel, s.Chain)
		}
		return fmt.Sprintf("状态：已连接 · %s", modeLabel)
	}
	return fmt.Sprintf("状态：未连接 · %s", modeLabel)
}

func formatTrayToolTip(s TrayStateSnapshot) string {
	modeLabel := "代理模式"
	if s.Mode == config.InboundTUN {
		modeLabel = "TUN 模式"
	}
	if s.Running {
		return fmt.Sprintf("Looma · 已连接 (%s)", modeLabel)
	}
	return fmt.Sprintf("Looma · 未连接 (%s)", modeLabel)
}

// TrayController manages the desktop SNI system tray and keeps its menu
// synchronized with AppService connection and inbound mode state.
type TrayController struct {
	mu           sync.Mutex
	tray         *mygo.Tray
	menu         *mygo.Menu
	statusItem   *mygo.MenuItem
	toggleItem   *mygo.MenuItem
	proxyItem    *mygo.MenuItem
	tunItem      *mygo.MenuItem
	lastRunning  bool
	iconAssigned bool
	svc          *AppService
	openMain     func()
	quitApp      func()
}

// NewTrayController creates and registers the SNI tray icon and menu.
// Must be called after mygo.App is ready (inside app.WhenReady).
func NewTrayController(svc *AppService, openMain, quitApp func()) (*TrayController, error) {
	c := &TrayController{
		svc:      svc,
		openMain: openMain,
		quitApp:  quitApp,
	}

	snap := snapshotFromService(svc)
	c.statusItem = &mygo.MenuItem{
		ID:       trayMenuIDStatus,
		Label:    formatTrayStatusLabel(snap),
		Disabled: true,
	}
	toggleLabel := "连接"
	if snap.Running {
		toggleLabel = "断开"
	}
	c.toggleItem = &mygo.MenuItem{
		ID:    trayMenuIDToggle,
		Label: toggleLabel,
		Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			go c.handleToggleConnect()
		},
	}
	c.proxyItem = &mygo.MenuItem{
		ID:      trayMenuIDProxy,
		Label:   "本地代理 (SOCKS5 / HTTP)",
		Type:    mygo.MenuItemRadio,
		Checked: snap.Mode != config.InboundTUN,
		Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			go c.handleSwitchMode(config.InboundProxy)
		},
	}
	c.tunItem = &mygo.MenuItem{
		ID:      trayMenuIDTUN,
		Label:   "VPN 模式 (TUN)",
		Type:    mygo.MenuItemRadio,
		Checked: snap.Mode == config.InboundTUN,
		Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			go c.handleSwitchMode(config.InboundTUN)
		},
	}

	c.menu = mygo.NewMenu([]*mygo.MenuItem{
		c.statusItem,
		c.toggleItem,
		mygo.Separator(),
		{
			Label: "切换模式",
			Submenu: []*mygo.MenuItem{
				c.proxyItem,
				c.tunItem,
			},
		},
		mygo.Separator(),
		{
			ID:    trayMenuIDOpen,
			Label: "打开主窗口",
			Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				if c.openMain != nil {
					c.openMain()
				}
			},
		},
		{
			ID:    trayMenuIDQuit,
			Label: "退出",
			Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				if c.quitApp != nil {
					c.quitApp()
				}
			},
		},
	})

	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "looma",
		Icon:           trayIconPNG(snap.Running),
		IconIsTemplate: true,
		ToolTip:        formatTrayToolTip(snap),
		Menu:           c.menu,
	})
	if err != nil {
		return nil, err
	}
	tray.OnClick(func() {
		if c.openMain != nil {
			c.openMain()
		}
	})

	c.tray = tray
	c.lastRunning = snap.Running
	c.iconAssigned = true
	return c, nil
}

func (c *TrayController) Active() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tray != nil
}

func (c *TrayController) Sync() {
	if c == nil {
		return
	}
	snap := snapshotFromService(c.svc)

	c.mu.Lock()
	tray := c.tray
	if tray == nil {
		c.mu.Unlock()
		return
	}
	iconChanged := !c.iconAssigned || c.lastRunning != snap.Running
	c.lastRunning = snap.Running
	c.iconAssigned = true
	c.mu.Unlock()

	c.statusItem.SetLabel(formatTrayStatusLabel(snap))
	if snap.Running {
		c.toggleItem.SetLabel("断开")
	} else {
		c.toggleItem.SetLabel("连接")
	}
	c.proxyItem.SetChecked(snap.Mode != config.InboundTUN)
	c.tunItem.SetChecked(snap.Mode == config.InboundTUN)
	tray.SetToolTip(formatTrayToolTip(snap))
	if iconChanged {
		if err := tray.SetIcon(trayIconPNG(snap.Running), true); err != nil {
			log.Printf("looma tray: update icon failed: %v", err)
		}
	}
}

func (c *TrayController) Destroy() {
	if c == nil {
		return
	}
	c.mu.Lock()
	tray := c.tray
	c.tray = nil
	c.mu.Unlock()
	if tray != nil {
		tray.Destroy()
	}
}

func (c *TrayController) handleToggleConnect() {
	snap := snapshotFromService(c.svc)
	if snap.Running {
		if err := c.svc.Disconnect(); err != nil {
			log.Printf("looma tray: disconnect failed: %v", err)
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := c.svc.Connect(ctx); err != nil {
			log.Printf("looma tray: connect failed: %v", err)
		}
	}
	c.Sync()
}

func (c *TrayController) handleSwitchMode(mode config.InboundMode) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := c.svc.SwitchInboundMode(ctx, mode); err != nil {
		log.Printf("looma tray: switch mode to %s failed: %v", mode, err)
	}
	c.Sync()
}
