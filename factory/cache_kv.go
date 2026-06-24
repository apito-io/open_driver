package factory

import (
	"fmt"
	"log"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/cache/bbolt"
	"gitlab.com/apito.io/open_driver/cache/memory"
	"gitlab.com/apito.io/open_driver/cache/redis"
	kvbbolt "gitlab.com/apito.io/open_driver/kv/bbolt"
	kvmemory "gitlab.com/apito.io/open_driver/kv/memory"
	kvredis "gitlab.com/apito.io/open_driver/kv/redis"
)

// DefaultCacheFactory creates cache drivers for open-core engines.
type DefaultCacheFactory struct{}

func NewDefaultCacheFactory() interfaces.CacheFactory {
	return &DefaultCacheFactory{}
}

func (f *DefaultCacheFactory) Create(cfg *models.Config, engineType string) (interfaces.CacheDBInterface, error) {
	log.Printf("Creating cache driver: %s", engineType)
	switch engineType {
	case "redis":
		return redis.GetRedisCacheDriver(cfg)
	case "coredb", "coreDB", "bbolt", "bolt":
		return bbolt.GetBoltCacheDriver(cfg)
	case "memory":
		return memory.GetMemoryCacheDriver(cfg)
	default:
		return memory.GetMemoryCacheDriver(cfg)
	}
}

// DefaultKVFactory creates KV drivers for open-core engines.
type DefaultKVFactory struct{}

func NewDefaultKVFactory() interfaces.KVFactory {
	return &DefaultKVFactory{}
}

func (f *DefaultKVFactory) Create(engineType string, cfg *models.Config) (interfaces.KeyValueServiceInterface, error) {
	log.Printf("Creating KV driver: %s", engineType)
	switch engineType {
	case "redis":
		return kvredis.GetKVRedisDriver(cfg)
	case "coredb", "coreDB", "bbolt", "bolt":
		return kvbbolt.GetKVBoltDriver(cfg)
	case "memory":
		return kvmemory.GetKVMemoryDriver(cfg)
	default:
		return nil, fmt.Errorf("unsupported kv engine: %s", engineType)
	}
}
