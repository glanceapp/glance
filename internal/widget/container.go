package widget

import (
	"context"
	"sync"
	"time"
)

type containerWidget struct {
	Widgets Widgets `yaml:"widgets"`
}

func (widget *containerWidget) Update(ctx context.Context) {
	var wg sync.WaitGroup
	now := time.Now()

	for w := range widget.Widgets {
		widget := widget.Widgets[w]

		if !widget.RequiresUpdate(&now) {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			widget.Update(ctx)
		}()
	}

	wg.Wait()
}

func (widget *containerWidget) SetProviders(providers *Providers) {
	for i := range widget.Widgets {
		widget.Widgets[i].SetProviders(providers)
	}
}

func (widget *containerWidget) RequiresUpdate(now *time.Time) bool {
	for i := range widget.Widgets {
		if widget.Widgets[i].RequiresUpdate(now) {
			return true
		}
	}

	return false
}
