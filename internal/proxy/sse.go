package proxy

import "net/http"

// flushWriter wraps http.ResponseWriter and exposes a Flush method that is a no-op
// when the underlying writer does not implement http.Flusher.
type flushWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newFlushWriter(w http.ResponseWriter) *flushWriter {
	fw := &flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.flusher = f
	}
	return fw
}

func (fw *flushWriter) Header() http.Header        { return fw.w.Header() }
func (fw *flushWriter) WriteHeader(code int)        { fw.w.WriteHeader(code) }
func (fw *flushWriter) Write(p []byte) (int, error) { return fw.w.Write(p) }
func (fw *flushWriter) Flush() {
	if fw.flusher != nil {
		fw.flusher.Flush()
	}
}
