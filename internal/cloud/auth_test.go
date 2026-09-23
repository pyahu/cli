package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The device approval URL the issuer hands back is not always one that can finish the sign-in.
//
// A Zitadel instance that requires its v2 login everywhere still hands out the v1 device path. On such
// an instance the v1 page accepts the password, re-renders itself, and never approves the device: the
// screen blinks and stays put while the CLI polls authorization_pending until the code expires, with no
// error logged on either side. Preferring the v2 path when it exists is what makes `pyahu login` finish.
func TestPreferWorkingLoginUI(t *testing.T) {
	t.Run("prefers the v2 device page when the issuer serves one", func(t *testing.T) {
		var asked []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			asked = append(asked, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		given := srv.URL + "/ui/login/device?user_code=ABCD-EFGH"
		got := preferWorkingLoginUI(context.Background(), srv.Client(), given)

		want := srv.URL + "/ui/v2/login/device?user_code=ABCD-EFGH"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if len(asked) != 1 || asked[0] != "/ui/v2/login/device" {
			t.Fatalf("expected exactly one probe of the v2 path, got %v", asked)
		}
	})

	t.Run("keeps the issuer's URL when there is no v2 page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/ui/v2/") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		given := srv.URL + "/ui/login/device?user_code=ABCD-EFGH"
		if got := preferWorkingLoginUI(context.Background(), srv.Client(), given); got != given {
			t.Fatalf("an issuer that serves only v1 must be left alone; got %q", got)
		}
	})

	t.Run("keeps a URL that is not Zitadel's v1 device path", func(t *testing.T) {
		// This CLI also signs in against issuers that are not Zitadel. Nothing may be rewritten there,
		// and no probe may even be attempted.
		var probed bool
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			probed = true
		}))
		defer srv.Close()

		for _, given := range []string{
			srv.URL + "/activate?code=ABCD",
			srv.URL + "/ui/v2/login/device?user_code=ABCD",
			"",
		} {
			if got := preferWorkingLoginUI(context.Background(), srv.Client(), given); got != given {
				t.Errorf("%q must be left alone, got %q", given, got)
			}
		}
		if probed {
			t.Error("no probe should be made for a URL that is not the v1 device path")
		}
	})

	t.Run("keeps the issuer's URL when the probe cannot be made", func(t *testing.T) {
		// A sign-in that might work beats one that certainly cannot, so an unreachable probe must not
		// replace the only URL the person has.
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		addr := srv.URL
		srv.Close() // nothing is listening now

		given := addr + "/ui/login/device?user_code=ABCD-EFGH"
		if got := preferWorkingLoginUI(context.Background(), http.DefaultClient, given); got != given {
			t.Fatalf("got %q, want the original %q", got, given)
		}
	})
}
