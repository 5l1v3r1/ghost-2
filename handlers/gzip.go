package handlers

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// Thanks to Andrew Gerrand for inspiration:
// https://groups.google.com/d/msg/golang-nuts/eVnTcMwNVjM/4vYU8id9Q2UJ
//
// Also, node's Connect library implementation of the compress middleware:
// https://github.com/senchalabs/connect/blob/master/lib/middleware/compress.js
//
// And StackOverflow's explanation of Vary: Accept-Encoding header:
// http://stackoverflow.com/questions/7848796/what-does-varyaccept-encoding-mean

// Internal gzipped writer that satisfies both the (body) writer in gzipped format,
// and maintains the rest of the ResponseWriter interface for header manipulation.
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

// Unambiguous Write() implementation (otherwise both ResponseWriter and Writer
// want to claim this method).
func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Gzip compression HTTP handler.
func GZIPHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if _, ok := w.(*gzipResponseWriter); ok {
				// Self-awareness, the ResponseWriter is already a gzip writer, ignore
				h.ServeHTTP(w, r)
				return
			}
			hdr := w.Header()
			setVaryHeader(hdr)

			// Do nothing on a HEAD request or if no accept-encoding is specified on the request
			acc, ok := r.Header["Accept-Encoding"]
			if r.Method == "HEAD" || !ok {
				h.ServeHTTP(w, r)
				return
			}
			if !acceptsGzip(acc) {
				// No gzip support from the client, return uncompressed
				h.ServeHTTP(w, r)
				return
			}

			// Prepare a gzip response container
			setGzipHeaders(hdr)
			gz := gzip.NewWriter(w)
			defer gz.Close()
			h.ServeHTTP(
				&gzipResponseWriter{
					Writer:         gz,
					ResponseWriter: w,
				}, r)
		})
}

func setVaryHeader(hdr http.Header) {
	// Manage the Vary header field
	vary := hdr["Vary"]
	ok := false
	for _, v := range vary {
		if strings.ToLower(v) == "accept-encoding" {
			ok = true
		}
	}
	if !ok {
		hdr.Add("Vary", "Accept-Encoding")
	}
}

func acceptsGzip(acc []string) bool {
	for _, v := range acc {
		trimmed := strings.ToLower(strings.Trim(v, " "))
		if trimmed == "*" || strings.Contains(trimmed, "gzip") {
			return true
		}
	}
	return false
}

func setGzipHeaders(hdr http.Header) {
	// The content-type will be explicitly set somewhere down the path of handlers
	hdr.Set("Content-Encoding", "gzip")
	hdr.Del("Content-Length")
}
