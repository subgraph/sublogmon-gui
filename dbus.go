package main

import (
	"errors"
	//"path"

	"github.com/godbus/dbus"
	//        "github.com/godbus/dbus/introspect"
	"log"
//	"github.com/op/go-logging"
	"github.com/gotk3/gotk3/glib"
)

const busName = "com.subgraph.EventNotifier"
const objectPath = "/com/subgraph/EventNotifier"
const interfaceName = "com.subgraph.EventNotifier"


type dbusServer struct {
	conn *dbus.Conn
	run bool
}

type slmData struct {
	EventID string
        LogLevel string
        Timestamp int64
        LogLine string
	Metadata map[string]string
}


func newDbusServer() (*dbusServer, error) {
	conn, err := dbus.SystemBus()

	if err != nil {
		return nil, err
	}

	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)

	if err != nil {
		return nil, err
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, errors.New("Bus name is already owned")
	}

	ds := &dbusServer{}

	if err := conn.Export(ds, objectPath, interfaceName); err != nil {
		return nil, err
	}

	ds.conn = conn
	ds.run = true
	return ds, nil
}

func (ds *dbusServer) Alert(data slmData) *dbus.Error {
//	log.Printf(message)
//        dn.show("sysevent", message, true)
//	guiLog(message)
	log.Printf("GOT IT:", data.LogLine)

	if data.LogLevel == "critical" {
		dn.show("sysevent", data.LogLine, true)
	} else {
		log.Println("Skipping event bubble for non-critical log item")
	}

//	guiLog(data)
	glib.IdleAdd(guiLog, data)

	return nil
}
