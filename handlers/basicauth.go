package handlers

// Inspired by node.js' Connect library implementation of the basicAuth middleware.
// https://github.com/senchalabs/connect

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// Internal writer that keeps track of the currently authenticated user.
type userResponseWriter struct {
	http.ResponseWriter
	user interface{}
}

// Implement the WrapWriter interface.
func (this *userResponseWriter) WrappedWriter() http.ResponseWriter {
	return this.ResponseWriter
}

// Writes an unauthorized response to the client, specifying the expected authentication
// information.
func Unauthorized(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("Unauthorized"))
}

// Writes a bad request response to the client, with an optional message.
func BadRequest(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	if msg == "" {
		msg = "Bad Request"
	}
	w.Write([]byte(msg))
}

// Returns a Basic Authentication handler, protecting the wrapped handler from
// being accessed if the authentication function is not successful.
func BasicAuthHandler(h http.Handler,
	authFn func(string, string) (interface{}, bool), realm string) http.Handler {

	if realm == "" {
		realm = "Authorization Required"
	}
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Self-awareness
			if _, ok := GetUser(w); ok {
				h.ServeHTTP(w, r)
				return
			}
			authInfo := r.Header.Get("Authorization")
			if authInfo == "" {
				// No authorization info, return 401
				Unauthorized(w, realm)
				return
			}
			parts := strings.Split(authInfo, " ")
			if len(parts) != 2 {
				BadRequest(w, "Bad authorization header")
				return
			}
			scheme := parts[0]
			creds, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				BadRequest(w, "Bad credentials encoding")
			}
			index := bytes.Index(creds, []byte(":"))
			if scheme != "Basic" || index < 0 {
				BadRequest(w, "Bad authorization header")
			}
			user, pwd := string(creds[:index]), string(creds[index+1:])
			udata, ok := authFn(user, pwd)
			if ok {
				// Save user data and continue
				uw := &userResponseWriter{w, udata}
				h.ServeHTTP(uw, r)
			} else {
				Unauthorized(w, realm)
			}
		})
}

// Return the currently authenticated user. This is the same data that was returned
// by the authentication function passed to BasicAuthHandler.
func GetUser(w http.ResponseWriter) (interface{}, bool) {
	usr, ok := GetResponseWriter(w, func(tst http.ResponseWriter) bool {
		_, ok := tst.(*userResponseWriter)
		return ok
	})
	if ok {
		return usr.(*userResponseWriter).user, true
	}
	return nil, false
}
