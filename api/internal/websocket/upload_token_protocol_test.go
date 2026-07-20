package websocket

import (
	"encoding/json"
	"testing"
)

func TestUploadTokenErrorPreservesRequestID(t *testing.T) {
	connection := &Connection{Send: make(chan []byte, 1)}
	connection.sendRequestError(
		"printer_out_of_paper",
		"Printer cannot accept a new task",
		"printer-1",
		"request-1",
	)

	var message struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(<-connection.Send, &message); err != nil {
		t.Fatal(err)
	}
	if message.Type != CmdTypeError {
		t.Fatalf("unexpected message type %q", message.Type)
	}
	if message.Data["request_id"] != "request-1" {
		t.Fatalf("request id was not preserved: %#v", message.Data)
	}
}

func TestUploadTokenResponsePayloadIncludesRequestID(t *testing.T) {
	payload := UploadTokenResponsePayload{RequestID: "request-2"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["request_id"] != "request-2" {
		t.Fatalf("request id was not encoded: %#v", decoded)
	}
}
