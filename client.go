package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"time"
)

// ////////////////////////////////////////////////////////////////////////////////// //

type req struct {
	cmd  string
	args []interface{}
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Client describes a Redis client
type Client struct {
	Network      string
	Addr         string
	TLSConfig    *tls.Config
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	DialTimeout  time.Duration
	LastCritical error

	conn         net.Conn
	respReader   *RespReader
	writeScratch []byte
	writeBuf     *bytes.Buffer

	pending       []req
	completed     []*Resp
	completedHead []*Resp
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Errors
var (
	ErrEmptyPipeline     = errors.New("Pipeline is empty")
	ErrNotConnected      = errors.New("Client not connected")
	ErrWrongConfResponse = errors.New("CONFIG command response must have Array type")
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Connect connect to Redis instance
func (c *Client) Connect() error {
	var err error

	if c.Network == "" {
		c.Network = "tcp"
	}

	switch {
	case c.TLSConfig != nil:
		c.conn, err = tls.Dial(c.Network, c.Addr, c.TLSConfig)
	case c.DialTimeout > 0:
		c.conn, err = net.DialTimeout(c.Network, c.Addr, c.DialTimeout)
	default:
		c.conn, err = net.Dial(c.Network, c.Addr)
	}

	if err != nil {
		return err
	}

	c.respReader = NewRespReader(c.conn)

	// if write buffer already exist just clear it and reuse
	if c.writeBuf != nil {
		c.writeBuf.Reset()
	} else {
		c.writeBuf = bytes.NewBuffer(make([]byte, 0, 128))
	}

	completed := make([]*Resp, 0, 10)

	c.completed = completed
	c.completedHead = completed

	return nil
}

// Cmd calls the given Redis command
func (c *Client) Cmd(cmd string, args ...interface{}) *Resp {
	if c.conn == nil {
		resp := errToResp(IOErr, ErrNotConnected)
		return &resp
	}

	err := c.writeRequest(req{cmd, args})

	if err != nil {
		resp := errToResp(IOErr, err)
		return &resp
	}

	return c.readResp(true)
}

// PipeAppend adds the given call to the pipeline queue
func (c *Client) PipeAppend(cmd string, args ...interface{}) {
	c.pending = append(c.pending, req{cmd, args})
}

// PipeResp returns the reply for the next request in the pipeline queue
func (c *Client) PipeResp() *Resp {
	if c.conn == nil {
		resp := errToResp(IOErr, ErrNotConnected)
		return &resp
	}

	if len(c.completed) > 0 {
		resp := c.completed[0]
		c.completed = c.completed[1:]
		return resp
	}

	if len(c.pending) == 0 {
		resp := errToResp(RedisErr, ErrEmptyPipeline)
		return &resp
	}

	nreqs := len(c.pending)
	err := c.writeRequest(c.pending...)

	c.pending = nil

	if err != nil {
		resp := errToResp(IOErr, err)
		return &resp
	}

	c.completed = c.completedHead

	for i := 0; i < nreqs; i++ {
		resp := c.readResp(true)
		c.completed = append(c.completed, resp)
	}

	return c.PipeResp()
}

// PipeClear clears the contents of the current pipeline queue, both commands
// queued by PipeAppend which have yet to be sent and responses which have yet
// to be retrieved through PipeResp
func (c *Client) PipeClear() (int, int) {
	callCount, replyCount := len(c.pending), len(c.completed)

	if callCount > 0 {
		c.pending = nil
	}

	if replyCount > 0 {
		c.completed = nil
	}

	return callCount, replyCount
}

// GetConfig read and parse full in-memory config
func (c *Client) GetConfig(configCommand string) (*Config, error) {
	resp := c.Cmd(configCommand, "GET", "*")

	if resp.Err != nil {
		return nil, resp.Err
	}

	if !resp.IsType(Array) {
		return nil, ErrWrongConfResponse
	}

	return parseInMemoryConfig(resp)
}

// Close closes the connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// ////////////////////////////////////////////////////////////////////////////////// //

func (c *Client) writeRequest(requests ...req) error {
	if c.ReadTimeout != 0 {
		c.conn.SetReadDeadline(getDeadline(c.WriteTimeout))
	}

	var err error

MAINLOOP:
	for _, r := range requests {
		c.writeBuf.Reset()
		elems := flattenedLength(r.args...) + 1

		_, err = writeArrayHeader(c.writeBuf, c.writeScratch, elems)

		if err != nil {
			break
		}

		_, err = writeTo(c.writeBuf, c.writeScratch, r.cmd, true, true)

		if err != nil {
			break
		}

		for _, arg := range r.args {
			_, err = writeTo(c.writeBuf, c.writeScratch, arg, true, true)

			if err != nil {
				break MAINLOOP
			}
		}

		_, err = c.writeBuf.WriteTo(c.conn)

		if err != nil {
			break MAINLOOP
		}
	}

	if err != nil {
		c.LastCritical = err
		c.Close()
	}

	return err
}

func (c *Client) readResp(strict bool) *Resp {
	if c.ReadTimeout != 0 {
		c.conn.SetReadDeadline(getDeadline(c.ReadTimeout))
	}

	resp := c.respReader.Read()

	if resp.IsType(IOErr) && (strict || !isTimeout(resp)) {
		c.LastCritical = resp.Err
		c.Close()
	}

	return resp
}

func getDeadline(timeout time.Duration) time.Time {
	return time.Now().Add(timeout)
}
