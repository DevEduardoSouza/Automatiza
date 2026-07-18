package whatsapp

import (
	"errors"
	"io"
	"net"
	"net/url"
	"testing"
)

// TestIsPreConnectionError guards the DNS-abort fix: a Docker embedded-resolver
// SERVFAIL ("lookup waha-router ...: server misbehaving") must be classified as
// pre-connection so doRequest retries it even for non-idempotent sends, while a
// post-POST reset/EOF must NOT be (it stays in the ambiguous assume-sent path).
func TestIsPreConnectionError(t *testing.T) {
	// Shaped exactly like what http.Client.Do returns for a dial-time DNS failure:
	// *url.Error -> *net.OpError -> *net.DNSError.
	dnsAbort := &url.Error{
		Op:  "Post",
		URL: "http://waha-router:3003/api/sendText",
		Err: &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &net.DNSError{Err: "server misbehaving", Name: "waha-router", IsTemporary: true},
		},
	}
	if !isPreConnectionError(dnsAbort) {
		t.Fatal("wrapped *net.DNSError must be pre-connection (retryable even for sends)")
	}

	// NXDOMAIN while the router container recreates — also pre-connection.
	nxdomain := &net.DNSError{Err: "no such host", Name: "waha-router", IsNotFound: true}
	if !isPreConnectionError(nxdomain) {
		t.Fatal("NXDOMAIN *net.DNSError must be pre-connection")
	}

	// Post-connection failures must NOT be caught here — retrying a send after
	// these risks a duplicate, so they belong to the ambiguous path.
	if isPreConnectionError(io.EOF) {
		t.Fatal("io.EOF is a post-POST ambiguous error, not pre-connection")
	}
	if isPreConnectionError(errors.New("connection reset by peer")) {
		t.Fatal("connection reset is post-POST ambiguous, not pre-connection")
	}
	if isPreConnectionError(nil) {
		t.Fatal("nil error is not pre-connection")
	}
}
