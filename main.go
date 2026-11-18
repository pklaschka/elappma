// Elappma installs and updates apps from GitHub releases. This is the app
// with a window and a menu bar icon; cmd/elappma is the command line. Both
// share the core in internal/.
package main

import (
	"context"
	"embed"
	"log"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/pklaschka/elappma/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	svc := &AppService{store: store.New()}
	notifier := notifications.New()

	app := application.New(application.Options{
		Name:        "Elappma",
		Description: "Installs and updates apps from GitHub releases",
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// keep running in the menu bar when the window is closed
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})
	svc.app = app

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "Elappma",
		Width:     760,
		Height:    560,
		MinWidth:  520,
		MinHeight: 360,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})
	// closing the window hides it; Elappma keeps checking from the menu bar
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})
	show := func() {
		window.Show()
		window.Focus()
	}

	checker := &updateChecker{svc: svc, notifier: notifier}
	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	}
	tray.SetTooltip("Elappma")
	menu := app.NewMenu()
	menu.Add("Elappma öffnen").OnClick(func(*application.Context) { show() })
	menu.Add("Nach Updates suchen").OnClick(func(*application.Context) {
		go checker.check(context.Background(), true)
	})
	menu.AddSeparator()
	menu.Add("Elappma beenden").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	checker.tray = tray

	notifier.OnNotificationResponse(func(notifications.NotificationResult) { show() })

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			if ok, err := notifier.CheckNotificationAuthorization(); err == nil && !ok {
				_, _ = notifier.RequestNotificationAuthorization()
			}
		}()
		go checker.loop()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// updateChecker looks for updates in the background and tells the user once
// per new set of updates.
type updateChecker struct {
	svc      *AppService
	notifier *notifications.NotificationService
	tray     *application.SystemTray
	notified []string // "repo@version" already announced
}

func (c *updateChecker) loop() {
	time.Sleep(30 * time.Second) // let the Mac settle after login
	for {
		c.check(context.Background(), false)
		hours := 6
		if cfg, err := c.svc.store.Config(); err == nil {
			hours = cfg.Interval()
		}
		time.Sleep(time.Duration(hours) * time.Hour)
	}
}

// check refreshes the list. It notifies about updates not announced yet, or
// about any update (and about none) when the user asked.
func (c *updateChecker) check(ctx context.Context, asked bool) {
	list, err := c.svc.Apps(ctx)
	if err != nil {
		log.Println("update check:", err)
		return
	}
	c.svc.app.Event.Emit("apps:changed", list)

	var names, keys []string
	for _, a := range list {
		if a.UpdateAvailable {
			names = append(names, a.Name+" "+a.Latest)
			keys = append(keys, a.Repo+"@"+a.Latest)
		}
	}
	if len(keys) == 0 {
		c.tray.SetLabel("")
		if asked {
			c.notify("Alles aktuell", "Für keine App gibt es ein Update.")
		}
		return
	}
	c.tray.SetLabel("•")
	fresh := slices.ContainsFunc(keys, func(k string) bool { return !slices.Contains(c.notified, k) })
	if !fresh && !asked {
		return
	}
	c.notified = keys
	title := "Update verfügbar"
	if len(names) > 1 {
		title = "Updates verfügbar"
	}
	c.notify(title, strings.Join(names, ", "))
}

func (c *updateChecker) notify(title, body string) {
	err := c.notifier.SendNotification(notifications.NotificationOptions{
		ID:    "elappma-updates",
		Title: title,
		Body:  body,
	})
	if err != nil {
		log.Println("notification:", err)
	}
}
