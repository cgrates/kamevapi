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
func NewKamEvapi(addr string, recons int, forkRead bool, logger *syslog.Writer) (*KamEvapi, error) {
	kea := &KamEvapi{kamaddr: addr, reconnects: recons, logger: logger, delayFunc: fib()}
	if err := kea.Connect(); err != nil {
		return nil, err
	}
	if forkRead {
		go kea.ReadEvents() // Read events right after connect
	}
	return kea, nil
}

type KamEvapi struct {
	kamaddr    string // IP:Port address where to reach kamailio
	reconnects int
	logger     *syslog.Writer
	delayFunc  func() int
	conn       net.Conn
	rcvBuffer  *bufio.Reader
	dataInChan chan string // Listen here for replies from Kamailio
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
	var conErr error
	for i := 0; i < kea.reconnects; i++ {
		kea.logger.Info(fmt.Sprintf("<KamEvapi> Attempting connect to Kamailio at: %s", kea.kamaddr))
		kea.conn, conErr = net.Dial("tcp", kea.kamaddr)
		if conErr == nil {
			kea.logger.Info("<KamEvapi> Successfully connected to Kamailio!")
			// Connected, init buffer, auth and subscribe to desired events and filters
			kea.rcvBuffer = bufio.NewReaderSize(kea.conn, 8192) // reinit buffer
			return nil                                          // Connected
		}
		time.Sleep(time.Duration(kea.delayFunc()) * time.Second)
	}
	return conErr
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

// Formats string content as netstring and sends over the socket
func (kea *KamEvapi) sendAsNetstring(dataStr string) error {
	if !kea.Connected() {
		return errors.New("Not connected to Kamailio")
	}
	cntLen := len([]byte(dataStr)) // Netstrings require number of bytes sent
	dataOut := fmt.Sprintf("%d:%s,", cntLen, dataStr)
	fmt.Fprint(kea.conn, dataOut)
	return nil
}

// Reads netstrings from socket, dispatch content
func (kea *KamEvapi) ReadEvents() {
	// Read events from buffer, firing them up further
	for {
		dataIn, err := kea.readNetstring()
		if err != nil {
			kea.logger.Warning("<KamEvapi> Kamailio connection broken: attemting reconnect")
			err := kea.Connect()
			if err != nil {
				kea.logger.Warning("<KamEvapi> Reconnect fail, giving up.")
				return
			}
			continue // Connection reset
		}
		kea.logger.Info(fmt.Sprintf("Got line from Kamailio : %s", dataIn))
		//kea.dataInChan <- dataIn
	}
	return
}

// Send data to Kamailio
func (kea *KamEvapi) Send(dataStr string) error {
	return kea.sendAsNetstring(dataStr)
	//resSend := <-kea.dataInChan
	//return resSend, nil
}
