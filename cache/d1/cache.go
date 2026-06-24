//go:build cloudflare

package d1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/cfd1"
)

type l1Entry struct {
	value     []byte
	expiresAt time.Time
}

// CacheDriver stores project/app cache entries in SYSTEM_D1 (shared across Worker isolates).
type CacheDriver struct {
	cfg   *models.Config
	db    *cfd1.DB
	once  sync.Once
	initErr error
	l1    sync.Map
	l1TTL time.Duration
}

func GetD1CacheDriver(cfg *models.Config) (*CacheDriver, error) {
	db, err := cfd1.NewDB(cfd1.DefaultConfig("bound", nil))
	if err != nil {
		return nil, err
	}
	return &CacheDriver{cfg: cfg, db: db, l1TTL: 45 * time.Second}, nil
}

func (c *CacheDriver) l1Get(key string) ([]byte, bool) {
	if v, ok := c.l1.Load(key); ok {
		e := v.(l1Entry)
		if time.Now().Before(e.expiresAt) {
			return e.value, true
		}
		c.l1.Delete(key)
	}
	return nil, false
}

func (c *CacheDriver) l1Set(key string, raw []byte) {
	c.l1.Store(key, l1Entry{value: raw, expiresAt: time.Now().Add(c.l1TTL)})
}

func (c *CacheDriver) l1Delete(keys ...string) {
	for _, k := range keys {
		c.l1.Delete(k)
	}
}

func (c *CacheDriver) ensureSchema(ctx context.Context) error {
	c.once.Do(func() {
		_, c.initErr = c.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS apito_cache (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at INTEGER
		)`)
		if c.initErr == nil {
			_, c.initErr = c.db.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_apito_cache_expires ON apito_cache(expires_at)`)
		}
	})
	return c.initErr
}

func (c *CacheDriver) parseTTL() time.Duration {
	if c.cfg == nil || c.cfg.CacheTTL == "" {
		return 600 * time.Second
	}
	if d, err := time.ParseDuration(c.cfg.CacheTTL); err == nil && d > 0 {
		return d
	}
	ttlSeconds, err := strconv.Atoi(c.cfg.CacheTTL)
	if err != nil || ttlSeconds <= 0 {
		return 600 * time.Second
	}
	return time.Duration(ttlSeconds) * time.Second
}

func (c *CacheDriver) putJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := c.ensureSchema(ctx); err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.l1Set(key, raw)
	var expiresAt any
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}
	_, err = c.db.Exec(ctx,
		`INSERT INTO apito_cache (key, value, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at`,
		key, string(raw), expiresAt,
	)
	return err
}

func (c *CacheDriver) getJSON(ctx context.Context, key string, dest any) error {
	if raw, ok := c.l1Get(key); ok {
		return json.Unmarshal(raw, dest)
	}
	if err := c.ensureSchema(ctx); err != nil {
		return err
	}
	row, err := c.db.QueryRow(ctx,
		`SELECT value FROM apito_cache
		 WHERE key = ? AND (expires_at IS NULL OR expires_at > ?)`,
		key, time.Now().Unix(),
	)
	if err != nil {
		return errors.New("cache not found. fetch one")
	}
	raw, ok := row["value"].(string)
	if !ok {
		if b, ok := row["value"].([]byte); ok {
			raw = string(b)
		}
	}
	if raw == "" {
		return errors.New("cache not found. fetch one")
	}
	c.l1Set(key, []byte(raw))
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		return err
	}
	return nil
}

func (c *CacheDriver) deleteKeys(ctx context.Context, keys ...string) error {
	c.l1Delete(keys...)
	if err := c.ensureSchema(ctx); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := c.db.Exec(ctx, `DELETE FROM apito_cache WHERE key = ?`, key); err != nil {
			return err
		}
	}
	return nil
}

func (c *CacheDriver) Put(ctx context.Context, id string, cache interface{}) error {
	return c.putJSON(ctx, id, cache, c.parseTTL())
}

func (c *CacheDriver) Get(ctx context.Context, id string) (interface{}, error) {
	var out interface{}
	if err := c.getJSON(ctx, id, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *CacheDriver) ListKeys(ctx context.Context) ([]string, error) {
	if err := c.ensureSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := c.db.Query(ctx,
		`SELECT key FROM apito_cache WHERE expires_at IS NULL OR expires_at > ?`,
		time.Now().Unix(),
	)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if k, ok := row["key"].(string); ok && k != "" {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (c *CacheDriver) appCacheKey(projectID string) string {
	return fmt.Sprintf("appcache:%s", projectID)
}

func (c *CacheDriver) projectKey(projectID string) string {
	return fmt.Sprintf("project:%s", projectID)
}

func (c *CacheDriver) bundleKey(projectID string) string {
	return fmt.Sprintf("project_bundle:%s", projectID)
}

type projectBundle struct {
	Project   *models.Project          `json:"project,omitempty"`
	AppCache  *models.ApplicationCache `json:"app_cache,omitempty"`
}

func (c *CacheDriver) GetAppCache(ctx context.Context, projectID string) (*models.ApplicationCache, error) {
	var bundle projectBundle
	if err := c.getJSON(ctx, c.bundleKey(projectID), &bundle); err == nil && bundle.AppCache != nil {
		if bundle.Project != nil {
			bundle.AppCache.Project = bundle.Project
		}
		return bundle.AppCache, nil
	}
	key := c.appCacheKey(projectID)
	var cache models.ApplicationCache
	if err := c.getJSON(ctx, key, &cache); err != nil {
		return nil, err
	}
	if p, err := c.GetProject(ctx, projectID); err == nil && p != nil {
		cache.Project = p
	}
	return &cache, nil
}

func (c *CacheDriver) PutAppCache(ctx context.Context, projectID string, cache *models.ApplicationCache) error {
	if cache == nil {
		return errors.New("cache is nil")
	}
	copy := *cache
	project := copy.Project
	copy.Project = nil
	bundle := projectBundle{Project: project, AppCache: &copy}
	if err := c.putJSON(ctx, c.bundleKey(projectID), &bundle, c.parseTTL()); err != nil {
		return err
	}
	return c.putJSON(ctx, c.appCacheKey(projectID), &copy, c.parseTTL())
}

func (c *CacheDriver) Expire(ctx context.Context, id string) error {
	return c.deleteKeys(ctx, id, c.appCacheKey(id), c.projectKey(id), c.bundleKey(id))
}

func (c *CacheDriver) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	var project models.Project
	if err := c.getJSON(ctx, c.projectKey(projectID), &project); err != nil {
		return nil, err
	}
	return &project, nil
}

func (c *CacheDriver) SaveProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	if project == nil {
		return nil, errors.New("project is nil")
	}
	if err := c.putJSON(ctx, c.projectKey(project.ID), project, c.parseTTL()); err != nil {
		return nil, err
	}
	var bundle projectBundle
	_ = c.getJSON(ctx, c.bundleKey(project.ID), &bundle)
	bundle.Project = project
	_ = c.putJSON(ctx, c.bundleKey(project.ID), &bundle, c.parseTTL())
	return project, nil
}
