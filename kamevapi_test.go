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
		regexp.MustCompile(".*"): {func(ev []byte, ignr int) { events = append(events, string(ev)) }},
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
	fib := fibDuration(time.Second, 0)
	f := fib()
	expected := 1 * time.Second
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 1 * time.Second
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 2 * time.Second
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 3 * time.Second
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
	f = fib()
	expected = 5 * time.Second
	if expected != f {
		t.Errorf("Exptected %v but received %v", expected, f)
	}
}

func TestNewKamEvapiError(t *testing.T) {
	var err error
	errExpect := "KamEvapi dial tcp 127.0.0.1:9435: connect: connection refused"
	if kea, err = NewKamEvapi("127.0.0.1:9435", 0, 3, 0, fibDuration, nil, nil); err == nil || err.Error() != errExpect {
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
	if err != nil {
		t.Fatal("Cannot connect to syslog:", err)
	}
	if kea, err = NewKamEvapi("127.0.0.1:9435", 0, 3, 0, fibDuration, nil, new(nopLogger)); err != nil {
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
	if err != nil {
		fmt.Println("Cannot connect to syslog:", err)
	}
	if kea, err = NewKamEvapi("127.0.0.1:9435", 0, 3, 0, fibDuration, nil, new(nopLogger)); err != nil {
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

type testLogger struct {
	*log.Logger
}

func (l *testLogger) Alert(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Close() error {
	return nil
}

func (l *testLogger) Crit(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Debug(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Emerg(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Err(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Notice(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Warning(msg string) error {
	l.Println(msg)
	return nil
}

func (l *testLogger) Info(msg string) error {
	l.Println(msg)
	return nil
}
func TestReadEventsErrDisconnect(t *testing.T) {
	mckConn := new(mockConn)
	kea := &KamEvapi{
		errReadEvents: make(chan error, 1),
		connMutex:     &sync.RWMutex{},
		conn:          mckConn,
		logger:        nopLogger{},
	}
	kea.errReadEvents <- io.EOF
	errExpect := "error"
	var capturedLogBuffer bytes.Buffer
	originalLogger := kea.logger
	kea.logger = &testLogger{log.New(&capturedLogBuffer, "", 0)}
	bufExpect := "Disconnecting from"
	if err := kea.ReadEvents(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	} else if rcv := capturedLogBuffer.String(); !strings.Contains(rcv, bufExpect) {
		t.Errorf("Expected %s but received %s", bufExpect, rcv)
	}
	kea.logger = originalLogger
}

func TestReadEventsErrNotConnected(t *testing.T) {
	c1, _ := net.Pipe()
	kea := &KamEvapi{
		errReadEvents: make(chan error, 1),
		connMutex:     &sync.RWMutex{},
		conn:          c1,
		logger:        nopLogger{},
		delayFunc:     fibDuration,
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
		logger:        nopLogger{},
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

func TestReadNetstringError4(t *testing.T) {
	kea := &KamEvapi{
		rcvBuffer: bufio.NewReader(bytes.NewBufferString("10:string,")), //It is not valid because it is not "7:string!,"
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
		delayFunc: fibDuration,
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
		delayFunc:  fibDuration,
	}
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
		delayFunc:  fibDuration,
	}
	errExpect := "KamEvapi dial tcp 127.0.0.1:9435: connect: connection refused"
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
		logger: nopLogger{},
	}
	var capturedLogBuffer bytes.Buffer
	originalLogger := kea.logger
	kea.logger = &testLogger{log.New(&capturedLogBuffer, "", 0)}
	bufExpect := "No handler for inbound data: test"
	kea.dispatchEvent([]byte("test"))
	if rcv := capturedLogBuffer.String(); !strings.Contains(rcv, bufExpect) {
		t.Errorf("Expected %s but received %s", bufExpect, rcv)
	}
	kea.logger = originalLogger
}

// Test for KamEvapiPool
func TestNewKamEvapiPool(t *testing.T) {
	var err error
	if _, err = NewKamEvapiPool(2, "127.0.0.1:9435", 0, 3, 0, fibDuration, new(nopLogger)); err != nil {
		t.Fatal("Could not create KamEvapi, error: ", err)
	}
}

func TestPushKamEvapi(t *testing.T) { //When keap nor kea are nil
	conns := make(chan *KamEvapi, 1)
	keap := &KamEvapiPool{
		allowedConns: make(chan struct{}, 1),
		conns:        conns,
	}
	kea := makeNewKea()
	keap.PushKamEvapi(kea)
	// fmt.Println(<-keap.conns)
	// fmt.Println(kea)
	if !reflect.DeepEqual(kea, <-keap.conns) {
		t.Errorf("Expected %v but received %v", kea, keap.conns)
	}
}

func TestPushKamEvapiErr1(t *testing.T) { //When kea is not connected
	conns := make(chan *KamEvapi, 1)
	keap := &KamEvapiPool{
		allowedConns: make(chan struct{}, 1),
		conns:        conns,
	}
	kea := makeNewKea()
	kea.conn = nil
	keap.PushKamEvapi(kea)
}

func TestPushKamEvapiErr2(t *testing.T) { //When keap is nil
	var kp *KamEvapiPool
	kea := makeNewKea()
	kp.PushKamEvapi(kea)
}

func TestPopKamEvapiErr(t *testing.T) { //When can't connect to Kameilio
	conns := make(chan *KamEvapi, 1)
	keap := &KamEvapiPool{
		allowedConns: make(chan struct{}, 1),
		conns:        conns,
		kamAddr:      "127.0.0.1:9435",
		connIdx:      0,
		reconnects:   2,
		logger:       nopLogger{},
	}
	var test struct{}
	keap.allowedConns <- test
	errExpect := "KamEvapi dial tcp 127.0.0.1:9435: connect: connection refused"
	if _, err := keap.PopKamEvapi(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func TestPopKamEvapi(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:9435")
	if err != nil {
		t.Fatal(err)
	}
	go l.Accept()
	defer l.Close()
	conns := make(chan *KamEvapi, 1)
	keap := &KamEvapiPool{
		allowedConns: make(chan struct{}, 1),
		conns:        conns,
		kamAddr:      "127.0.0.1:9435",
		connIdx:      0,
		reconnects:   2,
		logger:       nopLogger{},
	}
	var idk struct{}
	keap.allowedConns <- idk
	if _, err := keap.PopKamEvapi(); err != nil {
		t.Error(err)
	}
}
func TestPopKamEvapi2(t *testing.T) { //When len(keap.conns) != 0
	l, err := net.Listen("tcp", "127.0.0.1:9435")
	if err != nil {
		t.Fatal(err)
	}
	go l.Accept()
	defer l.Close()
	conns := make(chan *KamEvapi, 1)
	keap := &KamEvapiPool{
		allowedConns: make(chan struct{}, 1),
		conns:        conns,
		kamAddr:      "127.0.0.1:9435",
		connIdx:      0,
		reconnects:   2,
		logger:       nopLogger{},
	}
	// var idk struct{}
	// keap.allowedConns <- idk
	keap.conns <- &KamEvapi{}
	if _, err := keap.PopKamEvapi(); err != nil {
		t.Error(err)
	}
}

func TestPopKamEvapi3(t *testing.T) {
	var kp *KamEvapiPool
	errExpect := "UNCONFIGURED_KAMAILIO_POOL"
	if _, err := kp.PopKamEvapi(); err == nil || err.Error() != errExpect {
		t.Errorf("Expected %v but received %v", errExpect, err)
	}
}

func fibDuration(durationUnit, maxDuration time.Duration) func() time.Duration {
	a, b := 0, 1
	return func() time.Duration {
		a, b = b, a+b
		fibNrAsDuration := time.Duration(a) * durationUnit
		if maxDuration > 0 && maxDuration < fibNrAsDuration {
			return maxDuration
		}
		return fibNrAsDuration
	}
}
