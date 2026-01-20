// Command dast-lsp implements the Dast Language Server Protocol server.
package main

import (
	"context"
	"log"
	"os"

	"github.com/cpunion/dastlang/lsp/internal/server"
	"github.com/sourcegraph/jsonrpc2"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[dast-lsp] ")
	log.SetFlags(log.Ltime | log.Lshortfile)

	log.Println("Starting Dast Language Server...")

	ctx := context.Background()
	srv := server.NewServer()

	// Create stdio stream
	stream := jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{})

	// Create connection
	conn := jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(func(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
		srv.Handle(ctx, conn, req)
		return nil, nil
	}))

	srv.SetConnection(conn)

	log.Println("Listening on stdin/stdout...")

	// Wait for connection to close
	<-conn.DisconnectNotify()

	log.Println("Connection closed")
}

// stdrwc wraps stdin/stdout as a ReadWriteCloser.
type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
