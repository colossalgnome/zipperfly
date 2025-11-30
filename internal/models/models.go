package models

import "io"

// DownloadRecord represents a download entry from the database
type DownloadRecord struct {
	ID            string            `json:"id"`
	Bucket        string            `json:"bucket"`
	Objects       []string          `json:"objects"`
	Name          string            `json:"name,omitempty"`
	Callback      string            `json:"callback,omitempty"`
	Password      string            `json:"password,omitempty"`       // Optional ZIP password
	CustomHeaders map[string]string `json:"custom_headers,omitempty"` // Optional custom HTTP headers
}

// CallbackPayload is sent to the callback URL after processing
type CallbackPayload struct {
	ID                  string `json:"id"`
	Status              string `json:"status"`
	Timestamp           string `json:"timestamp"`
	Message             string `json:"message,omitempty"`
	DurationMs          int64  `json:"duration_ms"`
	FileCount           int    `json:"file_count"`
	CompressedSizeBytes int64  `json:"compressed_size_bytes"`
}

// ByteCounter wraps an io.Writer and counts bytes written
type ByteCounter struct {
	Writer io.Writer
	Count  int64
}

func (bc *ByteCounter) Write(p []byte) (int, error) {
	n, err := bc.Writer.Write(p)
	bc.Count += int64(n)
	return n, err
}
