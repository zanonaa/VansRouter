package providers

import (
	"sync"
	"testing"
)

func TestComboRoundRobinStickyAndReset(t *testing.T) {
	name := "test-sticky"
	ResetComboRotation(name)
	defer ResetComboRotation(name)
	models := []string{"a", "b", "c"}
	for i, want := range []string{"a", "a", "b", "b", "c", "c", "a"} {
		got := RotateCombo(models, name, "round-robin", 2)
		if got[0] != want {
			t.Fatalf("%d got %v expected %s", i, got, want)
		}
	}
	if models[0] != "a" {
		t.Fatal("mutated original")
	}
	ResetComboRotation(name)
	if got := RotateCombo(models, name, "round-robin", 2); got[0] != "a" {
		t.Fatal("reset failed")
	}
}
func TestComboRotationConcurrent(t *testing.T) {
	name := "concurrent"
	ResetComboRotation(name)
	defer ResetComboRotation(name)
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[string]int{}
	for i := 0; i < 90; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := RotateCombo([]string{"a", "b", "c"}, name, "round-robin", 1)
			mu.Lock()
			counts[got[0]]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	for _, id := range []string{"a", "b", "c"} {
		if counts[id] != 30 {
			t.Fatal(counts)
		}
	}
}
