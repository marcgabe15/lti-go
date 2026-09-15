package memstore_test

import (
	"testing"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/memstore"
	"github.com/marcgabe15/lti-go/storetest"
)

func TestStore(t *testing.T) {
	storetest.Run(t, func() lti.Store { return memstore.New() })
}
