// Package server implements the LSP server core.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/cpunion/dastlang/lsp/internal/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// Server is the main LSP server.
type Server struct {
	conn *jsonrpc2.Conn

	// Document state
	mu        sync.RWMutex
	documents map[protocol.DocumentURI]*Document

	// Root URI of the workspace
	rootURI protocol.DocumentURI

	// Initialized flag
	initialized bool
}

// Document represents an open text document.
type Document struct {
	URI     protocol.DocumentURI
	Version int
	Content string
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{
		documents: make(map[protocol.DocumentURI]*Document),
	}
}

// SetConnection sets the JSON-RPC connection.
func (s *Server) SetConnection(conn *jsonrpc2.Conn) {
	s.conn = conn
}

// Handle implements jsonrpc2.Handler.
func (s *Server) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	result, err := s.handleRequest(ctx, req)
	if err != nil {
		if req.Notif {
			log.Printf("notification error: %v", err)
			return
		}
		if respErr := conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		}); respErr != nil {
			log.Printf("reply error: %v", respErr)
		}
		return
	}

	if req.Notif {
		return
	}

	if err := conn.Reply(ctx, req.ID, result); err != nil {
		log.Printf("reply error: %v", err)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, req)
	case "initialized":
		return s.handleInitialized(ctx, req)
	case "shutdown":
		return s.handleShutdown(ctx, req)
	case "exit":
		return s.handleExit(ctx, req)
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, req)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, req)
	default:
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}
}

func (s *Server) handleInitialize(ctx context.Context, req *jsonrpc2.Request) (*protocol.InitializeResult, error) {
	var params protocol.InitializeParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.rootURI = params.RootURI
	s.mu.Unlock()

	log.Printf("Initialize: rootURI=%s", params.RootURI)

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.SyncFull, // Start with full sync, optimize later
				Save: &protocol.SaveOptions{
					IncludeText: false,
				},
			},
			HoverProvider:      false, // Phase 3
			DefinitionProvider: false, // Phase 3
		},
	}, nil
}

func (s *Server) handleInitialized(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	log.Println("Server initialized")
	return nil, nil
}

func (s *Server) handleShutdown(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	log.Println("Shutdown requested")
	return nil, nil
}

func (s *Server) handleExit(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	log.Println("Exit requested")
	return nil, nil
}

func (s *Server) handleDidOpen(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	doc := &Document{
		URI:     params.TextDocument.URI,
		Version: params.TextDocument.Version,
		Content: params.TextDocument.Text,
	}

	s.mu.Lock()
	s.documents[doc.URI] = doc
	s.mu.Unlock()

	log.Printf("Document opened: %s (version %d, %d bytes)",
		doc.URI, doc.Version, len(doc.Content))

	// Trigger diagnostics
	go s.publishDiagnostics(ctx, doc.URI)

	return nil, nil
}

func (s *Server) handleDidChange(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	if ok && len(params.ContentChanges) > 0 {
		// Full sync: use the last change which contains the full text
		doc.Content = params.ContentChanges[len(params.ContentChanges)-1].Text
		doc.Version = params.TextDocument.Version
	}
	s.mu.Unlock()

	if ok {
		log.Printf("Document changed: %s (version %d)", params.TextDocument.URI, params.TextDocument.Version)
		// Trigger diagnostics
		go s.publishDiagnostics(ctx, params.TextDocument.URI)
	}

	return nil, nil
}

func (s *Server) handleDidClose(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	s.mu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.mu.Unlock()

	log.Printf("Document closed: %s", params.TextDocument.URI)

	// Clear diagnostics for closed document
	s.clearDiagnostics(ctx, params.TextDocument.URI)

	return nil, nil
}

func (s *Server) handleDidSave(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	log.Printf("Document saved: %s", params.TextDocument.URI)

	// Re-trigger diagnostics on save
	go s.publishDiagnostics(ctx, params.TextDocument.URI)

	return nil, nil
}

// publishDiagnostics publishes diagnostics for a document.
func (s *Server) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok {
		return
	}

	// TODO: Call Dast compiler to get diagnostics
	// For now, return empty diagnostics
	diagnostics := s.analyzeDocument(doc)

	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	if s.conn != nil {
		if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", params); err != nil {
			log.Printf("Failed to publish diagnostics: %v", err)
		}
	}
}

// clearDiagnostics clears diagnostics for a document.
func (s *Server) clearDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []protocol.Diagnostic{},
	}

	if s.conn != nil {
		if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", params); err != nil {
			log.Printf("Failed to clear diagnostics: %v", err)
		}
	}
}

// analyzeDocument analyzes a document and returns diagnostics.
// TODO: This is a placeholder - integrate with Dast compiler.
func (s *Server) analyzeDocument(doc *Document) []protocol.Diagnostic {
	// Placeholder: return empty diagnostics
	// In Phase 2, this will call the Dast parser/typechecker
	return []protocol.Diagnostic{}
}

// GetDocument returns a document by URI.
func (s *Server) GetDocument(uri protocol.DocumentURI) (*Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.documents[uri]
	return doc, ok
}
