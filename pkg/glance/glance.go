package glance

import (
	g "github.com/glanceapp/glance/internal/glance"
)

func Serve(configPath string) error {
	return g.ServeApp(configPath)
}
