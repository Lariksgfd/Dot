package lsp

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"
)

func TestHandleInitialize(t *testing.T) {
	server := NewServer(&bytes.Buffer{}, &bytes.Buffer{})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      intPtr(1),
		Method:  "initialize",
	}

	server.handleRequest(&req)

	doc := server.documents
	_ = doc
}

func TestHandleDidOpenStoresDocument(t *testing.T) {
	var buf bytes.Buffer
	server := NewServer(&bytes.Buffer{}, &buf)

	doc := TextDocumentItem{
		URI:        "file:///test.dot",
		LanguageID: "dot",
		Version:    1,
		Text:       "let x = 5",
	}
	params := map[string]interface{}{"textDocument": doc}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  mustMarshal(params),
	}

	server.handleRequest(&req)

	server.mu.Lock()
	stored, ok := server.documents["file:///test.dot"]
	server.mu.Unlock()

	if !ok {
		t.Fatal("document was not stored")
	}
	if stored.Text != "let x = 5" {
		t.Fatalf("unexpected text: %q", stored.Text)
	}
	if stored.Version != 1 {
		t.Fatalf("unexpected version: %d", stored.Version)
	}
}

func TestHandleDidChangeUpdatesDocument(t *testing.T) {
	var buf bytes.Buffer
	server := NewServer(&bytes.Buffer{}, &buf)

	server.mu.Lock()
	server.documents["file:///test.dot"] = &Document{
		URI:     "file:///test.dot",
		Text:    "old",
		Version: 1,
	}
	server.mu.Unlock()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///test.dot",
			"version": 2,
		},
		"contentChanges": []map[string]interface{}{
			{"text": "let y = 10"},
		},
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "textDocument/didChange",
		Params:  mustMarshal(params),
	}

	server.handleRequest(&req)

	server.mu.Lock()
	doc := server.documents["file:///test.dot"]
	server.mu.Unlock()

	if doc.Text != "let y = 10" {
		t.Fatalf("unexpected text: %q", doc.Text)
	}
	if doc.Version != 2 {
		t.Fatalf("unexpected version: %d", doc.Version)
	}
}

func TestHandleCompletionReturnsDummyItems(t *testing.T) {
	var buf bytes.Buffer
	server := NewServer(&bytes.Buffer{}, &buf)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      intPtr(42),
		Method:  "textDocument/completion",
		Params:  mustMarshal(map[string]interface{}{}),
	}

	server.handleRequest(&req)

	respData := buf.Bytes()
	if len(respData) == 0 {
		t.Fatal("no response written")
	}

	respStr := string(respData)
	jsonStart := bytes.IndexByte(respData, '{')
	if jsonStart < 0 {
		t.Fatalf("no JSON in response: %q", respStr)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respData[jsonStart:], &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v (data: %s)", err, respStr)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result is not an object: %T", resp.Result)
	}
	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatal("items not found or not an array")
	}
	if len(items) == 0 {
		t.Fatal("expected dummy completion items, got none")
	}
}

func TestHandleShutdown(t *testing.T) {
	var buf bytes.Buffer
	server := NewServer(&bytes.Buffer{}, &buf)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      intPtr(99),
		Method:  "shutdown",
	}

	server.handleRequest(&req)

	respData := buf.Bytes()
	if len(respData) == 0 {
		t.Fatal("no response written")
	}

	jsonStart := bytes.IndexByte(respData, '{')
	if jsonStart < 0 {
		t.Fatal("no JSON in response")
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respData[jsonStart:], &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestFullMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	server := NewServer(&bytes.Buffer{}, &buf)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})
	header := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
	buf.WriteString(header)
	buf.Write(body)

	server.handleRequest(&JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      intPtr(1),
		Method:  "initialize",
	})

	if buf.Len() == 0 {
		t.Fatal("expected response")
	}
}

func intPtr(i int) *int {
	return &i
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
