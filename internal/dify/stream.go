package dify

import (
	"bufio"
	"encoding/json"
	"strings"
)

// ReadStream reads SSE lines from a scanner and sends StreamEvents to the returned channel.
// The channel is closed when the stream ends or an error occurs.
func ReadStream(scanner *bufio.Scanner) <-chan StreamEvent {
	ch := make(chan StreamEvent, 16)
	go func() {
		defer close(ch)
		var dataLine string
		for scanner.Scan() {
			line := scanner.Text()
			if rest, ok := strings.CutPrefix(line, "data:"); ok {
				dataLine = strings.TrimSpace(rest)
			} else if line == "" && dataLine != "" {
				// End of one SSE event block
				if dataLine == "[DONE]" {
					dataLine = ""
					continue
				}
				var ev StreamEvent
				if err := json.Unmarshal([]byte(dataLine), &ev); err != nil {
					ch <- StreamEvent{Err: err}
					dataLine = ""
					return
				}
				ch <- ev
				dataLine = ""
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Err: err}
		}
	}()
	return ch
}
