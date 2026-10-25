package core

import "testing"

func TestRecommendedMemoryBudget(t *testing.T) {
	for _, tc := range []struct {
		total, available uint64
		low, high        int
	}{
		{16384, 8192, 1024, 6144}, {8192, 4096, 768, 3072},
		{65536, 50000, 1024, 8192}, {2048, 512, 128, 256},
		{16384, 0, 128, 128}, {4096, 8192, 512, 2048},
	} {
		got := memoryStatus(tc.total, tc.available)
		if got.RecommendedMinMB != tc.low || got.RecommendedMaxMB != tc.high || got.UsedMB+got.AvailableMB != got.TotalMB || got.AvailableMB > got.TotalMB {
			t.Fatalf("memory recommendation for %d/%d: %+v", tc.total, tc.available, got)
		}
		options := DefaultOptions()
		options.MinMemoryMB, options.MaxMemoryMB = got.RecommendedMinMB, got.RecommendedMaxMB
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSystemMemory(t *testing.T) {
	status, err := SystemMemory()
	if err != nil {
		t.Skip(err)
	}
	if status.TotalMB == 0 || status.UsedMB+status.AvailableMB != status.TotalMB {
		t.Fatal(status)
	}
}
