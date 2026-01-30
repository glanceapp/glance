package glance

import (
	"context"
	"sync"
	"time"
)

type containerWidgetBase struct {
	Widgets widgets `yaml:"widgets"`
}

func (widget *containerWidgetBase) _initializeWidgets() error {
	for i := range widget.Widgets {
		if err := widget.Widgets[i].initialize(); err != nil {
			return formatWidgetInitError(err, widget.Widgets[i])
		}
	}

	return nil
}

func (widget *containerWidgetBase) _update(ctx context.Context) {
	var wg sync.WaitGroup
	now := time.Now()

	for i := range widget.Widgets {
		w := widget.Widgets[i]

		if !w.requiresUpdate(&now) {
			continue
		}

		wg.Add(1)
		go func(item interface {
			update(context.Context)
			requiresUpdate(*time.Time) bool
		}) {
			defer wg.Done()
			item.update(ctx)
		}(w)
	}

	wg.Wait()
}

func (widget *containerWidgetBase) _setProviders(providers *widgetProviders) {
	for i := range widget.Widgets {
		widget.Widgets[i].setProviders(providers)
	}
}

func (widget *containerWidgetBase) _requiresUpdate(now *time.Time) bool {
	for i := range widget.Widgets {
		if widget.Widgets[i].requiresUpdate(now) {
			return true
		}
	}

	return false
}
