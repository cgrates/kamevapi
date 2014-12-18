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
func NewKamEvapi(addr string, recons int, logger *syslog.Writer) (*KamEvapi, error) {
	kea := &KamEvapi{kamaddr: addr, reconnects: recons, logger: logger, delayFunc: fib()}
	if err := kea.Connect(); err != nil {
		return nil, err
	}
	return kea, nil
}

type KamEvapi struct {
	kamaddr        string // IP:Port address where to reach kamailio
	reconnects     int
	logger         *syslog.Writer
	delayFunc      func() int
	conn           net.Conn
	rcvBuffer      *bufio.Reader
	dataInChan     chan string   // Listen here for replies from Kamailio
	stopReadEvents chan struct{} //Keep a reference towards forkedReadEvents so we can stop them whenever necessary
	errReadEvents  chan error
}

// Reads bytes from the buffer and dispatch content received as netstring
func (kea *KamEvapi) readNetstring() (string, error) {
	contentLenStr, err := kea.rcvBuffer.ReadString(':')
	if err != nil {
		return "", err
	}
	cntLen, err := strconv.Atoi(contentLenStr[:len(contentLenStr)-1])
	if err != nil {
		return "", err
	}
	bytesRead := make([]byte, cntLen)
	for i := 0; i < cntLen; i++ {
		byteRead, err := kea.rcvBuffer.ReadByte()
		if err != nil {
			return "", err
		}
		bytesRead[i] = byteRead
	}
	if byteRead, err := kea.rcvBuffer.ReadByte(); err != nil { // Crosscheck that our received content ends in , which is standard for netstrings
		return "", err
	} else if byteRead != ',' {
		return "", fmt.Errorf("Crosschecking netstring failed, no comma in the end but: %s", string(byteRead))
	}
	return string(bytesRead), nil
}

// Reads netstrings from socket, dispatch content
func (kea *KamEvapi) readEvents(exitChan chan struct{}, errReadEvents chan error) {
	// Read events from buffer, firing them up further
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
		kea.logger.Info(fmt.Sprintf("Got line from Kamailio : %s", dataIn))
		//kea.dataInChan <- dataIn
	}
	return
}

// Formats string content as netstring and sends over the socket
func (kea *KamEvapi) sendAsNetstring(dataStr string) error {
	cntLen := len([]byte(dataStr)) // Netstrings require number of bytes sent
	dataOut := fmt.Sprintf("%d:%s,", cntLen, dataStr)
	fmt.Fprint(kea.conn, dataOut)
	return nil
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
	// Connected, init buffer, auth and subscribe to desired events and filters
	kea.rcvBuffer = bufio.NewReaderSize(kea.conn, 8192) // reinit buffer
	kea.stopReadEvents = make(chan struct{})
	kea.errReadEvents = make(chan error)
	go kea.readEvents(kea.stopReadEvents, kea.errReadEvents) // Fork read events in it's own goroutine
	return nil                                               // Connected
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
