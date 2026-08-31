package api

import (
	"net/http"
	"regexp"
	"strings"
)

var validServiceIDRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

var validLogStreams = map[string]bool{
	"stdout":   true,
	"stderr":   true,
	"combined": true,
}

// validateServiceID checks that the service ID matches the allowed pattern.
// Returns false and writes 400 if invalid.
func validateServiceID(w http.ResponseWriter, id string) bool {
	if !validServiceIDRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, "INVALID_SERVICE_ID", "Invalid service ID")
		return false
	}
	return true
}

// validateLogStream checks that the stream name is in the allowed set.
// Returns false and writes 400 if invalid.
func validateLogStream(w http.ResponseWriter, stream string) bool {
	if !validLogStreams[stream] {
		writeError(w, http.StatusBadRequest, "INVALID_STREAM", "Invalid log stream")
		return false
	}
	return true
}

// requestBodyLimit returns middleware that limits the request body size.
func requestBodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// containsShellMeta reports whether s contains characters that cmd.exe treats
// as syntax rather than as part of a command name.
//
// This is not a security boundary and must not be relied on as one. A health
// check's script_body is executed verbatim by design, so anything this rejects
// in target can be expressed there instead. What it does is catch a target
// that was written as if it were a shell one-liner, and point the author at
// the field meant for that, before the check starts failing in production for
// reasons the log will not explain.
//
// Anyone tempted to treat it as a filter should note the shape of the problem:
// the set below is a blocklist, and cmd.exe has more syntax than any such list
// covers. Restricting what may be executed belongs in the service config, not
// here.
func containsShellMeta(s string) bool {
	return strings.ContainsAny(s, "&|;><\\$()^`%\"'\n\r")
}
