/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

*/

package kamevapi

import (
	"flag"
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
	if kea, err = NewKamEvapi(*kamAddr, 0, 3, 0, fibDuration, nil, new(nopLogger)); err != nil {
		t.Fatal("Could not create KamEvapi, error: ", err)
	}
	_ = kea.Send("CGR-SM Connected!")
	time.Sleep(time.Duration(2) * time.Second) // Wait 5 mins for events to show up
}
