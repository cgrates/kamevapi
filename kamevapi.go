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
	"log"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// NewKamEvapi creates a new kamEvApi, connects it and in case forkRead is enabled starts listening in background
func NewKamEvapi(addr string, connIdx int, recons int, maxReconnectInterval time.Duration,
	delayFunc func(time.Duration, time.Duration) func() time.Duration,
	eventHandlers map[*regexp.Regexp][]func([]byte, int), logger *log.Logger) (*KamEvapi, error) {
	kea := &KamEvapi{
		kamaddr:              addr,
		connIdx:              connIdx,
		reconnects:           recons,
		maxReconnectInterval: maxReconnectInterval,
		eventHandlers:        eventHandlers,
		logger:               logger,
		delayFunc:            delayFunc,
		connMutex:            new(sync.RWMutex),
	}
	if err := kea.Connect(); err != nil {
		return nil, err
	}
	return kea, nil
}

type KamEvapi struct {
	kamaddr              string // IP:Port address where to reach kamailio
	connIdx              int    // Optional connection identifier between library and component using it
	reconnects           int
	maxReconnectInterval time.Duration
	eventHandlers        map[*regexp.Regexp][]func([]byte, int)
	logger               *log.Logger
	delayFunc            func(time.Duration, time.Duration) func() time.Duration // used to create/reset the delay function
	conn                 net.Conn
	rcvBuffer            *bufio.Reader
	dataInChan           chan string   // Listen here for replies from Kamailio
	stopReadEvents       chan struct{} //Keep a reference towards forkedReadEvents so we can stop them whenever necessary
	errReadEvents        chan error
	connMutex            *sync.RWMutex
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
}

// Formats string content as netstring and sends over the socket
func (kea *KamEvapi) sendAsNetstring(dataStr string) error {
	cntLen := len([]byte(dataStr)) // Netstrings require number of bytes sent
	dataOut := fmt.Sprintf("%d:%s,", cntLen, dataStr)
	kea.connMutex.RLock()
	_, err := fmt.Fprint(kea.conn, dataOut)
	if err != nil {
		return err
	}
	kea.connMutex.RUnlock()
	return nil
}

// Dispatch the event received from Kamailio towards handlers matching it
func (kea *KamEvapi) dispatchEvent(dataIn []byte) {
	matched := false
	for matcher, handlers := range kea.eventHandlers {
		if matcher.Match(dataIn) {
			matched = true
			for _, f := range handlers {
				go f(dataIn, kea.connIdx)
			}
		}
	}
	if !matched {
		kea.logger.Printf(fmt.Sprintf("WARNING: No handler for inbound data: %s", dataIn))
	}
}

// Connected checks if socket connected. Can be extended with pings
func (kea *KamEvapi) Connected() bool {
	kea.connMutex.RLock()
	defer kea.connMutex.RUnlock()
	return kea.conn != nil
}

// Disconnect disconnects from socket
func (kea *KamEvapi) Disconnect() (err error) {
	kea.connMutex.Lock()
	defer kea.connMutex.Unlock()
	if kea.conn != nil {
		kea.logger.Printf(fmt.Sprintf(" Disconnecting from %s", kea.kamaddr))
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
		kea.logger.Printf(fmt.Sprintf(" Attempting connect to Kamailio at: %s", kea.kamaddr))
	}

	conn, err := net.Dial("tcp", kea.kamaddr)
	if err != nil {
		if kea.logger != nil {
			kea.logger.Printf(
				fmt.Sprintf("<KamEvapi> Failed connecting to Kamailio at: %s, error: %s",
					kea.kamaddr, err))
		}
		return err
	}
	kea.connMutex.Lock()
	kea.conn = conn
	kea.connMutex.Unlock()
	if kea.logger != nil {
		kea.logger.Printf(fmt.Sprintf(" Successfully connected to %s!", kea.kamaddr))
	}
	// Connected, init buffer and prepare sync channels
	kea.connMutex.RLock()
	kea.rcvBuffer = bufio.NewReaderSize(kea.conn, 8192) // reinit buffer
	kea.connMutex.RUnlock()
	stopReadEvents := make(chan struct{})
	kea.stopReadEvents = stopReadEvents
	kea.errReadEvents = make(chan error)
	go kea.readEvents(stopReadEvents, kea.errReadEvents) // Fork read events in it's own goroutine
	return nil                                           // Connected
}

// ReconnectIfNeeded if not connected, attempt reconnect if allowed
func (kea *KamEvapi) ReconnectIfNeeded() error {
	if kea.Connected() { // No need to reconnect
		return nil
	}
	var err error
	delay := kea.delayFunc(time.Second, kea.maxReconnectInterval)
	for i := 0; kea.reconnects == -1 || i < kea.reconnects; i++ {
		if err = kea.Connect(); err == nil || kea.Connected() {
			break // No error or unrelated to connection
		}
		time.Sleep(delay())
	}
	if err == nil && !kea.Connected() {
		return errors.New("Not connected to Kamailio")
	}
	return err // nil or last error in the loop
}

// ReadEvents reads events from socket, attempt reconnect if disconnected
func (kea *KamEvapi) ReadEvents() (err error) {
	for {
		if err = <-kea.errReadEvents; err == io.EOF { // Disconnected, try reconnect
			if err = kea.Disconnect(); err != nil {
				break
			}
			if err = kea.ReconnectIfNeeded(); err != nil {
				break
			}
		} else if err != nil {
			break // return any error different from io.EOF
		}
	}
	return
}

// RemoteAddr returns the connection address if is connected
func (kea *KamEvapi) RemoteAddr() net.Addr {
	if !kea.Connected() {
		return nil
	}
	return kea.conn.RemoteAddr()
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
	kamAddr              string
	connIdx              int
	reconnects           int
	maxReconnectInterval time.Duration
	delayFuncConstructor func(time.Duration, time.Duration) func() time.Duration
	logger               *log.Logger
	allowedConns         chan struct{}  // Will be populated with allowed new connections
	conns                chan *KamEvapi // Keep here reference towards the list of opened sockets
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
		KamEvapi, err = NewKamEvapi(keap.kamAddr, keap.connIdx, keap.reconnects, keap.maxReconnectInterval,
			keap.delayFuncConstructor, nil, keap.logger)
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
func NewKamEvapiPool(maxconns int, kamAddr string, connIdx, reconnects int, maxReconnectInterval time.Duration,
	delayFuncConstructor func(time.Duration, time.Duration) func() time.Duration, l *log.Logger) (*KamEvapiPool, error) {
	pool := &KamEvapiPool{kamAddr: kamAddr, connIdx: connIdx, reconnects: reconnects, logger: l}
	pool.allowedConns = make(chan struct{}, maxconns)
	var emptyConn struct{}
	for i := 0; i < maxconns; i++ {
		pool.allowedConns <- emptyConn // Empty initiate so we do not need to wait later when we pop
	}
	pool.conns = make(chan *KamEvapi, maxconns)
	return pool, nil
}
