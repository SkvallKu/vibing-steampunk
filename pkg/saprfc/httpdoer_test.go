package saprfc

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/oisee/open-rfc-go/rfc"
)

func TestAfterTunnelError(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		drop, retry bool
	}{
		{"transport", fmt.Errorf("x: %w", rfc.ErrTransport), true, false},
		{"closed", rfc.ErrClosed, true, false},
		// The dump seen on 7.50 after some thirty-six data preview queries
		// in one session: it carries the text, and on other systems the ID.
		{"subpool text", &rfc.ABAPException{Kind: rfc.KindRuntime, PlainText: "No further temporary subroutine pools can be generated."}, true, true},
		{"subpool id", &rfc.ABAPException{Kind: rfc.KindRuntime, RuntimeID: "GENERATE_SUBPOOL_DIR_FULL"}, true, true},
		// Any other dump may have left work half done: the session goes,
		// the request is not repeated.
		{"other dump", &rfc.ABAPException{Kind: rfc.KindRuntime, RuntimeID: "DBSQL_SQL_ERROR"}, true, false},
		{"declared exception", &rfc.ABAPException{Kind: rfc.KindException, Key: "NOT_FOUND"}, false, false},
		{"other", errors.New("boom"), false, false},
	}
	for _, c := range cases {
		drop, retry := afterTunnelError(fmt.Errorf("call: %w", c.err))
		if drop != c.drop || retry != c.retry {
			t.Errorf("%s: got drop=%v retry=%v, want drop=%v retry=%v", c.name, drop, retry, c.drop, c.retry)
		}
	}
}

// The tunnel gets the path as pkg/adt escaped it: an object in a namespace is
// /sap/bc/adt/oo/classes/%2fsdf%2fcl_x, and with the slashes decoded ADT
// answers 404.
func TestTunnelURIKeepsTheEscapedPath(t *testing.T) {
	cases := []struct{ url, want string }{
		{"http://h/sap/bc/adt/oo/classes/%2fsdf%2fcl_cm_chk_adt_res_app/source/main",
			"/sap/bc/adt/oo/classes/%2fsdf%2fcl_cm_chk_adt_res_app/source/main"},
		{"http://h/sap/bc/adt/oo/classes/%2FSDF%2FCL_X?version=active",
			"/sap/bc/adt/oo/classes/%2FSDF%2FCL_X?version=active"},
		{"http://h/sap/bc/adt/programs/programs/zprog", "/sap/bc/adt/programs/programs/zprog"},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", c.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := tunnelURI(req.URL); got != c.want {
			t.Errorf("tunnelURI(%s) = %s, want %s", c.url, got, c.want)
		}
	}
}
