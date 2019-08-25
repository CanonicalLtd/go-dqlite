package client

import (
	"context"
	"encoding/binary"
	"io"

	"github.com/canonical/go-dqlite/internal/protocol"
	"github.com/pkg/errors"
)

// DialFunc is a function that can be used to establish a network connection.
type DialFunc protocol.DialFunc

// Client speaks the dqlite wire protocol.
type Client struct {
	conn *protocol.Conn
}

// File contains details about a database file.
type File struct {
	Name string
	Data []byte
}

// Connect to a dqlite instance.
func Connect(ctx context.Context, dial DialFunc, address string) (*Client, error) {
	// Establish the connection.
	conn, err := dial(ctx, address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish network connection")
	}

	// Latest protocol version.
	proto := make([]byte, 8)
	binary.LittleEndian.PutUint64(proto, protocol.ProtocolVersion)

	// Perform the protocol handshake.
	n, err := conn.Write(proto)
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "failed to send handshake")
	}
	if n != 8 {
		conn.Close()
		return nil, errors.Wrap(io.ErrShortWrite, "failed to send handshake")
	}

	client := &Client{conn: protocol.NewConn(conn)}

	return client, nil
}

func (c *Client) Dump(ctx context.Context, filename string) ([]File, error) {
	request := protocol.Message{}
	request.Init(16)
	response := protocol.Message{}
	response.Init(512)

	protocol.EncodeDump(&request, filename)

	if err := c.conn.Call(ctx, &request, &response); err != nil {
		return nil, errors.Wrap(err, "failed to send dump request")
	}

	files, err := protocol.DecodeFiles(&response)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse files response")
	}
	defer files.Close()

	dump := make([]File, 0)

	for {
		name, data := files.Next()
		if name == "" {
			break
		}
		dump = append(dump, File{Name: name, Data: data})
	}

	return dump, nil
}