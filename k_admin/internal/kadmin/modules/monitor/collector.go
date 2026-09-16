package monitor

import (
	"context"
	"errors"
	"math"
	"net"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type Collector interface {
	Collect(context.Context) (Metrics, error)
}

type systemCollector struct {
	startedAt time.Time
	mu        sync.Mutex
	loaded    bool
	host      HostMetrics
	cpu       CPUMetrics
}

func newSystemCollector(startedAt time.Time) Collector {
	return &systemCollector{startedAt: startedAt}
}

func (c *systemCollector) Collect(ctx context.Context) (Metrics, error) {
	if err := c.loadStatic(ctx); err != nil {
		return Metrics{}, err
	}

	percentages, err := cpu.PercentWithContext(ctx, 150*time.Millisecond, false)
	if err != nil {
		return Metrics{}, err
	}
	if len(percentages) == 0 {
		return Metrics{}, errors.New("cpu usage is unavailable")
	}
	memory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return Metrics{}, err
	}
	uptime, err := host.UptimeWithContext(ctx)
	if err != nil {
		return Metrics{}, err
	}

	var runtimeMemory runtime.MemStats
	runtime.ReadMemStats(&runtimeMemory)
	now := time.Now()
	hostMetrics := c.host
	hostMetrics.UptimeSeconds = uptime
	hostMetrics.BootTime = now.Add(-time.Duration(uptime) * time.Second).Format(time.RFC3339)
	cpuMetrics := c.cpu
	cpuMetrics.UsagePercent = roundPercent(percentages[0])

	return Metrics{
		SampledAt: now.Format(time.RFC3339),
		CPU:       cpuMetrics,
		Memory: MemoryMetrics{
			TotalBytes:     memory.Total,
			UsedBytes:      memory.Used,
			AvailableBytes: memory.Available,
			UsagePercent:   roundPercent(memory.UsedPercent),
		},
		Host: hostMetrics,
		Application: ApplicationMetrics{
			GoVersion:       runtime.Version(),
			PID:             os.Getpid(),
			UptimeSeconds:   uint64(now.Sub(c.startedAt).Seconds()),
			Goroutines:      runtime.NumGoroutine(),
			HeapAllocBytes:  runtimeMemory.HeapAlloc,
			HeapSystemBytes: runtimeMemory.HeapSys,
			MemoryBytes:     runtimeMemory.Sys,
			GCCount:         runtimeMemory.NumGC,
		},
	}, nil
}

func (c *systemCollector) loadStatic(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return nil
	}

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return err
	}
	logicalCores, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return err
	}
	physicalCores, err := cpu.CountsWithContext(ctx, false)
	if err != nil || physicalCores <= 0 {
		physicalCores = logicalCores
	}
	addresses := serverIPAddresses()
	serverIP := ""
	if len(addresses) > 0 {
		serverIP = addresses[0]
	}

	c.host = HostMetrics{
		Hostname:        info.Hostname,
		ServerIP:        serverIP,
		IPAddresses:     addresses,
		OS:              info.OS,
		Platform:        info.Platform,
		PlatformVersion: info.PlatformVersion,
		KernelVersion:   info.KernelVersion,
		Architecture:    info.KernelArch,
	}
	c.cpu = CPUMetrics{LogicalCores: logicalCores, PhysicalCores: physicalCores}
	c.loaded = true
	return nil
}

func serverIPAddresses() []string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return []string{}
	}
	type rankedIP struct {
		address string
		rank    int
	}
	ranked := make([]rankedIP, 0, len(addresses))
	seen := make(map[string]bool)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr != nil || ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			continue
		}
		text := ip.String()
		if seen[text] {
			continue
		}
		seen[text] = true
		rank := 2
		if ip.To4() != nil {
			rank = 1
			if ip.IsPrivate() {
				rank = 0
			}
		}
		ranked = append(ranked, rankedIP{address: text, rank: rank})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].rank != ranked[j].rank {
			return ranked[i].rank < ranked[j].rank
		}
		return ranked[i].address < ranked[j].address
	})
	result := make([]string, 0, len(ranked))
	for _, item := range ranked {
		result = append(result, item.address)
	}
	return result
}

func roundPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 100 {
		value = 100
	}
	return math.Round(value*100) / 100
}
