package glance

import (
	"errors"
	"testing"
)

func TestStartServerAndReportReturnsStartupError(t *testing.T) {
	startErr := errors.New("address already in use")
	exitChannel := make(chan error, 1)

	startServerAndReport(func() error {
		return startErr
	}, exitChannel)

	err := <-exitChannel
	if !errors.Is(err, startErr) {
		t.Fatalf("expected startup error to be reported, got %v", err)
	}
}
