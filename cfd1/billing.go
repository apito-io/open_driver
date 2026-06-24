package cfd1

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrDailyReadCapExceeded is returned when D1_DAILY_READ_CAP is exceeded.
var ErrDailyReadCapExceeded = errors.New("cfd1: daily read cap exceeded")

var (
	billingMu       sync.Mutex
	billingDay      string
	billingRowsRead int64
)

func dailyReadCap() int64 {
	raw := strings.TrimSpace(os.Getenv("D1_DAILY_READ_CAP"))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func resetBillingDayIfNeeded() {
	today := time.Now().UTC().Format("2006-01-02")
	billingMu.Lock()
	defer billingMu.Unlock()
	if billingDay != today {
		billingDay = today
		billingRowsRead = 0
	}
}

func recordRowsRead(n int64) error {
	if n <= 0 {
		return nil
	}
	cap := dailyReadCap()
	resetBillingDayIfNeeded()
	newTotal := atomic.AddInt64(&billingRowsRead, n)
	if cap > 0 && newTotal > cap {
		return ErrDailyReadCapExceeded
	}
	return nil
}

// DailyRowsRead returns the accumulated row-read estimate for the current UTC day.
func DailyRowsRead() int64 {
	resetBillingDayIfNeeded()
	return atomic.LoadInt64(&billingRowsRead)
}

// ResetBillingForTest resets the in-process daily counter (tests only).
func ResetBillingForTest() {
	billingMu.Lock()
	defer billingMu.Unlock()
	billingDay = time.Now().UTC().Format("2006-01-02")
	billingRowsRead = 0
}
