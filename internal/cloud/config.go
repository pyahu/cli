// Package cloud talks to Pyahu Cloud: it signs a person in and fetches the short-lived credential
// their `kubectl` uses to reach a cluster.
//
// Everything this package knows about Pyahu Cloud is public by construction. The console's address is
// the one people type into a browser; the issuer is an OIDC issuer, which publishes its own
// configuration at a well-known URL; and the client id belongs to a PUBLIC OAuth client, which by
// definition has no secret, exactly as the ids embedded in `gcloud`, `aws` and `gh` do. Nothing here
// grants anything: every call is authenticated by a token the person obtained by signing in, and every
// authorization decision is made on the server.
package cloud

import "os"

const (
	// DefaultConsoleURL is where the Pyahu Cloud console lives, and therefore where the two endpoints
	// this CLI uses are served. It is the address people already type.
	DefaultConsoleURL = "https://console.pyahu.cloud"

	// DefaultIssuer is the OIDC issuer people sign in against. An issuer is public by design: it
	// publishes its endpoints, its keys and its capabilities at a well-known URL for anybody to read.
	DefaultIssuer = "https://zitadel.de.pyahu.cloud"

	// DefaultClientID identifies this CLI to the issuer. It is a PUBLIC client: it holds no secret,
	// and it cannot, because a secret shipped in a binary on somebody's laptop is not a secret. What
	// protects the flow is that the person has to approve the sign-in themselves, on a page served by
	// the issuer, showing a code this CLI printed.
	DefaultClientID = "pyahu-cli"
)

// Config is where this CLI points. Every field has a public default and an environment override, so a
// development instance needs no code change and no release.
type Config struct {
	ConsoleURL string
	Issuer     string
	ClientID   string
}

// LoadConfig reads the configuration from the environment, falling back to Pyahu Cloud.
func LoadConfig() Config {
	return Config{
		ConsoleURL: envOr("PYAHU_CONSOLE_URL", DefaultConsoleURL),
		Issuer:     envOr("PYAHU_ISSUER", DefaultIssuer),
		ClientID:   envOr("PYAHU_CLIENT_ID", DefaultClientID),
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
