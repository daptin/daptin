package server

import (
	"github.com/daptin/daptin/server/database"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
	"net/http"
	"sync"
	"time"
)

// StatCache holds cached statistics data with timestamps
type StatCache struct {
	data      interface{}
	timestamp time.Time
}

// HostStats holds all host statistics with caching
type HostStats struct {
	mutex         sync.RWMutex
	cacheValidity time.Duration
	cpuCache      StatCache
	memCache      StatCache
	diskCache     StatCache
	netCache      StatCache
	hostCache     StatCache
	loadCache     StatCache
	processCache  StatCache
}

// NewHostStats creates a new HostStats instance with specified cache validity duration
func NewHostStats(cacheValidity time.Duration) *HostStats {
	return &HostStats{
		cacheValidity: cacheValidity,
	}
}

// GetCPUInfo returns CPU information with caching
func (hs *HostStats) GetCPUInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.cpuCache.data != nil && time.Since(hs.cpuCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.cpuCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get CPU info
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	// Get CPU percentage
	cpuPercent, err := cpu.Percent(0, true)
	if err != nil {
		return nil, err
	}

	// Get CPU counts
	cpuCounts, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"info":    cpuInfo,
		"percent": cpuPercent,
		"counts":  cpuCounts,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.cpuCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// GetMemInfo returns memory information with caching
func (hs *HostStats) GetMemInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.memCache.data != nil && time.Since(hs.memCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.memCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get virtual memory stats
	virtualMem, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	// Get swap memory stats
	swapMem, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"virtual": virtualMem,
		"swap":    swapMem,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.memCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// GetDiskInfo returns disk information with caching
func (hs *HostStats) GetDiskInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.diskCache.data != nil && time.Since(hs.diskCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.diskCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get disk partitions
	//partitions, err := disk.Partitions(true)
	//if err != nil {
	//	return nil, err
	//}

	// Get disk IO counters
	ioCounters, err := disk.IOCounters()
	if err != nil {
		return nil, err
	}

	// Get usage for each partition
	//usageStats := make(map[string]*disk.UsageStat)
	//for _, partition := range partitions {
	//	usage, err := disk.Usage(partition.Mountpoint)
	//	if err != nil {
	//		continue // Skip this partition if there's an error
	//	}
	//	usageStats[partition.Mountpoint] = usage
	//}

	result := map[string]interface{}{
		//"partitions": partitions,
		"io": ioCounters,
		//"usage":      usageStats,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.diskCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// GetNetInfo returns network information with caching
func (hs *HostStats) GetNetInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.netCache.data != nil && time.Since(hs.netCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.netCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// Get IO counters
	ioCounters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	// Get connection stats
	connections, err := net.Connections("all")
	if err != nil {
		// This might fail due to permissions, so we'll just continue without it
		connections = nil
	}

	result := map[string]interface{}{
		"interfaces":  interfaces,
		"io":          ioCounters,
		"connections": connections,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.netCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// GetHostInfo returns host information with caching
func (hs *HostStats) GetHostInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.hostCache.data != nil && time.Since(hs.hostCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.hostCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get host info
	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	// Get temperature sensors (might not be available on all platforms)
	temps, _ := sensors.SensorsTemperatures()

	// Get users
	users, _ := host.Users()

	result := map[string]interface{}{
		"info":         hostInfo,
		"temperatures": temps,
		"users":        users,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.hostCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// GetLoadInfo returns load average information with caching
func (hs *HostStats) GetLoadInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.loadCache.data != nil && time.Since(hs.loadCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.loadCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get load average
	avg, err := load.Avg()
	if err != nil {
		return nil, err
	}

	// Get misc stats
	misc, err := load.Misc()
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"avg":  avg,
		"misc": misc,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.loadCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// GetProcessInfo returns process information with caching
func (hs *HostStats) GetProcessInfo() (interface{}, error) {
	hs.mutex.RLock()
	if hs.processCache.data != nil && time.Since(hs.processCache.timestamp) < hs.cacheValidity {
		defer hs.mutex.RUnlock()
		return hs.processCache.data, nil
	}
	hs.mutex.RUnlock()

	// Get process list
	pids, err := process.Pids()
	if err != nil {
		return nil, err
	}

	// Get basic info for top processes (limit to 10 to avoid overwhelming)
	processCount := len(pids)
	topProcesses := make([]map[string]interface{}, 0, 10)

	// Only get details for the first 10 processes to avoid performance issues
	limit := 10
	if processCount < limit {
		limit = processCount
	}

	for i := 0; i < limit; i++ {
		p, err := process.NewProcess(pids[i])
		if err != nil {
			continue
		}

		name, _ := p.Name()
		cmdline, _ := p.Cmdline()
		cpuPercent, _ := p.CPUPercent()
		memPercent, _ := p.MemoryPercent()

		procInfo := map[string]interface{}{
			"pid":         pids[i],
			"name":        name,
			"cmdline":     cmdline,
			"cpu_percent": cpuPercent,
			"mem_percent": memPercent,
		}

		topProcesses = append(topProcesses, procInfo)
	}

	result := map[string]interface{}{
		"count":         processCount,
		"top_processes": topProcesses,
	}

	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.processCache = StatCache{
		data:      result,
		timestamp: time.Now(),
	}
	return result, nil
}

// Global instance of HostStats with 30-second cache validity
var hostStats = NewHostStats(30 * time.Second)

func CreateStatisticsHandler(db database.DatabaseConnection) func(*gin.Context) {
	return func(c *gin.Context) {
		stats := make(map[string]interface{})

		// Web stats
		stats["web"] = Stats.Data()

		// Database stats
		stats["db"] = db.Stats()

		// CPU stats
		cpuStats, err := hostStats.GetCPUInfo()
		if err == nil {
			stats["cpu"] = cpuStats
		} else {
			stats["cpu"] = map[string]string{"error": err.Error()}
		}

		// Memory stats
		memStats, err := hostStats.GetMemInfo()
		if err == nil {
			stats["memory"] = memStats
		} else {
			stats["memory"] = map[string]string{"error": err.Error()}
		}

		// Disk stats
		diskStats, err := hostStats.GetDiskInfo()
		if err == nil {
			stats["disk"] = diskStats
		} else {
			stats["disk"] = map[string]string{"error": err.Error()}
		}

		// Network stats
		//netStats, err := hostStats.GetNetInfo()
		//if err == nil {
		//	stats["network"] = netStats
		//} else {
		//	stats["network"] = map[string]string{"error": err.Error()}
		//}

		// Host stats
		hostInfo, err := hostStats.GetHostInfo()
		if err == nil {
			stats["host"] = hostInfo
		} else {
			stats["host"] = map[string]string{"error": err.Error()}
		}

		// Load stats
		loadStats, err := hostStats.GetLoadInfo()
		if err == nil {
			stats["load"] = loadStats
		} else {
			stats["load"] = map[string]string{"error": err.Error()}
		}

		// Process stats
		processStats, err := hostStats.GetProcessInfo()
		if err == nil {
			stats["process"] = processStats
		} else {
			stats["process"] = map[string]string{"error": err.Error()}
		}

		c.JSON(http.StatusOK, stats)
	}
}
