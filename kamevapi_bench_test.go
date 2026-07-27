// Copyright ITsysCOM GmbH
// SPDX-License-Identifier: MIT

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
			r := bytes.NewReader(netstring)
			buf := bufio.NewReaderSize(r, 8192)
			kea := &KamEvapi{rcvBuffer: buf}
			b.SetBytes(int64(len(netstring)))
			for b.Loop() {
				r.Reset(netstring)
				buf.Reset(r)
				if _, err := kea.readNetstring(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
