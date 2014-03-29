/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

*/

package kamevapi

import (
	"flag"
	"log/syslog"
	"testing"
	"time"
)

var testLocal = flag.Bool("local", false, "Perform the tests only on local test environment, not by default.") // This flag will be passed here via "go test -local" args
var kamAddr = flag.String("kam_addr", "192.168.56.73:8228", "Address where to reach kamailio evapi")

var kea *KamEvapi

func TestKamailioConn(t *testing.T) {
	if !*testLocal {
		return
	}
	var err error
	l, err := syslog.New(syslog.LOG_INFO, "TestKamEvapi")
	if err != nil {
		t.Fatal("Cannot connect to syslog:", err)
	}
	if kea, err = NewKamEvapi(*kamAddr, 3, true, l); err != nil {
		t.Fatal("Could not create KamEvapi, error: ", err)
	}
	err = kea.Send("CGR-SM Connected!")
	time.Sleep(time.Duration(5) * time.Minute) // Wait 5 mins for events to show up
}
