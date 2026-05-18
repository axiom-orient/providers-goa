package client

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

type sseMessage struct {
	Event string
	Data  []byte
}

type sseDecoder struct {
	r *bufio.Reader
}

func newSSEDecoder(r io.Reader) *sseDecoder {
	return &sseDecoder{r: bufio.NewReader(r)}
}

func (d *sseDecoder) Next() (sseMessage, error) {
	var msg sseMessage
	var dataLines []string
	for {
		line, err := d.r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return sseMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) == 0 && msg.Event == "" {
				if errors.Is(err, io.EOF) {
					return sseMessage{}, io.EOF
				}
				continue
			}
			msg.Data = []byte(strings.Join(dataLines, "\n"))
			return msg, nil
		}
		if strings.HasPrefix(line, ":") {
			if errors.Is(err, io.EOF) {
				if len(dataLines) == 0 && msg.Event == "" {
					return sseMessage{}, io.EOF
				}
				msg.Data = []byte(strings.Join(dataLines, "\n"))
				return msg, nil
			}
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			field = line
			value = ""
		}
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}
		switch field {
		case "event":
			msg.Event = value
		case "data":
			dataLines = append(dataLines, value)
		}
		if errors.Is(err, io.EOF) {
			if len(dataLines) == 0 && msg.Event == "" {
				return sseMessage{}, io.EOF
			}
			msg.Data = []byte(strings.Join(dataLines, "\n"))
			return msg, nil
		}
	}
}
