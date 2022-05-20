/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

*/

package kamevapi

import (
	"bytes"
	"flag"
	"log"
	"testing"
	"time"
)

var testLocal = flag.Bool("local", false, "Perform the tests only on local test environment, not by default.") // This flag will be passed here via "go test -local" args
var kamAddr = flag.String("kam_addr", "127.0.0.1:8448", "Address where to reach kamailio evapi")

var kea *KamEvapi

func TestKamailioConn(t *testing.T) {
	if !*testLocal {
		return
	}
	var err error
	var buf bytes.Buffer
	l := log.New(&buf, "logger: ", log.Lshortfile)
	if err != nil {
		t.Fatal("Cannot connect to syslog:", err)
	}
	if kea, err = NewKamEvapi(*kamAddr, 0, 3, 0, nil, l); err != nil {
		t.Fatal("Could not create KamEvapi, error: ", err)
	}
	err = kea.Send("CGR-SM Connected!")
	time.Sleep(time.Duration(2) * time.Second) // Wait 5 mins for events to show up
}
