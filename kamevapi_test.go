/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

Provides Kamailio evapi socket communication.
*/

package kamevapi

import (
	"bufio"
	"os"
	"reflect"
	"regexp"
	"testing"
	"time"
)

var authRequest = `188:{"event":"CGR_AUTH_REQUEST",
  "tr_index":"35215",
  "tr_label":"852281699",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_setuptime":"1420142013"},`

var callStart = `264:{"event":"CGR_CALL_START",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "h_entry":"2395",
  "h_id":"2711",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016"},`
var callEnd = `248:{"event":"CGR_CALL_END",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001", 
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016",
  "cgr_duration":"6"},`

func TestDispatchEvent(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Error("Error creating pipe!")
	}
	kea = &KamEvapi{}
	kea.rcvBuffer = bufio.NewReader(r)
	var events []string
	kea.eventHandlers = map[*regexp.Regexp][]func([]byte, int){
		regexp.MustCompile(".*"): []func([]byte, int){func(ev []byte, ignr int) { events = append(events, string(ev)) }},
	}
	go kea.readEvents(make(chan struct{}), make(chan error))
	w.WriteString(authRequest)
	w.WriteString(callStart)
	w.WriteString(callEnd)
	time.Sleep(50 * time.Millisecond)
	expectEvents := []string{
		`{"event":"CGR_AUTH_REQUEST",
  "tr_index":"35215",
  "tr_label":"852281699",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_setuptime":"1420142013"}`, `{"event":"CGR_CALL_START",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "h_entry":"2395",
  "h_id":"2711",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016"}`, `{"event":"CGR_CALL_END",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001", 
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016",
  "cgr_duration":"6"}`,
	}
	expectEvents2 := []string{
		`{"event":"CGR_CALL_END",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001", 
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016",
  "cgr_duration":"6"}`,
		`{"event":"CGR_AUTH_REQUEST",
  "tr_index":"35215",
  "tr_label":"852281699",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_setuptime":"1420142013"}`, `{"event":"CGR_CALL_START",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "h_entry":"2395",
  "h_id":"2711",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016"}`,
	}
	expectEvents3 := []string{
		`{"event":"CGR_CALL_END",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001", 
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016",
  "cgr_duration":"6"}`, `{"event":"CGR_CALL_START",
  "callid":"fcab096696848e58191ed79fdd732751@0:0:0:0:0:0:0:0",
  "from_tag":"4c759c18",
  "h_entry":"2395",
  "h_id":"2711",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_answertime":"1420142016"}`,
		`{"event":"CGR_AUTH_REQUEST",
  "tr_index":"35215",
  "tr_label":"852281699",
  "cgr_reqtype":"postpaid",
  "cgr_account":"1001",
  "cgr_destination":"1002",
  "cgr_setuptime":"1420142013"}`,
	}
	if !reflect.DeepEqual(expectEvents, events) &&
		!reflect.DeepEqual(expectEvents2, events) &&
		!reflect.DeepEqual(expectEvents3, events) {
		t.Errorf("Received events: %+v", events)
	}

}
