package bungie

import (
	"sync"
	"time"

	"raidhub/lib/utils"
	"raidhub/lib/utils/logging"
)

var (
	globalMonitor       *globalAPIMonitor
	globalMonitorLock   sync.Once
	globalMonitorLogger logging.Logger
)

// globalAPIMonitor manages a single polling goroutine for all systems
type globalAPIMonitor struct {
	systems   map[string]*systemMonitor
	mu        sync.RWMutex
	stopChan  chan struct{}
	updateChan chan string // Channel to signal immediate system updates
}

// systemMonitor tracks availability for a specific system
type systemMonitor struct {
	systemName  string
	monitorWG   sync.WaitGroup
	available   bool
	mu          sync.RWMutex
}

// getGlobalMonitor returns the singleton global monitor
func getGlobalMonitor() *globalAPIMonitor {
	globalMonitorLock.Do(func() {
		globalMonitorLogger = logging.NewLogger("BUNGIE_API_MONITOR")
		globalMonitor = &globalAPIMonitor{
			systems:    make(map[string]*systemMonitor),
			stopChan:   make(chan struct{}),
			updateChan: make(chan string, 100), // Buffered channel for immediate signals
		}
		go globalMonitor.monitor()
	})
	return globalMonitor
}

// GetAPIAvailabilityMonitor returns a monitor for the given system name
func GetAPIAvailabilityMonitor(systemName string) *APIAvailabilityMonitor {
	return getGlobalMonitor().getSystemMonitor(systemName)
}

// APIAvailabilityMonitor is a wrapper around systemMonitor for backwards compatibility
type APIAvailabilityMonitor struct {
	system *systemMonitor
}

// getSystemMonitor returns or creates a system monitor
func (gm *globalAPIMonitor) getSystemMonitor(systemName string) *APIAvailabilityMonitor {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	
	if monitor, exists := gm.systems[systemName]; exists {
		return &APIAvailabilityMonitor{system: monitor}
	}
	
	// Create new system monitor
	monitor := &systemMonitor{
		systemName: systemName,
		available:  false, // Start as disabled
	}
	// Start with workers blocked
	monitor.monitorWG.Add(1)
	
	gm.systems[systemName] = monitor
	return &APIAvailabilityMonitor{system: monitor}
}

// monitor polls the Bungie settings API and updates all system monitors
func (gm *globalAPIMonitor) monitor() {
	// Check immediately on start
	gm.updateAllSystems()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gm.updateAllSystems()
		case systemName := <-gm.updateChan:
			// Immediate block requested for specific system (no API call needed)
			gm.blockSystemImmediately(systemName)
		case <-gm.stopChan:
			return
		}
	}
}

// updateAllSystems checks API availability for all systems from a single settings call
func (gm *globalAPIMonitor) updateAllSystems() {
	result, err := Client.GetCommonSettings()
	if err != nil {
		globalMonitorLogger.Error("BUNGIE_SETTINGS_CHECK_ERROR", map[string]any{
			logging.ERROR: err.Error(),
		})
		return
	}
	if !result.Success || result.Data == nil {
		globalMonitorLogger.Error("FAILED_TO_GET_BUNGIE_SETTINGS", map[string]any{
			"success": result.Success,
		})
		return
	}
	
	// Update all registered systems
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	
	for _, monitor := range gm.systems {
		gm.updateSystemState(monitor, result.Data.Systems)
	}
}

// blockSystemImmediately blocks workers for a system without making an API call
// This is called when a SystemDisabled error is detected from the API
func (gm *globalAPIMonitor) blockSystemImmediately(systemName string) {
	gm.mu.RLock()
	monitor, exists := gm.systems[systemName]
	gm.mu.RUnlock()
	
	if !exists {
		return
	}
	
	monitor.mu.Lock()
	wasAvailable := monitor.available
	monitor.available = false // Block the system
	monitor.mu.Unlock()
	
	// Only update wait group if transitioning from available to disabled
	if wasAvailable {
		globalMonitorLogger.Info("BUNGIE_API_DISABLED", map[string]any{
			"action": "blocking_workers",
			"system": monitor.systemName,
			"source": "system_disabled_signal",
		})
		monitor.monitorWG.Add(1)
	}
}

// updateSystemState updates a single system's state
func (gm *globalAPIMonitor) updateSystemState(monitor *systemMonitor, systems map[string]CoreSystem) {
	isAPIDisabled := true
	if system, exists := systems[monitor.systemName]; exists {
		isAPIDisabled = !system.Enabled
	}
	
	monitor.mu.Lock()
	wasAvailable := monitor.available
	monitor.available = !isAPIDisabled
	monitor.mu.Unlock()

	// If API status changed, update the wait group
	if wasAvailable && isAPIDisabled {
		globalMonitorLogger.Info("BUNGIE_API_DISABLED", map[string]any{
			"action": "blocking_workers",
			"system": monitor.systemName,
		})
		monitor.monitorWG.Add(1)
	} else if !wasAvailable && !isAPIDisabled {
		globalMonitorLogger.Info("BUNGIE_API_ENABLED", map[string]any{
			"action": "unblocking_workers",
			"system": monitor.systemName,
		})
		monitor.monitorWG.Done()
	}
}

// GetReadOnlyWaitGroup returns a read-only wrapper of the internal wait group
func (m *APIAvailabilityMonitor) GetReadOnlyWaitGroup() *utils.ReadOnlyWaitGroup {
	return utils.NewReadOnlyWaitGroup(&m.system.monitorWG)
}

// IsRunning returns whether the global monitor is currently running
func (m *APIAvailabilityMonitor) IsRunning() bool {
	return globalMonitor != nil
}

// SignalSystemDisabled triggers an immediate check for system disabled status
// This should be called when a worker receives a SystemDisabled error from Bungie API
func SignalSystemDisabled(systemName string) {
	if globalMonitor != nil && globalMonitor.updateChan != nil {
		select {
		case globalMonitor.updateChan <- systemName:
		default:
			// Channel full, skip - will be caught on next poll
		}
	}
}

// Stop stops all monitors
func StopAllMonitors() {
	if globalMonitor != nil {
		close(globalMonitor.stopChan)
	}
}

// GetCompositeAPIAvailabilityMonitor returns a composite read-only wait group for multiple systems
// Workers will block until ALL systems are available
func GetCompositeAPIAvailabilityMonitor(systemNames []string) *utils.ReadOnlyWaitGroup {
	if len(systemNames) == 0 {
		return nil
	}
	
	if len(systemNames) == 1 {
		monitor := GetAPIAvailabilityMonitor(systemNames[0])
		return monitor.GetReadOnlyWaitGroup()
	}
	
	// Get monitors for each system
	var wgs []*utils.ReadOnlyWaitGroup
	for _, systemName := range systemNames {
		monitor := GetAPIAvailabilityMonitor(systemName)
		wgs = append(wgs, monitor.GetReadOnlyWaitGroup())
	}
	
	// Create a composite wait group
	return utils.NewReadOnlyWaitGroupMulti(wgs)
}

