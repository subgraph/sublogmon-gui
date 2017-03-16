
package main

import (
	"fmt"
	"log"

	"github.com/godbus/dbus"
	"github.com/TheCreeper/go-notify"
)

type DesktopNotifications struct {
	notifications map[string]uint32
}

func newDesktopNotifications() *DesktopNotifications {
	if _, err := dbus.SessionBus(); err != nil {
		log.Printf("Error enabling dbus based notifications! %+v\n", err)
		return nil
	}

	dn := new(DesktopNotifications)
	dn.notifications = make(map[string]uint32)
	return dn
}

func (dn *DesktopNotifications) show(cat, message string, showFullscreen bool) error {
	hints := make(map[string]interface{})
	//hints[notify.HintResident] = true
	hints[notify.HintTransient] = false
	hints[notify.HintActionIcons] = "sublogmon"
	if showFullscreen {
		hints[notify.HintUrgency] = notify.UrgencyCritical
	}
	notification := notify.Notification{
		AppName: "EventNotifier",
		AppIcon: "dialog-error",
		Timeout: notify.ExpiresNever,
		Hints:   hints,
	}
	notification.Summary = "Subgraph Event Notifier"
	notification.Body = message

	nid, err := notification.Show()
	if err != nil {
		return fmt.Errorf("Error showing notification: %v", err)
	}
	dn.notifications[cat] = nid
	return nil
}

