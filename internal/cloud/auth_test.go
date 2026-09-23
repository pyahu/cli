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
// A Zitadel instance that requires its v2 login still redirects a device flow into the v1 UI. There the
// password is accepted, the page re-renders, and the device is never approved: the screen blinks and
// stays put while the CLI polls authorization_pending until the code expires, with no error logged on
// either side.
//
// These tests model what the issuer ACTUALLY returns: its OIDC endpoint (…/device?user_code=X), which
// then answers with a Location into one login UI or the other. An earlier version of this code matched
// on the login path being present in the returned URL, which it never is, so the fix shipped inert. The
// fixtures below redirect, because that is the only way the bug is visible.
func TestPreferWorkingLoginUI(t *testing.T) {
	// deviceEndpoint stands in for the issuer: /device answers with a redirect into `into`, and every
	// login path answers 200 unless it is absent from `serves`.
	deviceEndpoint := func(into string, serves ...string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/device" {
				http.Redirect(w, r, into+"?user_code="+r.URL.Query().Get("user_code"), http.StatusFound)
				return
			}
			for _, ok := range serves {
				if strings.HasPrefix(r.URL.Path, ok) {
					w.WriteHeader(http.StatusOK)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		}))
	}

	t.Run("prefers v2 when the issuer redirects a device flow into v1", func(t *testing.T) {
		srv := deviceEndpoint("/ui/login/device", "/ui/login/", "/ui/v2/login/")
		defer srv.Close()

		given := srv.URL + "/device?user_code=ABCD-EFGH"
		got := preferWorkingLoginUI(context.Background(), srv.Client(), given)

		want := srv.URL + "/ui/v2/login/device?user_code=ABCD-EFGH"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("leaves it alone when the issuer already sends to v2", func(t *testing.T) {
		srv := deviceEndpoint("/ui/v2/login/device", "/ui/v2/login/")
		defer srv.Close()

		given := srv.URL + "/device?user_code=ABCD-EFGH"
		if got := preferWorkingLoginUI(context.Background(), srv.Client(), given); got != given {
			t.Fatalf("an instance already on v2 must be left alone; got %q", got)
		}
	})

	t.Run("leaves it alone when there is no v2 page to go to", func(t *testing.T) {
		// An issuer that serves only the v1 UI: rewriting would send the person to a 404, which is
		// strictly worse than a page that might work.
		srv := deviceEndpoint("/ui/login/device", "/ui/login/")
		defer srv.Close()

		given := srv.URL + "/device?user_code=ABCD-EFGH"
		if got := preferWorkingLoginUI(context.Background(), srv.Client(), given); got != given {
			t.Fatalf("got %q, want the original %q", got, given)
		}
	})

	t.Run("leaves alone an issuer that does not redirect at all", func(t *testing.T) {
		// This CLI also signs in against issuers that are not Zitadel.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		given := srv.URL + "/activate?code=ABCD"
		if got := preferWorkingLoginUI(context.Background(), srv.Client(), given); got != given {
			t.Fatalf("got %q, want the original %q", got, given)
		}
	})

	t.Run("leaves it alone when the issuer cannot be reached", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		addr := srv.URL
		srv.Close() // nothing is listening now

		given := addr + "/device?user_code=ABCD-EFGH"
		if got := preferWorkingLoginUI(context.Background(), http.DefaultClient, given); got != given {
			t.Fatalf("got %q, want the original %q", got, given)
		}
	})

	t.Run("empty stays empty", func(t *testing.T) {
		if got := preferWorkingLoginUI(context.Background(), http.DefaultClient, ""); got != "" {
			t.Fatalf("got %q", got)
		}
	})
}
