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
//
// STUB: signature only, body panics. B1 Task 12 implements this.
package anakin

import "net/http"

// NoNetwork returns an *http.Client whose Transport fails every round trip.
func NoNetwork() *http.Client {
	panic("not implemented")
}
