package conn

import (
	"bufio"
	"io"
	"sync"
)

const teeBufferSize = 32

type asyncPipeWriter struct {
	pipe *io.PipeWriter
	ch   chan []byte
	once sync.Once
}

func newAsyncPipeWriter(pipe *io.PipeWriter) *asyncPipeWriter {
	w := &asyncPipeWriter{
		pipe: pipe,
		ch:   make(chan []byte, teeBufferSize),
	}
	go func() {
		for b := range w.ch {
			if _, err := w.pipe.Write(b); err != nil {
				return
			}
		}
	}()
	return w
}

func (w *asyncPipeWriter) Write(b []byte) {
	cp := make([]byte, len(b))
	copy(cp, b)
	select {
	case w.ch <- cp:
	default:
	}
}

func (w *asyncPipeWriter) Close() {
	w.once.Do(func() {
		close(w.ch)
		w.pipe.Close()
	})
}

// conn.Tee is a wraps a conn.Conn
// causing all writes/reads to be tee'd just
// like the unix command such that all data that
// is read and written to the connection through its
// interfaces will also be copied into two dedicated pipes
// used for consuming a copy of the data stream
//
// this is useful for introspecting the traffic flowing
// over a connection without having to tamper with the actual
// code that reads and writes over the connection
//
// NB: copied inspection data is best-effort. If consumers fall behind,
// inspection bytes are dropped instead of blocking real traffic.

type Tee struct {
	readPipe struct {
		rd *io.PipeReader
		wr *asyncPipeWriter
	}
	writePipe struct {
		rd *io.PipeReader
		wr *asyncPipeWriter
	}
	Conn
}

func NewTee(conn Conn) *Tee {
	c := &Tee{
		Conn: conn,
	}

	var readPipeWriter *io.PipeWriter
	var writePipeWriter *io.PipeWriter
	c.readPipe.rd, readPipeWriter = io.Pipe()
	c.writePipe.rd, writePipeWriter = io.Pipe()

	c.readPipe.wr = newAsyncPipeWriter(readPipeWriter)
	c.writePipe.wr = newAsyncPipeWriter(writePipeWriter)
	return c
}

func (c *Tee) ReadBuffer() *bufio.Reader {
	return bufio.NewReader(c.readPipe.rd)
}

func (c *Tee) WriteBuffer() *bufio.Reader {
	return bufio.NewReader(c.writePipe.rd)
}

func (c *Tee) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if n > 0 {
		c.readPipe.wr.Write(b[:n])
	}
	if err != nil {
		c.readPipe.wr.Close()
	}
	return
}

func (c *Tee) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if n > 0 {
		c.writePipe.wr.Write(b[:n])
	}
	if err != nil {
		c.writePipe.wr.Close()
	}
	return
}
