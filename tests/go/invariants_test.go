package sdet

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// Implement I1–I6 against API_URL. Reset state at the start of every test
// and restore the authority to healthy, even on failure.
//
//   I1  Same Idempotency-Key + user → one reservation (50 sequential + 50 concurrent)
//   I2  Never oversell: 200 concurrent reserves on stock 50 → 50 confirmed, 150 rejected
//   I3  After an outage, no reservation remains pending
//   I4  Status only moves pending → confirmed | reversed
//   I5  Same key, different body → 422
//   I6  /health.mode reflects the authority within 5s

func target() string {
	if v := os.Getenv("TARGET"); v != "" {
		return v
	}
	return "reference"
}

func TestI1_IdempotentReplay(t *testing.T) {
	c := NewClient()
	t.Cleanup(func() { _ = c.SetMode("healthy") })
	if err := c.Reset(); err != nil {
		t.Fatal(err)
	}

	user := "u-i1"
	key := "idem-i1"
	firstID := ""
	for i := 0; i < 50; i++ {
		code, out, err := c.Reserve(user, "hype-001", key, 1)
		if err != nil {
			Record("I1", target(), false, err.Error())
			t.Fatal(err)
		}
		if code != 201 && code != 200 {
			Record("I1", target(), false, fmt.Sprintf("unexpected status %d for sequential replay", code))
			t.Fatalf("unexpected status %d for sequential replay: %#v", code, out)
		}
		id := fmt.Sprint(out["reservationId"])
		if firstID == "" {
			firstID = id
		} else if id != firstID {
			Record("I1", target(), false, fmt.Sprintf("replay changed reservationId: %s != %s", id, firstID))
			t.Fatalf("sequential replay changed reservationId: %s != %s", id, firstID)
		}
	}

	var (
		mu  sync.Mutex
		ids = map[string]int{}
	)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, out, err := c.Reserve(user, "hype-001", key, 1)
			if err != nil {
				Record("I1", target(), false, err.Error())
				t.Errorf("concurrent replay failed: %v", err)
				return
			}
			mu.Lock()
			ids[fmt.Sprint(out["reservationId"])]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(ids) != 1 {
		Record("I1", target(), false, fmt.Sprintf("expected 1 reservationId from 50 concurrent replays, got %d", len(ids)))
		t.Fatalf("expected 1 reservationId from 50 concurrent replays, got %d (%v)", len(ids), ids)
	}
	Record("I1", target(), true, "one idempotent reservation across 50 sequential + 50 concurrent replays")
}

func TestI2_NeverOversell(t *testing.T) {
	c := NewClient()
	t.Cleanup(func() { _ = c.SetMode("healthy") })
	if err := c.Reset(); err != nil {
		t.Fatal(err)
	}

	var (
		mu        sync.Mutex
		confirmed int
		rejected  int
	)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			code, out, err := c.Reserve(fmt.Sprintf("u-%d", idx), "hype-001", fmt.Sprintf("i2-%d", idx), 1)
			if err != nil {
				Record("I2", target(), false, err.Error())
				t.Errorf("reservation %d failed: %v", idx, err)
				return
			}
			status := fmt.Sprint(out["status"])
			if code == 201 || status == "confirmed" {
				mu.Lock()
				confirmed++
				mu.Unlock()
				return
			}
			mu.Lock()
			rejected++
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()

	if confirmed != 50 || rejected != 150 {
		Record("I2", target(), false, fmt.Sprintf("expected 50 confirmed and 150 rejected; got %d/%d", confirmed, rejected))
		t.Fatalf("expected 50 confirmed and 150 rejected; got %d confirmed and %d rejected", confirmed, rejected)
	}
	Record("I2", target(), true, "stock of 50 allowed exactly 50 confirmed reservations")
}

func TestI3_ReplayCompleteness(t *testing.T) {
	c := NewClient()
	t.Cleanup(func() { _ = c.SetMode("healthy") })
	if err := c.Reset(); err != nil {
		t.Fatal(err)
	}

	if err := c.SetMode("down"); err != nil {
		Record("I3", target(), false, err.Error())
		t.Fatal(err)
	}
	if _, _, err := c.Reserve("u-i3-1", "hype-001", "i3-a", 1); err != nil {
		Record("I3", target(), false, err.Error())
		t.Fatal(err)
	}
	if _, _, err := c.Reserve("u-i3-2", "hype-001", "i3-b", 1); err != nil {
		Record("I3", target(), false, err.Error())
		t.Fatal(err)
	}
	if _, _, err := c.Reserve("u-i3-3", "hype-001", "i3-c", 1); err != nil {
		Record("I3", target(), false, err.Error())
		t.Fatal(err)
	}

	if err := c.SetMode("healthy"); err != nil {
		Record("I3", target(), false, err.Error())
		t.Fatal(err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		pending := 0
		for _, user := range []string{"u-i3-1", "u-i3-2", "u-i3-3"} {
			list, err := c.List(user)
			if err != nil {
				Record("I3", target(), false, err.Error())
				t.Fatal(err)
			}
			for _, row := range list {
				if fmt.Sprint(row["status"]) == "pending" {
					pending++
				}
			}
		}
		if pending == 0 {
			Record("I3", target(), true, "all reservations were replayed and no pending entries remained")
			return
		}
		time.Sleep(300 * time.Millisecond)
	}

	Record("I3", target(), false, "pending reservation(s) remained after authority recovery")
	t.Fatal("pending reservation(s) remained after authority recovery")
}

func TestI4_StatusMonotonic(t *testing.T) {
	c := NewClient()
	t.Cleanup(func() { _ = c.SetMode("healthy") })
	if err := c.Reset(); err != nil {
		t.Fatal(err)
	}

	if err := c.SetMode("down"); err != nil {
		Record("I4", target(), false, err.Error())
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	_, out, err := c.Reserve("u-i4", "hype-001", "i4-key", 1)
	if err != nil {
		Record("I4", target(), false, err.Error())
		t.Fatal(err)
	}
	if fmt.Sprint(out["status"]) != "pending" {
		Record("I4", target(), false, fmt.Sprintf("expected a pending reservation while the authority was down; got %#v", out))
		t.Fatalf("expected a pending reservation while the authority was down; got %#v", out)
	}
	if err := c.SetMode("healthy"); err != nil {
		Record("I4", target(), false, err.Error())
		t.Fatal(err)
	}

	deadline := time.Now().Add(15 * time.Second)
	seenPending := false
	seenTerminal := false
	for time.Now().Before(deadline) {
		list, err := c.List("u-i4")
		if err != nil {
			Record("I4", target(), false, err.Error())
			t.Fatal(err)
		}
		for _, row := range list {
			status := fmt.Sprint(row["status"])
			if status == "pending" {
				seenPending = true
			}
			if status == "confirmed" || status == "reversed" {
				seenTerminal = true
			}
			if status != "pending" && status != "confirmed" && status != "reversed" {
				Record("I4", target(), false, fmt.Sprintf("unexpected status %q", status))
				t.Fatalf("unexpected status %q", status)
			}
		}
		if seenPending && seenTerminal {
			Record("I4", target(), true, "reservation state only moved pending → confirmed | reversed")
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	Record("I4", target(), false, "reservation never reached a valid terminal state after recovery")
	t.Fatal("reservation never reached a valid terminal state after recovery")
}

func TestI5_KeyMismatch(t *testing.T) {
	c := NewClient()
	t.Cleanup(func() { _ = c.SetMode("healthy") })
	if err := c.Reset(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := c.Reserve("u-i5", "hype-001", "i5-key", 1); err != nil {
		Record("I5", target(), false, err.Error())
		t.Fatal(err)
	}
	code, out, err := c.Reserve("u-i5", "hype-002", "i5-key", 1)
	if err != nil {
		Record("I5", target(), false, err.Error())
		t.Fatal(err)
	}
	if code != 422 {
		Record("I5", target(), false, fmt.Sprintf("expected 422 for same key with different body; got %d: %#v", code, out))
		t.Fatalf("expected 422 for same key with different body; got %d: %#v", code, out)
	}
	Record("I5", target(), true, "same idempotency key with different payload was rejected")
}

func TestI6_HealthHonesty(t *testing.T) {
	c := NewClient()
	t.Cleanup(func() { _ = c.SetMode("healthy") })
	if err := c.Reset(); err != nil {
		t.Fatal(err)
	}

	if err := c.SetMode("down"); err != nil {
		Record("I6", target(), false, err.Error())
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h, err := c.Health()
		if err == nil && fmt.Sprint(h["mode"]) == "down" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if mode, _ := c.Health(); fmt.Sprint(mode["mode"]) != "down" {
		Record("I6", target(), false, fmt.Sprintf("health mode did not change to down within 5s: %#v", mode))
		t.Fatalf("health mode did not change to down within 5s: %#v", mode)
	}

	if err := c.SetMode("healthy"); err != nil {
		Record("I6", target(), false, err.Error())
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h, err := c.Health()
		if err == nil && fmt.Sprint(h["mode"]) == "healthy" {
			Record("I6", target(), true, "API health mode tracked authority mode")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	Record("I6", target(), false, "health mode never returned to healthy after authority restored")
	t.Fatal("health mode never returned to healthy after authority restored")
}
