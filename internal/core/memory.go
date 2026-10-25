package core

type MemoryStatus struct {
	TotalMB          uint64 `json:"totalMB"`
	AvailableMB      uint64 `json:"availableMB"`
	UsedMB           uint64 `json:"usedMB"`
	RecommendedMinMB int    `json:"recommendedMinMB"`
	RecommendedMaxMB int    `json:"recommendedMaxMB"`
}

func memoryStatus(total, available uint64) MemoryStatus {
	if available > total {
		available = total
	}
	// Leave at least a quarter of available RAM for the OS and JVM overhead;
	// cap the default heap at 8 GiB. Modpacks may need a custom budget.
	budget := min(total/2, available*3/4, 8192)
	maximum := max(128, int(budget/256)*256)
	minimum := max(128, min(1024, maximum/4/128*128))
	return MemoryStatus{TotalMB: total, AvailableMB: available, UsedMB: total - available, RecommendedMinMB: minimum, RecommendedMaxMB: maximum}
}
