package glance

import (
	"context"
	"html/template"
	"sync"
	"time"
)

const liveUpdateTickInterval = 10 * time.Second

func (a *application) startLiveUpdateTicker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(liveUpdateTickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.tickAllPages(ctx)
			}
		}
	}()
}

func (a *application) tickAllPages(ctx context.Context) {
	seen := make(map[*page]struct{})
	for _, p := range a.slugToPage {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		a.tickPage(ctx, p)
	}
}

func (a *application) tickPage(ctx context.Context, p *page) {
	now := time.Now()

	type candidate struct {
		wgt      widget
		prevHTML template.HTML
	}

	p.mu.Lock()

	var candidates []candidate
	var toUpdate []widget

	for _, w := range p.HeadWidgets {
		if w.requiresUpdate(&now) {
			candidates = append(candidates, candidate{wgt: w, prevHTML: w.Render()})
			toUpdate = append(toUpdate, w)
		}
	}
	for c := range p.Columns {
		for _, w := range p.Columns[c].Widgets {
			if w.requiresUpdate(&now) {
				candidates = append(candidates, candidate{wgt: w, prevHTML: w.Render()})
				toUpdate = append(toUpdate, w)
			}
		}
	}

	if len(toUpdate) == 0 {
		p.mu.Unlock()
		return
	}

	var wg sync.WaitGroup
	for _, w := range toUpdate {
		wg.Add(1)
		go func(w widget) {
			defer wg.Done()
			w.update(ctx)
		}(w)
	}
	wg.Wait()

	for _, c := range candidates {
		newHTML := c.wgt.Render()
		if newHTML != c.prevHTML {
			a.hub.publish(event{
				Type:     "widget-updated",
				WidgetID: c.wgt.GetID(),
				Time:     now,
			})
		}
	}

	p.mu.Unlock()
}
