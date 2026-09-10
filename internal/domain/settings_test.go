package domain_test

import (
	"testing"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestSettings_MaxAttemptsDefaultsToThreeTotalAttempts(t *testing.T) {
	if got := (domain.Settings{}).MaxAttempts(); got != 3 {
		t.Fatalf("MaxAttempts()=%d want 3", got)
	}
}

func TestSettings_MaxAttemptsUsesConfiguredMaxRetries(t *testing.T) {
	settings := domain.Settings{MaxRetries: 5}
	if got := settings.MaxAttempts(); got != 5 {
		t.Fatalf("MaxAttempts()=%d want 5", got)
	}
}
