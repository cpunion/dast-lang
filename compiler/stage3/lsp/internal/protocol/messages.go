package protocol

// InitializeParams contains the parameters for the initialize request.
type InitializeParams struct {
	ProcessID             int                `json:"processId"`
	RootURI               DocumentURI        `json:"rootUri"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions interface{}        `json:"initializationOptions,omitempty"`
}

// ClientCapabilities represents the capabilities provided by the client.
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

// TextDocumentClientCapabilities defines capabilities for text document features.
type TextDocumentClientCapabilities struct {
	Synchronization    *TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Completion         *CompletionClientCapabilities       `json:"completion,omitempty"`
	Hover              *HoverClientCapabilities            `json:"hover,omitempty"`
	Definition         *DefinitionClientCapabilities       `json:"definition,omitempty"`
	PublishDiagnostics *PublishDiagnosticsCapabilities     `json:"publishDiagnostics,omitempty"`
}

type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

type CompletionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type HoverClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type PublishDiagnosticsCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

type WorkspaceClientCapabilities struct {
	ApplyEdit bool `json:"applyEdit,omitempty"`
}

// InitializeResult is the result returned from the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities describes the capabilities of the language server.
type ServerCapabilities struct {
	TextDocumentSync           TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	CompletionProvider         *CompletionOptions      `json:"completionProvider,omitempty"`
	HoverProvider              bool                    `json:"hoverProvider,omitempty"`
	DefinitionProvider         bool                    `json:"definitionProvider,omitempty"`
	ReferencesProvider         bool                    `json:"referencesProvider,omitempty"`
	DocumentFormattingProvider bool                    `json:"documentFormattingProvider,omitempty"`
}

// TextDocumentSyncKind defines how the host (editor) should sync document changes.
type TextDocumentSyncKind int

const (
	SyncNone        TextDocumentSyncKind = 0
	SyncFull        TextDocumentSyncKind = 1
	SyncIncremental TextDocumentSyncKind = 2
)

// TextDocumentSyncOptions defines options for text document synchronization.
type TextDocumentSyncOptions struct {
	OpenClose bool                 `json:"openClose,omitempty"`
	Change    TextDocumentSyncKind `json:"change,omitempty"`
	Save      *SaveOptions         `json:"save,omitempty"`
}

type SaveOptions struct {
	IncludeText bool `json:"includeText,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
}

// DidOpenTextDocumentParams is sent when a text document is opened.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams is sent when a text document is changed.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams is sent when a text document is closed.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidSaveTextDocumentParams is sent when a text document is saved.
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         string                 `json:"text,omitempty"`
}

// PublishDiagnosticsParams contains the diagnostics to publish.
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
