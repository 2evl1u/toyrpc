package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

type JSONEncDec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	enc  *json.Encoder
	dec  *json.Decoder
}

var _ Codec = (*JSONEncDec)(nil)

func (j *JSONEncDec) Close() error {
	return j.conn.Close()
}

func (j *JSONEncDec) ReadHeader(header *Header) error {
	return j.dec.Decode(header)
}

func (j *JSONEncDec) ReadBody(body any) error {
	return j.dec.Decode(body)
}

func (j *JSONEncDec) Write(h *Header, body any) (err error) {
	defer func() {
		_ = j.buf.Flush()
		if err != nil {
			_ = j.Close()
		}
	}()
	if err := j.enc.Encode(h); err != nil {
		log.Println("rpc encode fail: json error encoding header:", err)
		return err
	}
	if err := j.enc.Encode(body); err != nil {
		log.Println("rpc encode fail: json error encoding body:", err)
		return err
	}
	return nil
}

func NewJSONEncDec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	j := &JSONEncDec{
		conn: conn,
		buf:  buf,
		enc:  json.NewEncoder(buf),
		dec:  json.NewDecoder(conn),
	}
	return j
}
