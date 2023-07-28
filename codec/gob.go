package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobEncDec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	enc  *gob.Encoder
	dec  *gob.Decoder
}

var _ Codec = (*GobEncDec)(nil)

func (g *GobEncDec) Close() error {
	return g.conn.Close()
}

func (g *GobEncDec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g *GobEncDec) ReadBody(body any) error {
	return g.dec.Decode(body)
}

func (g *GobEncDec) Write(h *Header, body any) (err error) {
	defer func() {
		_ = g.buf.Flush()
		if err != nil {
			_ = g.Close()
		}
	}()
	if err := g.enc.Encode(h); err != nil {
		log.Println("rpc encode fail, gob error encoding header:", err)
		return err
	}
	if err := g.enc.Encode(body); err != nil {
		log.Println("rpc encode fail: gob error encoding body:", err)
		return err
	}
	return nil
}

func NewGobEncDec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	g := &GobEncDec{
		conn: conn,
		buf:  buf,
		enc:  gob.NewEncoder(buf),
		dec:  gob.NewDecoder(conn),
	}
	return g
}
