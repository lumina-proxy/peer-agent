package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	Version    = 1
	MaxLineLen = 1024
)

type Registration struct {
	Token    string `json:"token"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

type RegistrationAck struct {
	OK      bool   `json:"ok"`
	Country string `json:"country,omitempty"`
	Message string `json:"message,omitempty"`
}

func WriteJSONLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b)+1 > MaxLineLen {
		return fmt.Errorf("protocol: line too long (%d)", len(b))
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func ReadJSONLine(r *bufio.Reader, v any) error {
	line, err := readLimitedLine(r)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(line), v)
}

func WriteLine(w io.Writer, s string) error {
	if len(s)+1 > MaxLineLen {
		return fmt.Errorf("protocol: line too long (%d)", len(s))
	}
	_, err := io.WriteString(w, s+"\n")
	return err
}

func ReadLine(r *bufio.Reader) (string, error) {
	return readLimitedLine(r)
}

func readLimitedLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) > MaxLineLen {
		return "", fmt.Errorf("protocol: line exceeds %d bytes", MaxLineLen)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
