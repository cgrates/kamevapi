/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

Provides Kamailio evapi socket communication.
*/

package kamevapi

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
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

func TestKamevapiFib(t *testing.T) {
	fib := fib()
	f := fib()
	expected := 1
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 1
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 2
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 3
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 5
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
}

func TestNewKamEvapiError(t *testing.T) {
	var buf bytes.Buffer
	var err error
	lg := log.New(&buf, "logger: ", log.Lshortfile)
	if err != nil {
		t.Fatal("Cannot connect to syslog:", err)
	}
	errExpect := "dial tcp 127.0.0.1:9435: connect: connection refused"
	if kea, err = NewKamEvapi("127.0.0.1:9435", 0, 3, nil, lg); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestReadevents(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:9435")
	if err != nil {
		t.Fatal(err)
	}
	go l.Accept()
	defer l.Close()
	var buf bytes.Buffer
	lg := log.New(&buf, "logger: ", log.Lshortfile)
	if err != nil {
		t.Fatal("Cannot connect to syslog:", err)
	}
	if kea, err = NewKamEvapi("127.0.0.1:9435", 0, 3, nil, lg); err != nil {
		t.Fatal("Could not create KamEvapi, error: ", err)
	}
	exitChan := make(chan struct{}, 1)
	errorChan := make(chan error, 1)
	errorChan <- err
	close(exitChan)
	kea.readEvents(exitChan, errorChan)
}

func makeNewKea() *KamEvapi {
	l, err := net.Listen("tcp", "127.0.0.1:9435")
	if err != nil {
		fmt.Println(err)
	}
	go l.Accept()
	defer l.Close()
	var buf bytes.Buffer
	lg := log.New(&buf, "logger: ", log.Lshortfile)
	if err != nil {
		fmt.Println("Cannot connect to syslog:", err)
	}
	if kea, err = NewKamEvapi("127.0.0.1:9435", 0, 3, nil, lg); err != nil {
		fmt.Println(err)
	}
	return kea
}

func TestConnectError(t *testing.T) {
	kea := makeNewKea()
	mckChan := make(chan struct{})
	kea.stopReadEvents = mckChan
	kea.Connect()
}

func TestSend(t *testing.T) {
	kea := makeNewKea()
	if err := kea.Send("test"); err != nil {
		t.Error(err)
	}
}

func TestRemoteAddr(t *testing.T) {
	kea := makeNewKea()
	if err := kea.RemoteAddr(); err == nil {
		t.Error("Address shouldn't be nil")
	}
}

type mockConn struct{}

func (mockConn) Close() error                       { return errors.New("error") }
func (mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (mockConn) Write(b []byte) (n int, err error)  { return 0, nil }
func (mockConn) LocalAddr() net.Addr                { return &net.IPAddr{} }
func (mockConn) RemoteAddr() net.Addr               { return &net.IPAddr{} }
func (mockConn) SetDeadline(t time.Time) error      { return nil }
func (mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mockConn) SetWriteDeadline(t time.Time) error { return nil }
func TestReadEventsErrDisconnect(t *testing.T) {
	mckConn := new(mockConn)
	// c1, _ := net.Pipe()
	kea := &KamEvapi{
		errReadEvents: make(chan error, 1),
		connMutex:     &sync.RWMutex{},
		conn:          mckConn,
		logger:        log.Default(),
	}
	kea.errReadEvents <- io.EOF
	errExpect := "error"
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	bufExpect := "Disconnecting from"
	if err := kea.ReadEvents(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	} else if rcv := buf.String(); !strings.Contains(rcv, bufExpect) {
		t.Errorf("Expected %s but received %s", bufExpect, rcv)
	}
	buf.Reset()
}

func TestReadEventsErrNotConnected(t *testing.T) {
	c1, _ := net.Pipe()
	kea := &KamEvapi{
		errReadEvents: make(chan error, 1),
		connMutex:     &sync.RWMutex{},
		conn:          c1,
		logger:        log.Default(),
	}
	kea.errReadEvents <- io.EOF
	errExpect := "Not connected to Kamailio"
	if err := kea.ReadEvents(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but recevied %v", errExpect, err)
	}
}

func TestReadEventsErrNotEOF(t *testing.T) {
	c1, _ := net.Pipe()
	kea := &KamEvapi{
		errReadEvents: make(chan error, 1),
		connMutex:     &sync.RWMutex{},
		conn:          c1,
		logger:        log.Default(),
	}
	kea.errReadEvents <- errors.New("not EOF!")
	errExpect := "not EOF!"
	if err := kea.ReadEvents(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but recevied %v", errExpect, err)
	}

}

func TestSendAsNetstring(t *testing.T) {
	w, r := net.Pipe() //w -> server & r -> client
	kea := &KamEvapi{
		conn:      w,
		connMutex: new(sync.RWMutex),
	}
	ch := make(chan string)
	go func() {
		b := make([]byte, 512)
		n, err := r.Read(b)
		if err != nil {
			t.Error(err)
			ch <- ""
			return
		}
		ch <- string(b[:n])
	}()
	if err := kea.sendAsNetstring("string!"); err != nil {
		t.Fatal(err)
	}
	rcv := <-ch
	rcvExpect := "7:string!,"
	if rcv != rcvExpect {
		t.Errorf("Expected %s but received %s", rcvExpect, rcv)
	}
}

func TestReadNetstring(t *testing.T) {
	kea := &KamEvapi{
		rcvBuffer: bufio.NewReader(bytes.NewBufferString("7:string!,")),
	}
	rcv, err := kea.readNetstring()
	if err != nil {
		t.Error(err)
	}
	rcvExpect := []byte("string!")
	if !reflect.DeepEqual(rcv, rcvExpect) {
		t.Errorf("Expected %s but received %s", rcvExpect, rcv)
	}
}

func TestReadNetstringError1(t *testing.T) {
	kea := &KamEvapi{
		rcvBuffer: bufio.NewReader(bytes.NewBufferString("3:string")),
	}
	errExpect := "Crosschecking netstring failed, no comma in the end but: i"
	if _, err := kea.readNetstring(); err == nil || errExpect != err.Error() {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestReadNetstringError2(t *testing.T) {
	kea := &KamEvapi{
		rcvBuffer: bufio.NewReader(bytes.NewBufferString(":string")),
	}
	errExpect := `strconv.Atoi: parsing "": invalid syntax`
	if _, err := kea.readNetstring(); err == nil || errExpect != err.Error() {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestReadNetstringError3(t *testing.T) {
	kea := &KamEvapi{
		rcvBuffer: bufio.NewReader(bytes.NewBufferString("7:string,")), //It is not valid because it is not "7:string!,"
	}
	errExpect := io.EOF
	if _, err := kea.readNetstring(); err == nil || errExpect != err {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestRemoteAddrErr(t *testing.T) {
	kea := &KamEvapi{
		conn:      nil,
		connMutex: &sync.RWMutex{},
	}
	if err := kea.RemoteAddr(); err != nil {
		t.Error(err)
	}
}

func TestSendErr(t *testing.T) {
	kea := &KamEvapi{
		conn:      nil,
		connMutex: &sync.RWMutex{},
	}
	errExpect := "Not connected to Kamailio"
	if err := kea.Send("idk"); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestReconnectIfNeeded(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:9435")
	if err != nil {
		fmt.Println(err)
	}
	go l.Accept()
	defer l.Close()
	kea := &KamEvapi{
		kamaddr:    "127.0.0.1:9435",
		connMutex:  &sync.RWMutex{},
		conn:       nil,
		reconnects: 4,
		delayFunc:  func() int { return 0 },
	}
	// kea.connMutex = &sync.RWMutex{}
	// kea.conn = nil
	// kea.reconnects = 4
	// kea.delayFunc = func() int { return 0 }
	if err := kea.ReconnectIfNeeded(); err != nil {
		t.Error(err)
	}
}

func TestReconnectIfNeeded2(t *testing.T) {
	kea := &KamEvapi{
		kamaddr:    "127.0.0.1:9435",
		connMutex:  &sync.RWMutex{},
		conn:       nil,
		reconnects: 4,
		delayFunc:  func() int { return 0 },
	}
	errExpect := "dial tcp 127.0.0.1:9435: connect: connection refused"
	if err := kea.ReconnectIfNeeded(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestDispatchEventErr(t *testing.T) {
	kea := &KamEvapi{
		connIdx:       2,
		eventHandlers: map[*regexp.Regexp][]func([]byte, int){
			// regexp.MustCompile(".*"): []func([]byte, int){},
		},
		logger: log.Default(),
	}
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	bufExpect := "No handler for inbound data: test"
	kea.dispatchEvent([]byte("test"))
	if rcv := buf.String(); !strings.Contains(rcv, bufExpect) {
		t.Errorf("Expected %s but received %s", bufExpect, rcv)
	}
	buf.Reset()
}

//Test for KamEvapiPool
func TestNewKamEvapiPool(t *testing.T) {
	var buf bytes.Buffer
	var err error
	lg := log.New(&buf, "logger: ", log.Lshortfile)
	if err != nil {
		t.Fatal("Cannot connect to syslog:", err)
	}
	if _, err = NewKamEvapiPool(2, "127.0.0.1:9435", 0, 3, lg); err != nil {
		t.Fatal("Could not create KamEvapi, error: ", err)
	}
}
