//go:build cloudflare

package d1

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/open_driver/cfd1"
	kvmemory "github.com/apito-io/open_driver/kv/memory"
)

// KVD1Service persists KV entries in SYSTEM_D1 so JWT sessions survive Worker isolate churn.
type KVD1Service struct {
	db     *cfd1.DB
	memory *kvmemory.KVMemoryService
	once   sync.Once
	initErr error
}

func GetKVD1Driver(cfg *models.Config) (*KVD1Service, error) {
	db, err := cfd1.NewDB(cfd1.DefaultConfig("bound", nil))
	if err != nil {
		return nil, err
	}
	mem, err := kvmemory.GetKVMemoryDriver(cfg)
	if err != nil {
		return nil, err
	}
	return &KVD1Service{db: db, memory: mem}, nil
}

func (k *KVD1Service) ensureSchema(ctx context.Context) error {
	k.once.Do(func() {
		_, k.initErr = k.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS apito_kv (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at INTEGER
		)`)
		if k.initErr == nil {
			_, k.initErr = k.db.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_apito_kv_expires ON apito_kv(expires_at)`)
		}
	})
	return k.initErr
}

func (k *KVD1Service) SetValue(ctx context.Context, key string, value string, expiration time.Duration) error {
	if err := k.ensureSchema(ctx); err != nil {
		return err
	}
	var expiresAt any
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration).Unix()
	}
	_, err := k.db.Exec(ctx,
		`INSERT INTO apito_kv (key, value, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at`,
		key, value, expiresAt,
	)
	return err
}

func (k *KVD1Service) GetValue(ctx context.Context, key string) (string, error) {
	if err := k.ensureSchema(ctx); err != nil {
		return "", err
	}
	row, err := k.db.QueryRow(ctx,
		`SELECT value FROM apito_kv
		 WHERE key = ? AND (expires_at IS NULL OR expires_at > ?)`,
		key, time.Now().Unix(),
	)
	if err != nil {
		return "", errors.New("key not found")
	}
	val, ok := row["value"].(string)
	if !ok || val == "" {
		if v, ok := row["value"].([]byte); ok {
			val = string(v)
		}
	}
	if val == "" {
		return "", errors.New("key not found")
	}
	return val, nil
}

func (k *KVD1Service) DelValue(ctx context.Context, key string) error {
	if err := k.ensureSchema(ctx); err != nil {
		return err
	}
	_, err := k.db.Exec(ctx, `DELETE FROM apito_kv WHERE key = ?`, key)
	return err
}

func (k *KVD1Service) CheckRedisKey(ctx context.Context, keys ...string) (bool, error) {
	for _, key := range keys {
		if _, err := k.GetValue(ctx, key); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func (k *KVD1Service) SetJSONObject(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return fmt.Errorf("d1 kv: SetJSONObject not implemented")
}

func (k *KVD1Service) GetJSONObject(ctx context.Context, key string) (interface{}, error) {
	return nil, fmt.Errorf("d1 kv: GetJSONObject not implemented")
}

func (k *KVD1Service) AddToSortedSets(ctx context.Context, setName string, key string, exp time.Duration) error {
	return k.memory.AddToSortedSets(ctx, setName, key, exp)
}

func (k *KVD1Service) GetFromSortedSets(ctx context.Context, setName string, key string) (float64, error) {
	return k.memory.GetFromSortedSets(ctx, setName, key)
}

func (k *KVD1Service) SetToHashMap(ctx context.Context, hash, key string, value string) error {
	return k.memory.SetToHashMap(ctx, hash, key, value)
}

func (k *KVD1Service) GetFromHashMap(ctx context.Context, hash, key string) (string, error) {
	return k.memory.GetFromHashMap(ctx, hash, key)
}

func (k *KVD1Service) CheckKeyHashMap(ctx context.Context, hash, key string) bool {
	return k.memory.CheckKeyHashMap(ctx, hash, key)
}

func (k *KVD1Service) AddToSets(ctx context.Context, key string, value string) error {
	return k.memory.AddToSets(ctx, key, value)
}

func (k *KVD1Service) RemoveSets(ctx context.Context, key string, value string) error {
	return k.memory.RemoveSets(ctx, key, value)
}
