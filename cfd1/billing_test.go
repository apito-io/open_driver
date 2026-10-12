package cfd1

import (
	"os"
	"testing"
)

func TestDailyReadCap(t *testing.T) {
	t.Setenv("D1_DAILY_READ_CAP", "100")
	ResetBillingForTest()
	if err := recordRowsRead(50); err != nil {
		t.Fatal(err)
	}
	if DailyRowsRead() != 50 {
		t.Fatalf("expected 50, got %d", DailyRowsRead())
	}
	if err := recordRowsRead(60); err != ErrDailyReadCapExceeded {
		t.Fatalf("expected cap error, got %v", err)
	}
	os.Unsetenv("D1_DAILY_READ_CAP")
	ResetBillingForTest()
}
