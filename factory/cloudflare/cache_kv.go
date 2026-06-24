//go:build cloudflare

package cloudflare

import (
	"log"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	cached1 "gitlab.com/apito.io/open_driver/cache/d1"
	"gitlab.com/apito.io/open_driver/cache/memory"
	kvd1 "gitlab.com/apito.io/open_driver/kv/d1"
	kvmemory "gitlab.com/apito.io/open_driver/kv/memory"
)

// CacheFactory uses D1 on Workers so cache survives isolate churn.
type CacheFactory struct{}

func NewCacheFactory() interfaces.CacheFactory {
	return &CacheFactory{}
}

func (f *CacheFactory) Create(cfg *models.Config, engineType string) (interfaces.CacheDBInterface, error) {
	log.Printf("cloudflare cache: %s", engineType)
	switch engineType {
	case "d1", "cloudflared1":
		return cached1.GetD1CacheDriver(cfg)
	case "memory":
		return memory.GetMemoryCacheDriver(cfg)
	default:
		log.Printf("cloudflare cache: falling back to d1 for engine %q", engineType)
		return cached1.GetD1CacheDriver(cfg)
	}
}

// KVFactory uses in-memory KV on Workers.
type KVFactory struct{}

func NewKVFactory() interfaces.KVFactory {
	return &KVFactory{}
}

func (f *KVFactory) Create(engineType string, cfg *models.Config) (interfaces.KeyValueServiceInterface, error) {
	log.Printf("cloudflare kv: %s", engineType)
	switch engineType {
	case "d1", "cloudflared1":
		return kvd1.GetKVD1Driver(cfg)
	case "memory", "kv", "":
		return kvmemory.GetKVMemoryDriver(cfg)
	default:
		log.Printf("cloudflare kv: falling back to d1 for engine %q", engineType)
		return kvd1.GetKVD1Driver(cfg)
	}
}
