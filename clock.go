package lti

import "time"

// Clock abstracts time so launch/state/nonce expiry logic can be tested
// deterministically. See the ltitest package for a manually-advanceable
// implementation.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
