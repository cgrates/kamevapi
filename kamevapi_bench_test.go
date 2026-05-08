/*
Released under MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM GmbH. All Rights Reserved.

Provides Kamailio evapi socket communication.
*/

package kamevapi

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
)

func BenchmarkReadNetstring(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"500B", 500},
		{"4KB", 4 << 10},
		{"64KB", 64 << 10},
		{"1MB", 1 << 20},
		{"30MB", 30 << 20},
	}
	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			payload := bytes.Repeat([]byte("x"), s.size)
			netstring := fmt.Appendf(nil, "%d:%s,", len(payload), payload)
			b.SetBytes(int64(len(netstring)))
			for b.Loop() {
				kea := &KamEvapi{rcvBuffer: bufio.NewReaderSize(bytes.NewReader(netstring), 8192)}
				if _, err := kea.readNetstring(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
