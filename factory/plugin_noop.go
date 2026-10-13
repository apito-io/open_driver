package factory

import (
	"context"

	"github.com/apito-io/engine/interfaces"
)

// NoopPluginHost satisfies PluginHost without loading third-party plugins.
// A functions-enabled binary can replace it with a plugin supervisor at boot.
type NoopPluginHost struct{}

func NewNoopPluginHost() interfaces.PluginHost {
	return &NoopPluginHost{}
}

func (h *NoopPluginHost) Load(ctx context.Context, runtime interfaces.PluginRuntime) error {
	return nil
}

func (h *NoopPluginHost) Wait() {}

func (h *NoopPluginHost) StartMonitoring(ctx context.Context, runtime interfaces.PluginRuntime) {}

func (h *NoopPluginHost) PrintRoutes() {}
