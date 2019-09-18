package desim

import (
	"bytes"
	"io"
	"strconv"
	"sync"
)

const mutelog = mutelogger(0)

func LogMute() Logger { return mutelog }

func LogJSON(w io.Writer) Logger {
	var mu sync.Mutex
	bufpool := sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 1<<10))
		},
	}

	return &kvlogger{
		encoder: func(keys, values []string) {
			buf := bufpool.Get().(*bytes.Buffer)
			buf.Reset()
			buf.WriteRune('{')
			for i, k := range keys {
				if i != 0 {
					buf.WriteRune(',')
				}
				v := values[i]
				buf.WriteString(strconv.Quote(k))
				buf.WriteRune(':')
				buf.WriteString(strconv.Quote(v))
			}
			buf.WriteString("}\n")
			mu.Lock()
			io.Copy(w, buf)
			mu.Unlock()
			bufpool.Put(buf)
		},
	}
}

func LogPretty(w io.Writer) Logger {
	var mu sync.Mutex
	bufpool := sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 1<<10))
		},
	}
	return &kvlogger{
		encoder: func(keys, values []string) {
			buf := bufpool.Get().(*bytes.Buffer)
			buf.Reset()
			for i, k := range keys {
				if i != 0 {
					buf.WriteString("\t")
				}
				v := values[i]
				buf.WriteString(k)
				buf.WriteRune('=')
				buf.WriteString(strconv.Quote(v))
			}
			buf.WriteRune('\n')

			mu.Lock()
			io.Copy(w, buf)
			mu.Unlock()
			bufpool.Put(buf)
		},
	}
}

type kvlogger struct {
	encoder func(keys, values []string)
	keys    []string
	values  []string
}

func (log *kvlogger) KV(k, v string) Logger {
	return &kvlogger{
		encoder: log.encoder,
		keys:    append(log.keys, k),
		values:  append(log.values, v),
	}
}

func (log kvlogger) Event(msg string) {
	log.encoder(
		append(log.keys, "event"),
		append(log.values, msg),
	)
}

type mutelogger uint8

func (l mutelogger) KV(_, _ string) Logger { return l }
func (mutelogger) Event(_ string)          {}
