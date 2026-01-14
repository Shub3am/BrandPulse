// nonetwork.go is the no-network guarantee, as a package rather than a CI
// setting.
//
// Python had pytest-socket and Go has nothing equivalent, so the ban is
// enforced by construction: HTTPClient takes an *http.Client, and every
// track's tests and the CI job inject this one. A test that reaches the
// network fails on the transport instead of quietly spending a credit.
//
// It lives here, exported, so there is one implementation rather than the same
// eight lines copied into six worktrees.
package anakin

import (
	"fmt"
	"net/http"
)

// NoNetwork returns an *http.Client whose Transport fails every round trip.
func NoNetwork() *http.Client {
	return &http.Client{Transport: noNetworkTransport{}}
}

type noNetworkTransport struct{}

// RoundTrip names the URL that was attempted, because the useful half of this
// failure is which call escaped the fixtures.
func (noNetworkTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("anakin: this client has no network, and something tried %s %s; tests run on fixtures",
		req.Method, req.URL)
}
