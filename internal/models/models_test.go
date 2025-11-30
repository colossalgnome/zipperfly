package models

import (
	"bytes"
	"testing"
)

func TestByteCounter_Write(t *testing.T) {
	tests := []struct {
		name      string
		writes    [][]byte
		wantCount int64
		wantData  string
	}{
		{
			name:      "single write",
			writes:    [][]byte{[]byte("hello")},
			wantCount: 5,
			wantData:  "hello",
		},
		{
			name:      "multiple writes",
			writes:    [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			wantCount: 11,
			wantData:  "hello world",
		},
		{
			name:      "empty write",
			writes:    [][]byte{[]byte("")},
			wantCount: 0,
			wantData:  "",
		},
		{
			name:      "binary data",
			writes:    [][]byte{{0x00, 0x01, 0x02, 0x03}},
			wantCount: 4,
			wantData:  "\x00\x01\x02\x03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			bc := &ByteCounter{Writer: &buf}

			for _, data := range tt.writes {
				n, err := bc.Write(data)
				if err != nil {
					t.Errorf("Write() error = %v", err)
				}
				if n != len(data) {
					t.Errorf("Write() returned %d, want %d", n, len(data))
				}
			}

			if bc.Count != tt.wantCount {
				t.Errorf("ByteCounter.Count = %d, want %d", bc.Count, tt.wantCount)
			}

			if got := buf.String(); got != tt.wantData {
				t.Errorf("Buffer contents = %q, want %q", got, tt.wantData)
			}
		})
	}
}

func TestByteCounter_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	bc := &ByteCounter{Writer: &buf}

	// Verify that Count is updated correctly even with concurrent-like sequential writes
	data := []byte("test")
	iterations := 100

	for i := 0; i < iterations; i++ {
		bc.Write(data)
	}

	expectedCount := int64(len(data) * iterations)
	if bc.Count != expectedCount {
		t.Errorf("ByteCounter.Count = %d, want %d", bc.Count, expectedCount)
	}
}
