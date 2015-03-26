/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

Provides Kamailio evapi socket communication.
*/

package kamevapi

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/syslog"
	"net"
	"regexp"
	"strconv"
	"time"
)

// successive Fibonacci numbers.
func fib() func() int {
	a, b := 0, 1
	return func() int {
		a, b = b, a+b
		return a
	}
}

// Creates a new kamEvApi, connects it and in case forkRead is enabled starts listening in background
func NewKamEvapi(addr, connId string, recons int, eventHandlers map[*regexp.Regexp][]func([]byte, string), logger *syslog.Writer) (*KamEvapi, error) {
	kea := &KamEvapi{
		kamaddr:       addr,
		connId:        connId,
		reconnects:    recons,
		eventHandlers: eventHandlers,
		logger:        logger,
		delayFunc:     fib(),
		ping:          true,
		ping_delay:    20}
	if err := kea.Connect(); err != nil {
		return nil, err
	}
	return kea, nil
}

type KamEvapi struct {
	kamaddr        string // IP:Port address where to reach kamailio
	connId         string // Optional connection identifier between library and component using it
	reconnects     int
	eventHandlers  map[*regexp.Regexp][]func([]byte, string)
	logger         *syslog.Writer
	delayFunc      func() int
	conn           net.Conn
	rcvBuffer      *bufio.Reader
	dataInChan     chan string   // Listen here for replies from Kamailio
	stopReadEvents chan struct{} //Keep a reference towards forkedReadEvents so we can stop them whenever necessary
	errReadEvents  chan error
	ping           bool
	ping_delay     int
}

// Reads bytes from the buffer and dispatch content received as netstring
func (kea *KamEvapi) readNetstring() ([]byte, error) {
	contentLenStr, err := kea.rcvBuffer.ReadString(':')
	if err != nil {
		return nil, err
	}
	cntLen, err := strconv.Atoi(contentLenStr[:len(contentLenStr)-1])
	if err != nil {
		return nil, err
	}
	bytesRead := make([]byte, cntLen)
	for i := 0; i < cntLen; i++ {
		byteRead, err := kea.rcvBuffer.ReadByte()
		if err != nil {
			return nil, err
		}
		bytesRead[i] = byteRead
	}
	if byteRead, err := kea.rcvBuffer.ReadByte(); err != nil { // Crosscheck that our received content ends in , which is standard for netstrings
		return nil, err
	} else if byteRead != ',' {
		return nil, fmt.Errorf("Crosschecking netstring failed, no comma in the end but: %s", string(byteRead))
	}
	return bytesRead, nil
}

// Reads netstrings from socket, dispatch content
func (kea *KamEvapi) readEvents(exitChan chan struct{}, errReadEvents chan error) {
	for {
		select {
		case <-exitChan:
			return
		default: // Unlock waiting here
		}
		dataIn, err := kea.readNetstring()
		if err != nil {
			errReadEvents <- err
			return
		}
		kea.dispatchEvent(dataIn)
	}
	return
}

// Formats string content as netstring and sends over the socket
func (kea *KamEvapi) sendAsNetstring(dataStr string) error {
	cntLen := len([]byte(dataStr)) // Netstrings require number of bytes sent
	dataOut := fmt.Sprintf("%d:%s,", cntLen, dataStr)
	_, err := fmt.Fprint(kea.conn, dataOut)
	if err != nil {
		kea.logger.Err(fmt.Sprintf("<KamEvapi> Socket invalid (%s)", err))
		kea.Disconnect()
		kea.ReconnectIfNeeded()
	}
	return nil
}

// Dispatch the event received from Kamailio towards handlers matching it
func (kea *KamEvapi) dispatchEvent(dataIn []byte) {
	matched := false
	for matcher, handlers := range kea.eventHandlers {
		if matcher.Match(dataIn) {
			matched = true
			for _, f := range handlers {
				go f(dataIn, kea.connId)
			}
		}
	}
	if !matched {
		kea.logger.Warning(fmt.Sprintf("<KamEvapi> WARNING: No handler for inbound data: %s", dataIn))
	}
}

// Checks if socket connected. Can be extended with pings
func (kea *KamEvapi) Connected() bool {
	if kea.conn == nil {
		return false
	}
	return true
}

// Disconnects from socket
func (kea *KamEvapi) Disconnect() (err error) {
	if kea.conn != nil {
		err = kea.conn.Close()
		kea.conn = nil
	}
	return
}

// Connect or reconnect
func (kea *KamEvapi) Connect() error {
	if kea.Connected() {
		kea.Disconnect()
	}
	if kea.stopReadEvents != nil { // ToDo: Check if the channel is not already closed
		close(kea.stopReadEvents) // we have read events already processing, request stop
		kea.stopReadEvents = nil
	}
	var err error
	if kea.logger != nil {
		kea.logger.Info(fmt.Sprintf("<KamEvapi> Attempting connect to Kamailio at: %s", kea.kamaddr))
	}
	if kea.conn, err = net.Dial("tcp", kea.kamaddr); err != nil {
		return err
	}
	if kea.logger != nil {
		kea.logger.Info("<KamEvapi> Successfully connected to Kamailio!")
	}
	// Connected, init buffer and prepare sync channels
	kea.rcvBuffer = bufio.NewReaderSize(kea.conn, 8192) // reinit buffer
	stopReadEvents := make(chan struct{})
	kea.stopReadEvents = stopReadEvents
	kea.errReadEvents = make(chan error)
	go kea.readEvents(stopReadEvents, kea.errReadEvents) // Fork read events in it's own goroutine
	if kea.ping {
		go kea.Ping()
	}
	return nil
}

func (kea *KamEvapi) Ping() {
	data := []byte("{\"Event\":\"ping\"}")
	cntLen := len(data)
	dataOut := fmt.Sprintf("%d:%s,", cntLen, data)

	for {
		_, err := fmt.Fprint(kea.conn, dataOut)
		if err != nil {
			kea.logger.Err(fmt.Sprintf("<KamEvapi> Invalid Socket (%s)", err))
			kea.Disconnect()
			kea.ReconnectIfNeeded()
		}
		time.Sleep(time.Duration(kea.ping_delay) * time.Second)
	}
	return
}

// If not connected, attempt reconnect if allowed
func (kea *KamEvapi) ReconnectIfNeeded() error {
	if kea.Connected() { // No need to reconnect
		return nil
	}
	if kea.reconnects == 0 { // No reconnects allowed
		return errors.New("Not connected to Kamailio")
	}

	var err error
	for i := 0; i < kea.reconnects; i++ {
		kea.logger.Info(fmt.Sprintf("<KamEvapi> Socket is disconnected. Reconnecting"))
		if err = kea.Connect(); err == nil || kea.Connected() {
			break // No error or unrelated to connection
		}
		time.Sleep(time.Duration(kea.delayFunc()) * time.Second)
	}
	return err // nil or last error in the loop
}

// Reads events from socket, attempt reconnect if disconnected
func (kea *KamEvapi) ReadEvents() (err error) {
	for {
		if err = <-kea.errReadEvents; err == io.EOF { // Disconnected, try reconnect
			if err = kea.ReconnectIfNeeded(); err != nil {
				break
			}
		}
	}
	return err
}

// Send data to Kamailio
func (kea *KamEvapi) Send(dataStr string) error {
	if err := kea.ReconnectIfNeeded(); err != nil {
		return err
	}
	return kea.sendAsNetstring(dataStr)
	//resSend := <-kea.dataInChan
	//return resSend, nil
}

// Connection handler for commands sent to FreeSWITCH
type KamEvapiPool struct {
	kamAddr      string
	connId       string
	reconnects   int
	logger       *syslog.Writer
	allowedConns chan struct{}  // Will be populated with allowed new connections
	conns        chan *KamEvapi // Keep here reference towards the list of opened sockets
}

// Retrieves a connection from the pool
func (keap *KamEvapiPool) PopKamEvapi() (*KamEvapi, error) {
	if keap == nil {
		return nil, errors.New("UNCONFIGURED_KAMAILIO_POOL")
	}
	if len(keap.conns) != 0 { // Select directly if available, so we avoid randomness of selection
		KamEvapi := <-keap.conns
		return KamEvapi, nil
	}
	var KamEvapi *KamEvapi
	var err error
	select { // No KamEvapi available in the pool, wait for first one showing up
	case KamEvapi = <-keap.conns:
	case <-keap.allowedConns:
		KamEvapi, err = NewKamEvapi(keap.kamAddr, keap.connId, keap.reconnects, nil, keap.logger)
		if err != nil {
			return nil, err
		}
		return KamEvapi, nil
	}
	return KamEvapi, nil
}

// Push the connection back to the pool
func (keap *KamEvapiPool) PushKamEvapi(kea *KamEvapi) {
	if keap == nil { // Did not initialize the pool
		return
	}
	if kea == nil || !kea.Connected() {
		keap.allowedConns <- struct{}{}
		return
	}
	keap.conns <- kea
}

// Instantiates a new KamEvapiPool
func NewKamEvapiPool(maxconns int, kamAddr, connId string, reconnects int, l *syslog.Writer) (*KamEvapiPool, error) {
	pool := &KamEvapiPool{kamAddr: kamAddr, connId: connId, reconnects: reconnects, logger: l}
	pool.allowedConns = make(chan struct{}, maxconns)
	var emptyConn struct{}
	for i := 0; i < maxconns; i++ {
		pool.allowedConns <- emptyConn // Empty initiate so we do not need to wait later when we pop
	}
	pool.conns = make(chan *KamEvapi, maxconns)
	return pool, nil
}
