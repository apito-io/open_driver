package cfd1

import "errors"

var (
	// ErrNotImplemented is returned by cloudflared1 methods not yet ported to D1.
	ErrNotImplemented = errors.New("cloudflared1: not implemented")
	// ErrCredentialsRequired indicates missing D1 database id or API credentials.
	ErrCredentialsRequired = errors.New("cloudflared1: credentials required")
)
