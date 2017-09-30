package desim

import (
	"fmt"
	"io"
)

func LogJSON(w io.Writer) Logger {
	return &kvlogger{
		encoder: func(keys, values []string) {
			fmt.Fprint(w, "{")
			for i, k := range keys {
				if i != 0 {
					fmt.Fprint(w, ",")
				}
				v := values[i]
				fmt.Fprintf(w, "%q:%q", k, v)
			}
			fmt.Fprint(w, "}\n")
		},
	}
}

func LogPretty(w io.Writer) Logger {
	return &kvlogger{
		encoder: func(keys, values []string) {
			for i, k := range keys {
				if i != 0 {
					fmt.Fprint(w, "\t")
				}
				v := values[i]
				fmt.Fprintf(w, "%s=%q", k, v)
			}
			fmt.Fprint(w, "\n")
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
