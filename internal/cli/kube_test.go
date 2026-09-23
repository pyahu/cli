package cli

import (
	"strings"
	"testing"

	"github.com/pyahu/cli/internal/cloud"
)

func env(name, org string) cloud.Environment {
	return cloud.Environment{Name: name, OrganizationName: org, OrganizationID: org, TenantID: name}
}

// Asking somebody to name the only option is ceremony.
func TestPicksTheOnlyEnvironmentWithoutBeingTold(t *testing.T) {
	got, err := pickEnvironment([]cloud.Environment{env("production", "acme")}, "")

	if err != nil {
		t.Fatalf("expected a pick, got %v", err)
	}
	if got.Name != "production" {
		t.Fatalf("unexpected pick %q", got.Name)
	}
}

// Guessing between two environments is the kind of convenience that eventually points kubectl at the
// wrong one.
func TestRefusesToGuessBetweenEnvironments(t *testing.T) {
	_, err := pickEnvironment([]cloud.Environment{env("production", "acme"), env("staging", "acme")}, "")

	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "production") || !strings.Contains(err.Error(), "staging") {
		t.Fatalf("the refusal must list what to choose from, got %q", err)
	}
}

// Two organizations can each have an environment called "production". Picking one would be picking
// somebody's production at random.
func TestRefusesAnAmbiguousNameAcrossOrganizations(t *testing.T) {
	_, err := pickEnvironment(
		[]cloud.Environment{env("production", "acme"), env("production", "globex")}, "production")

	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "more than one organization") {
		t.Fatalf("the refusal must say why, got %q", err)
	}
}

func TestMatchesAnEnvironmentByNameIgnoringCase(t *testing.T) {
	got, err := pickEnvironment([]cloud.Environment{env("production", "acme"), env("staging", "acme")}, "PRODUCTION")

	if err != nil {
		t.Fatalf("expected a pick, got %v", err)
	}
	if got.Name != "production" {
		t.Fatalf("unexpected pick %q", got.Name)
	}
}

// The empty case is the ordinary one before an admin turns the feature on, so it has to say what to do
// rather than read as a failure of the CLI.
func TestSaysWhoTurnsAccessOnWhenThereIsNothing(t *testing.T) {
	_, err := pickEnvironment(nil, "")

	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "console") {
		t.Fatalf("the message must point at the console, got %q", err)
	}
}
