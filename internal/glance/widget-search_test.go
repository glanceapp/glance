package glance

import (
	"strings"
	"testing"
)

func TestSearchWidgetRendersUseLayout(t *testing.T) {
	widget := &searchWidget{UseLayout: true}
	if err := widget.initialize(); err != nil {
		t.Fatalf("initialize search widget: %v", err)
	}

	if !strings.Contains(string(widget.Render()), `data-use-layout="true"`) {
		t.Fatal("expected rendered search widget to enable layout-aware shortcut handling")
	}
}
