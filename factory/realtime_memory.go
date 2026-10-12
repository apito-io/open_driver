package factory

import (
	"fmt"
	"log"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/open_driver/realtime/memory"
)

// DefaultMemoryRealtimeFactory creates the in-process memory realtime bus.
type DefaultMemoryRealtimeFactory struct{}

func NewDefaultMemoryRealtimeFactory() interfaces.RealtimeBusFactory {
	return &DefaultMemoryRealtimeFactory{}
}

func (f *DefaultMemoryRealtimeFactory) Create(engineType string, cfg *models.Config) (interfaces.RealtimeBus, error) {
	log.Printf("Creating memory realtime bus: %s", engineType)
	switch engineType {
	case "memory", "":
		return memory.GetMemoryRealtimeBus(cfg)
	default:
		return nil, fmt.Errorf("memory realtime factory does not support engine: %s", engineType)
	}
}
