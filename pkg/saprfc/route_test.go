package saprfc

import "testing"

func TestRedactRouteHidesEveryPasswordHop(t *testing.T) {
	for in, want := range map[string]string{
		"/H/router.example/S/3299/W/s3cret/H/": "/H/router.example/S/3299/W/***/H/",
		"/H/a/w/one/H/b/W/two/H/":              "/H/a/w/***/H/b/W/***/H/",
		"/H/router.example/S/3299":             "/H/router.example/S/3299",
		"/H/a/W/":                              "/H/a/W/",
	} {
		if got := RedactRoute(in); got != want {
			t.Errorf("RedactRoute(%q) = %q, want %q", in, got, want)
		}
	}
}
