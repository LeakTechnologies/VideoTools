//go:build !native_media

package ui

import (
	"errors"
	"testing"
)

func TestAutomaticPreviewStubPolicyParity(t *testing.T) {
	p := NewInlineVideoPlayer()
	if err := p.LoadAutomatic("default"); err != nil {
		t.Fatal(err)
	}
	<-p.SetAutomaticPreviewsEnabled(false)
	if err := p.LoadAutomatic("disabled"); !errors.Is(err, ErrAutomaticPreviewsDisabled) {
		t.Fatalf("disabled stub admission: %v", err)
	}
	if err := p.Load("manual"); err != nil {
		t.Fatal(err)
	}
	<-p.SetAutomaticPreviewsEnabled(true)
	if err := p.LoadAutomatic("enabled"); err != nil {
		t.Fatal(err)
	}
}
