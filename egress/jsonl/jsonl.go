package jsonl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mehrdadrad/tcpdog/config"
)

type jsonl struct {
	fieldsLen  []int
	fieldsName []string
	file       io.WriteCloser
	buffer     *bytes.Buffer
}

var comma = []byte(",")[0]

func (j *jsonl) init(conf map[string]interface{}, fields []config.Field) error {
	var err error

	for _, f := range fields {
		j.fieldsLen = append(j.fieldsLen, len(f.Name)+3)
		j.fieldsName = append(j.fieldsName, f.Name)
	}

	filename, ok := conf["filename"].(string)
	if !ok {
		return fmt.Errorf("file has not been configured")
	}

	j.file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	return err
}

func (j *jsonl) marshal(buf *bytes.Buffer) {
	buf.Next(1) // skip bracket

	j.buffer.WriteRune('[')
	for _, l := range j.fieldsLen {
		buf.Next(l)
		v, _ := buf.ReadBytes(comma)
		j.buffer.Write(v)
	}

	buf.Next(12)                 // skip timestamp key
	j.buffer.Write(buf.Next(10)) // write timestamp

	j.buffer.WriteRune(']')
}

func (j *jsonl) header() {
	m := strings.Join(j.fieldsName, ",")
	j.buffer.WriteString(fmt.Sprintf("[%s,timestamp]", m))
}
func (j *jsonl) flush() {
	j.buffer.WriteRune('\n')
	j.file.Write(j.buffer.Bytes())
	j.buffer.Reset()
}

func (j *jsonl) cleanup() {
	j.file.Close()
}

// Start encodes and writes tcp fields to a specific file in jsonl format
func Start(ctx context.Context, tp config.Tracepoint, bufpool *sync.Pool, ch chan *bytes.Buffer) error {
	var (
		j   = &jsonl{buffer: new(bytes.Buffer)}
		err error
	)

	cfg := config.FromContext(ctx)
	err = j.init(cfg.Egress[tp.Egress].Config, cfg.Fields[tp.Fields])
	if err != nil {
		return err
	}

	j.header()
	j.flush()

	go func() {
		defer j.cleanup()
		var buf *bytes.Buffer

		for {
			select {
			case buf = <-ch:
			case <-ctx.Done():
				return
			}

			j.marshal(buf)
			j.flush()

			bufpool.Put(buf)
		}
	}()

	return nil
}
