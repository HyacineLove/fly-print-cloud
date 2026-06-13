package websocket

import (
	"encoding/json"
	"testing"

	"fly-print-cloud/api/internal/models"
)

const testContentHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDispatchPreviewFileIncludesContentHash(t *testing.T) {
	t.Parallel()

	conn := &Connection{
		NodeID: "node-1",
		Send:   make(chan []byte, 1),
	}
	manager := &ConnectionManager{
		connections: map[string]*Connection{"node-1": conn},
	}

	err := manager.DispatchPreviewFile(
		"node-1",
		"file-1",
		"/api/v1/files/file-1",
		"sample.pdf",
		123,
		"application/pdf",
		testContentHash,
	)
	if err != nil {
		t.Fatalf("DispatchPreviewFile() error = %v", err)
	}

	var msg Message
	if err := json.Unmarshal(<-conn.Send, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("message data type = %T, want map", msg.Data)
	}
	if data["content_hash"] != testContentHash {
		t.Fatalf("content_hash = %v, want %s", data["content_hash"], testContentHash)
	}
}

func TestDispatchPrintJobIncludesContentHash(t *testing.T) {
	t.Parallel()

	conn := &Connection{
		NodeID:      "node-1",
		Send:        make(chan []byte, 1),
		pendingAcks: make(map[string]chan struct{}),
	}
	manager := &ConnectionManager{
		connections: map[string]*Connection{"node-1": conn},
	}
	job := &models.PrintJob{
		ID:          "job-1",
		Name:        "sample.pdf",
		PrinterID:   "printer-1",
		FileURL:     "/api/v1/files/file-1",
		ContentHash: testContentHash,
		Copies:      1,
		MaxRetries:  3,
	}

	received := make(chan Command, 1)
	go func() {
		var cmd Command
		if err := json.Unmarshal(<-conn.Send, &cmd); err == nil {
			received <- cmd
			conn.handleAckDirect(&CommandAck{MsgID: cmd.MsgID, CommandID: cmd.CommandID})
		}
	}()

	if err := manager.DispatchPrintJob("node-1", job, "Printer"); err != nil {
		t.Fatalf("DispatchPrintJob() error = %v", err)
	}

	cmd := <-received
	data, ok := cmd.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("command data type = %T, want map", cmd.Data)
	}
	if data["content_hash"] != testContentHash {
		t.Fatalf("content_hash = %v, want %s", data["content_hash"], testContentHash)
	}
}
