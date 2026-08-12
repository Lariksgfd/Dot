package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type CompletionItem struct {
	Label      string `json:"label"`
	Kind       int    `json:"kind,omitempty"`
	Detail     string `json:"detail,omitempty"`
	InsertText string `json:"insertText,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	TextDocumentSync   int                `json:"textDocumentSync"`
	CompletionProvider *CompletionOptions `json:"completionProvider,omitempty"`
}

type CompletionOptions struct {
	ResolveProvider bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

const (
	TextDocumentSyncFull = 1
	CompletionItemText   = 1
	CompletionItemMethod = 2
	CompletionItemFunc   = 3
)

type Document struct {
	URI     string
	Text    string
	Version int
}

type Server struct {
	mu        sync.Mutex
	documents map[string]*Document
	reader    *bufio.Reader
	writer    *io.Writer
	encoder   *json.Encoder
}

func NewServer(r io.Reader, w io.Writer) *Server {
	enc := json.NewEncoder(w)
	return &Server{
		documents: make(map[string]*Document),
		reader:    bufio.NewReader(r),
		writer:    &w,
		encoder:   enc,
	}
}

func (s *Server) Run() error {
	for {
		line, err := s.reader.ReadBytes('\n')
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		var contentLen int
		header := string(line)
		if _, err := fmt.Sscanf(header, "Content-Length: %d\r\n", &contentLen); err != nil {
			continue
		}

		blank, err := s.reader.ReadBytes('\n')
		if err != nil {
			return fmt.Errorf("read blank line error: %w", err)
		}
		if string(blank) != "\r\n" && string(blank) != "\n" {
			continue
		}

		body := make([]byte, contentLen)
		if _, err := io.ReadFull(s.reader, body); err != nil {
			return fmt.Errorf("read body error: %w", err)
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}

		s.handleRequest(&req)
	}
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (s *Server) sendResponse(resp *JSONRPCResponse) {
	data, _ := json.Marshal(resp)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	w := *s.writer
	w.Write([]byte(header))
	w.Write(data)
}

func (s *Server) sendNotification(method string, params interface{}) {
	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(notif)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	w := *s.writer
	w.Write([]byte(header))
	w.Write(data)
}

func (s *Server) handleRequest(req *JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// no-op notification
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	case "shutdown":
		s.handleShutdown(req)
	case "exit":
		os.Exit(0)
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) {
	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: TextDocumentSyncFull,
			CompletionProvider: &CompletionOptions{
				ResolveProvider:   true,
				TriggerCharacters: []string{".", ">", ":"},
			},
		},
		ServerInfo: ServerInfo{
			Name:    "dot-lsp",
			Version: "0.1.0",
		},
	}
	s.sendResponse(&JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func (s *Server) handleDidOpen(req *JSONRPCRequest) {
	var params struct {
		TextDocument TextDocumentItem `json:"textDocument"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}

	s.mu.Lock()
	s.documents[params.TextDocument.URI] = &Document{
		URI:     params.TextDocument.URI,
		Text:    params.TextDocument.Text,
		Version: params.TextDocument.Version,
	}
	s.mu.Unlock()
}

func (s *Server) handleDidChange(req *JSONRPCRequest) {
	var params struct {
		TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
		ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}

	s.mu.Lock()
	doc, exists := s.documents[params.TextDocument.URI]
	if !exists {
		doc = &Document{URI: params.TextDocument.URI}
		s.documents[params.TextDocument.URI] = doc
	}
	for _, change := range params.ContentChanges {
		doc.Text = change.Text
	}
	doc.Version = params.TextDocument.Version
	s.mu.Unlock()
}

func (s *Server) handleCompletion(req *JSONRPCRequest) {
	dummies := []CompletionItem{
		{Label: "fn", Kind: CompletionItemFunc, Detail: "function", InsertText: "func "},
		{Label: "struct", Kind: CompletionItemText, Detail: "struct definition", InsertText: "struct "},
		{Label: "let", Kind: CompletionItemText, Detail: "variable binding", InsertText: "let "},
		{Label: "if", Kind: CompletionItemText, Detail: "conditional", InsertText: "if "},
		{Label: "for", Kind: CompletionItemText, Detail: "loop", InsertText: "for "},
	}

	s.sendResponse(&JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: CompletionList{
			IsIncomplete: false,
			Items:        dummies,
		},
	})
}

func (s *Server) handleShutdown(req *JSONRPCRequest) {
	s.sendResponse(&JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  nil,
	})
}
