package monitor

const (
	ViewPermission   = "system:monitor:view"
	UpdatePermission = "system:monitor:update"
)

type Status struct {
	Enabled                 bool     `json:"enabled"`
	SamplingIntervalSeconds int      `json:"samplingIntervalSeconds"`
	Metrics                 *Metrics `json:"metrics"`
	LastError               string   `json:"lastError"`
}

type Metrics struct {
	SampledAt   string             `json:"sampledAt"`
	CPU         CPUMetrics         `json:"cpu"`
	Memory      MemoryMetrics      `json:"memory"`
	Host        HostMetrics        `json:"host"`
	Application ApplicationMetrics `json:"application"`
}

type CPUMetrics struct {
	UsagePercent  float64 `json:"usagePercent"`
	LogicalCores  int     `json:"logicalCores"`
	PhysicalCores int     `json:"physicalCores"`
}

type MemoryMetrics struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type HostMetrics struct {
	Hostname        string   `json:"hostname"`
	ServerIP        string   `json:"serverIp"`
	IPAddresses     []string `json:"ipAddresses"`
	OS              string   `json:"os"`
	Platform        string   `json:"platform"`
	PlatformVersion string   `json:"platformVersion"`
	KernelVersion   string   `json:"kernelVersion"`
	Architecture    string   `json:"architecture"`
	BootTime        string   `json:"bootTime"`
	UptimeSeconds   uint64   `json:"uptimeSeconds"`
}

type ApplicationMetrics struct {
	GoVersion       string `json:"goVersion"`
	PID             int    `json:"pid"`
	UptimeSeconds   uint64 `json:"uptimeSeconds"`
	Goroutines      int    `json:"goroutines"`
	HeapAllocBytes  uint64 `json:"heapAllocBytes"`
	HeapSystemBytes uint64 `json:"heapSystemBytes"`
	MemoryBytes     uint64 `json:"memoryBytes"`
	GCCount         uint32 `json:"gcCount"`
}
