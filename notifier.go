package main

import (
	"log"
	"fmt"
	"os"
	)

var dn *DesktopNotifications

func main() {

	dbs, err := newDbusServer();
        dn = newDesktopNotifications()

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	gui_main()

	once := true

	for dbs.run == true {
		/* Run forever */

		if once {
			fmt.Fprintf(os.Stderr, "Looks like the log GUI was closed. Continuing to run the desktop notifier in the background...\n")
			once = false
		}

        }

}
