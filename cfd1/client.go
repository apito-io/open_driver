// Package cfd1 executes SQL against Cloudflare D1 (bound binding or REST API).
package cfd1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
)

const (
	// EngineName is the Apito driver engine identifier for Cloudflare D1.
	EngineName = "cloudflared1"
)

// AccessMode selects how SQL reaches D1.
type AccessMode int

const (
	// AccessBound uses a statically bound D1 database (system/starter).
	AccessBound AccessMode = iota
	// AccessREST uses Cloudflare D1 HTTP API (dynamic project/tenant DBs).
	AccessREST
)

// BoundExecFunc runs SQL via a Worker-bound D1 database.
type BoundExecFunc func(ctx context.Context, sql string, params []any) (Result, error)

// Config identifies which D1 database to query.
type Config struct {
	Mode       AccessMode
	DatabaseID string // D1 UUID
	AccountID  string
	APIToken   string
	// BoundExec, when set, runs SQL via Worker D1 binding (wasm entry injects this).
	BoundExec BoundExecFunc
}

// DB is a lightweight D1 SQL executor (no database/sql, no bun).
type DB struct {
	cfg Config
}

// Result is a single D1 statement outcome.
type Result struct {
	Rows         []map[string]any
	RowsAffected int64
	LastInsertID int64
	RowsRead     int64
}

// NewDB opens a D1 executor for the given config.
func NewDB(cfg Config) (*DB, error) {
	id := strings.TrimSpace(cfg.DatabaseID)
	if id == "" {
		if cfg.Mode == AccessBound {
			id = "bound"
		} else {
			return nil, fmt.Errorf("cfd1: database id required")
		}
	}
	if cfg.Mode == AccessREST {
		if strings.TrimSpace(cfg.AccountID) == "" || strings.TrimSpace(cfg.APIToken) == "" {
			return nil, fmt.Errorf("cfd1: account id and api token required for REST mode")
		}
	}
	return &DB{cfg: cfg}, nil
}

// DatabaseID returns the D1 database UUID.
func (d *DB) DatabaseID() string {
	if d == nil {
		return ""
	}
	return d.cfg.DatabaseID
}

// Exec runs a write statement.
func (d *DB) Exec(ctx context.Context, sql string, params ...any) (Result, error) {
	return d.run(ctx, sql, params)
}

// Query runs a read statement.
func (d *DB) Query(ctx context.Context, sql string, params ...any) ([]map[string]any, error) {
	res, err := d.run(ctx, sql, params)
	if err != nil {
		return nil, err
	}
	return res.Rows, nil
}

// QueryRow runs a read expecting zero or one row.
func (d *DB) QueryRow(ctx context.Context, sql string, params ...any) (map[string]any, error) {
	rows, err := d.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (d *DB) run(ctx context.Context, sql string, params []any) (Result, error) {
	if d == nil {
		return Result{}, fmt.Errorf("cfd1: nil db")
	}
	if err := checkDailyReadCap(); err != nil {
		return Result{}, err
	}
	var (
		res Result
		err error
	)
	switch d.cfg.Mode {
	case AccessBound:
		if d.cfg.BoundExec == nil {
			return Result{}, fmt.Errorf("cfd1: bound exec not configured")
		}
		res, err = d.cfg.BoundExec(ctx, sql, params)
	default:
		res, err = d.execREST(ctx, sql, params)
	}
	if err != nil {
		return res, err
	}
	rowsRead := res.RowsRead
	if rowsRead <= 0 && len(res.Rows) > 0 {
		rowsRead = int64(len(res.Rows))
	}
	if err := recordRowsRead(rowsRead); err != nil {
		return res, err
	}
	return res, nil
}

func checkDailyReadCap() error {
	cap := dailyReadCap()
	if cap <= 0 {
		return nil
	}
	resetBillingDayIfNeeded()
	if atomic.LoadInt64(&billingRowsRead) >= cap {
		return ErrDailyReadCapExceeded
	}
	return nil
}

type restQueryRequest struct {
	SQL    string `json:"sql"`
	Params []any  `json:"params,omitempty"`
}

type restQueryResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Changes   int64 `json:"changes"`
			LastRowID int64 `json:"last_row_id"`
			RowsRead  int64 `json:"rows_read"`
		} `json:"meta"`
	} `json:"result"`
}

func (d *DB) execREST(ctx context.Context, sql string, params []any) (Result, error) {
	body, err := json.Marshal(restQueryRequest{SQL: sql, Params: params})
	if err != nil {
		return Result{}, err
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		strings.TrimSpace(d.cfg.AccountID), strings.TrimSpace(d.cfg.DatabaseID))
	status, raw, err := restHTTPPost(ctx, url, d.cfg.APIToken, body)
	if err != nil {
		return Result{}, err
	}
	if status >= 400 {
		return Result{}, fmt.Errorf("cfd1 rest %d: %s", status, string(raw))
	}
	var parsed restQueryResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Result{}, err
	}
	if !parsed.Success {
		if len(parsed.Errors) > 0 {
			return Result{}, fmt.Errorf("cfd1: %s", parsed.Errors[0].Message)
		}
		return Result{}, fmt.Errorf("cfd1: query failed")
	}
	out := Result{}
	if len(parsed.Result) > 0 {
		out.Rows = parsed.Result[0].Results
		out.RowsAffected = parsed.Result[0].Meta.Changes
		out.LastInsertID = parsed.Result[0].Meta.LastRowID
		out.RowsRead = parsed.Result[0].Meta.RowsRead
	}
	return out, nil
}

// CreateDatabase provisions a new D1 database via Cloudflare API.
func CreateDatabase(ctx context.Context, accountID, apiToken, name string) (databaseID string, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("cfd1: database name required")
	}
	payload := map[string]string{"name": name}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database", strings.TrimSpace(accountID))
	status, raw, err := restHTTPPost(ctx, url, apiToken, body)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("cfd1 create db %d: %s", status, string(raw))
	}
	var parsed struct {
		Success bool `json:"success"`
		Result  struct {
			UUID string `json:"uuid"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if !parsed.Success {
		if len(parsed.Errors) > 0 {
			return "", fmt.Errorf("cfd1: %s", parsed.Errors[0].Message)
		}
		return "", fmt.Errorf("cfd1: create database failed")
	}
	if strings.TrimSpace(parsed.Result.UUID) == "" {
		return "", fmt.Errorf("cfd1: empty database uuid")
	}
	return parsed.Result.UUID, nil
}

// DeleteDatabase removes a D1 database via Cloudflare API.
func DeleteDatabase(ctx context.Context, accountID, apiToken, databaseID string) error {
	databaseID = strings.TrimSpace(databaseID)
	if databaseID == "" {
		return fmt.Errorf("cfd1: database id required")
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s",
		strings.TrimSpace(accountID), databaseID)
	status, raw, err := restHTTPDelete(ctx, url, apiToken)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("cfd1 delete db %d: %s", status, string(raw))
	}
	var parsed struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return err
		}
		if !parsed.Success && len(parsed.Errors) > 0 {
			return fmt.Errorf("cfd1: %s", parsed.Errors[0].Message)
		}
	}
	return nil
}
