package session

import (
	"bytes"
	"context"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	oscStart           = "\x1b]"
	oscTerminatorBEL   = byte('\a')
	oscTerminatorSTEsc = byte('\x1b')
	oscTerminatorSTChr = byte('\\')
	cwdOSCMarker       = "\x1b]9;cwd="
)

func splitUTF8SafeChunk(pending []byte, chunk []byte) (complete []byte, rest []byte) {
	data := append(pending, chunk...)
	if len(data) == 0 {
		return nil, nil
	}

	end := len(data)
	for back := 0; back < utf8.UTFMax && end-back > 0; back++ {
		start := end - back - 1
		if start < 0 {
			break
		}
		if utf8.RuneStart(data[start]) {
			if utf8.FullRune(data[start:end]) {
				return data[:end], nil
			}
			return data[:start], append([]byte(nil), data[start:end]...)
		}
	}

	if utf8.Valid(data) {
		return data, nil
	}

	return data, nil
}

func appendTerminalOutput(buffer []byte, chunk []byte) []byte {
	buffer = append(buffer, chunk...)
	if len(buffer) <= sshOutputBufferLimit {
		return buffer
	}

	start := len(buffer) - sshOutputBufferLimit
	if newline := bytes.IndexByte(buffer[start:], '\n'); newline >= 0 {
		start += newline + 1
	} else {
		for start < len(buffer) && !utf8.RuneStart(buffer[start]) {
			start++
		}
	}
	return append([]byte(nil), buffer[start:]...)
}

func stripTerminalSignals(pending []byte, chunk []byte) ([]byte, []byte, []string) {
	data := append(append([]byte(nil), pending...), chunk...)
	if len(data) == 0 {
		return nil, nil, nil
	}

	visible := make([]byte, 0, len(data))
	cwds := make([]string, 0, 1)
	index := 0
	for index < len(data) {
		start := bytes.Index(data[index:], []byte(oscStart))
		if start < 0 {
			visible = append(visible, data[index:]...)
			break
		}
		start += index
		visible = append(visible, data[index:start]...)

		end, terminatorSize := findOSCTerminator(data, start+len(oscStart))
		if end < 0 {
			return visible, append([]byte(nil), data[start:]...), cwds
		}

		if bytes.HasPrefix(data[start:], []byte(cwdOSCMarker)) {
			payloadStart := start + len(cwdOSCMarker)
			cwds = append(cwds, string(data[payloadStart:end]))
		}
		index = end + terminatorSize
	}

	if suffixLength := longestOSCPrefixSuffix(data); suffixLength > 0 && suffixLength <= len(visible) {
		visible = visible[:len(visible)-suffixLength]
		return visible, append([]byte(nil), data[len(data)-suffixLength:]...), cwds
	}

	return visible, nil, cwds
}

func findOSCTerminator(data []byte, start int) (end int, terminatorSize int) {
	for i := start; i < len(data); i++ {
		switch data[i] {
		case oscTerminatorBEL:
			return i, 1
		case oscTerminatorSTEsc:
			if i+1 < len(data) && data[i+1] == oscTerminatorSTChr {
				return i, 2
			}
		}
	}
	return -1, 0
}

func longestOSCPrefixSuffix(data []byte) int {
	marker := []byte(oscStart)
	max := len(marker) - 1
	if len(data) < max {
		max = len(data)
	}
	for length := max; length > 0; length-- {
		if bytes.Equal(data[len(data)-length:], marker[:length]) {
			return length
		}
	}
	return 0
}

func emitSessionEvent(ctx context.Context, eventName string, optionalData ...interface{}) {
	if ctx == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	runtime.EventsEmit(ctx, eventName, optionalData...)
}
