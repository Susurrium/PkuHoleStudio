package db

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPostUpdateRoundClaimIsAtomicAcrossConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedule.db")
	first := openDatabaseAt(t, path)
	defer first.Close()
	second := openDatabaseAt(t, path)
	defer second.Close()
	ctx := context.Background()
	if err := first.WritePostUpdateSchedule(ctx, "account", "original"); err != nil {
		t.Fatal(err)
	}
	var winners atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			repo := first
			if index%2 == 1 {
				repo = second
			}
			won, err := repo.ComparePostUpdateSchedule(ctx, "account", "original", fmt.Sprintf("claimed-%d", index))
			if err != nil {
				t.Error(err)
			}
			if won {
				winners.Add(1)
			}
		}(i)
	}
	group.Wait()
	if winners.Load() != 1 {
		t.Fatalf("claimed by %d executors", winners.Load())
	}
	if err := second.WritePostUpdateSchedule(ctx, "account", "disabled"); err != nil {
		t.Fatal(err)
	}
	if won, err := first.ComparePostUpdateSchedule(ctx, "account", "original", "late progress"); err != nil || won {
		t.Fatalf("late worker overwrote new settings: won=%v err=%v", won, err)
	}
	state, err := first.ReadPostUpdateSchedule(ctx, "account")
	if err != nil || state != "disabled" {
		t.Fatalf("new settings lost: %q %v", state, err)
	}
}
