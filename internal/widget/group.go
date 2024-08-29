package widget

import (
	"context"
	"errors"
	"html/template"
	"sync"
	"time"

	"github.com/glanceapp/glance/internal/assets"
)

type Group struct {
	widgetBase `yaml:",inline"`
	Widgets    Widgets `yaml:"widgets"`
}

func (widget *Group) Initialize() error {
	widget.withError(nil)
	widget.HideHeader = true

	for i := range widget.Widgets {
		widget.Widgets[i].SetHideHeader(true)

		if widget.Widgets[i].GetType() == "group" {
			return errors.New("nested groups are not allowed")
		}

		if err := widget.Widgets[i].Initialize(); err != nil {
			return err
		}
	}

	return nil
}

func (widget *Group) Update(ctx context.Context) {
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

func (widget *Group) SetProviders(providers *Providers) {
	for i := range widget.Widgets {
		widget.Widgets[i].SetProviders(providers)
	}
}

func (widget *Group) RequiresUpdate(now *time.Time) bool {
	for i := range widget.Widgets {
		if widget.Widgets[i].RequiresUpdate(now) {
			return true
		}
	}

	return false
}

func (widget *Group) Render() template.HTML {
	return widget.render(widget, assets.GroupTemplate)
}
