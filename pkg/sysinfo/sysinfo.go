package sysinfo

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

type timestampJSON struct {
	time.Time
}

func (t timestampJSON) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(t.Unix(), 10)), nil
}

func (t *timestampJSON) UnmarshalJSON(data []byte) error {
	i, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}

	t.Time = time.Unix(i, 0)
	return nil
}

type SystemInfo struct {
	HostInfoIsAvailable bool          `json:"host_info_is_available"`
	BootTime            timestampJSON `json:"boot_time"`
	Hostname            string        `json:"hostname"`
	Platform            string        `json:"platform"`

	CPU struct {
		LoadIsAvailable bool  `json:"load_is_available"`
		Load1Percent    uint8 `json:"load1_percent"`
		Load15Percent   uint8 `json:"load15_percent"`

		TemperatureIsAvailable bool  `json:"temperature_is_available"`
		TemperatureC           uint8 `json:"temperature_c"`
	} `json:"cpu"`

	Memory struct {
		IsAvailable bool   `json:"memory_is_available"`
		TotalMB     uint64 `json:"total_mb"`
		UsedMB      uint64 `json:"used_mb"`
		UsedPercent uint8  `json:"used_percent"`

		SwapIsAvailable bool   `json:"swap_is_available"`
		SwapTotalMB     uint64 `json:"swap_total_mb"`
		SwapUsedMB      uint64 `json:"swap_used_mb"`
		SwapUsedPercent uint8  `json:"swap_used_percent"`
	} `json:"memory"`

	Mountpoints []MountpointInfo `json:"mountpoints"`
}

type MountpointInfo struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	TotalMB     uint64 `json:"total_mb"`
	UsedMB      uint64 `json:"used_mb"`
	UsedPercent uint8  `json:"used_percent"`
}

type SystemInfoRequest struct {
	CPUTempSensor            string                       `yaml:"cpu-temp-sensor"`
	HideMountpointsByDefault bool                         `yaml:"hide-mountpoints-by-default"`
	Mountpoints              map[string]MointpointRequest `yaml:"mountpoints"`
}

type MointpointRequest struct {
	Name string `yaml:"name"`
	Hide *bool  `yaml:"hide"`
}

// Currently caches hostname indefinitely which isn't ideal
// Potential issue with caching boot time as it may not initially get reported correctly:
// https://github.com/shirou/gopsutil/issues/842#issuecomment-1908972344
type cacheableHostInfo struct {
	available bool
	hostname  string
	platform  string
	bootTime  timestampJSON
}

var cachedHostInfo cacheableHostInfo

func getHostInfo() (cacheableHostInfo, error) {
	var err error
	info := cacheableHostInfo{}

	info.hostname, err = os.Hostname()
	if err != nil {
		return info, err
	}

	info.platform, _, _, err = host.PlatformInformation()
	if err != nil {
		return info, err
	}

	bootTime, err := host.BootTime()
	if err != nil {
		return info, err
	}

	info.bootTime = timestampJSON{time.Unix(int64(bootTime), 0)}
	info.available = true

	return info, nil
}

func Collect(req *SystemInfoRequest) (*SystemInfo, []error) {
	if req == nil {
		req = &SystemInfoRequest{}
	}

	var errs []error

	addErr := func(err error) {
		errs = append(errs, err)
	}

	info := &SystemInfo{
		Mountpoints: []MountpointInfo{},
	}

	applyCachedHostInfo := func() {
		info.HostInfoIsAvailable = true
		info.BootTime = cachedHostInfo.bootTime
		info.Hostname = cachedHostInfo.hostname
		info.Platform = cachedHostInfo.platform
	}

	if cachedHostInfo.available {
		applyCachedHostInfo()
	} else {
		hostInfo, err := getHostInfo()
		if err == nil {
			cachedHostInfo = hostInfo
			applyCachedHostInfo()
		} else {
			addErr(fmt.Errorf("getting host info: %v", err))
		}
	}

	coreCount, err := cpu.Counts(true)
	if err == nil {
		loadAvg, err := load.Avg()
		if err == nil {
			info.CPU.LoadIsAvailable = true
			if runtime.GOOS == "windows" {
				// The numbers returned here seem unreliable on Windows. Even with the CPU pegged
				// at close to 50% for multiple minutes, load1 is sometimes way under or way over
				// with no clear pattern. Dividing by core count gives numbers that are way too
				// low so that's likely not necessary as it is with unix.
				info.CPU.Load1Percent = uint8(math.Min(loadAvg.Load1*100, 100))
				info.CPU.Load15Percent = uint8(math.Min(loadAvg.Load15*100, 100))
			} else {
				info.CPU.Load1Percent = uint8(math.Min((loadAvg.Load1/float64(coreCount))*100, 100))
				info.CPU.Load15Percent = uint8(math.Min((loadAvg.Load15/float64(coreCount))*100, 100))
			}
		} else {
			addErr(fmt.Errorf("getting load avg: %v", err))
		}
	} else {
		addErr(fmt.Errorf("getting core count: %v", err))
	}

	memory, err := mem.VirtualMemory()
	if err == nil {
		info.Memory.IsAvailable = true
		info.Memory.TotalMB = memory.Total / 1024 / 1024
		info.Memory.UsedMB = memory.Used / 1024 / 1024
		info.Memory.UsedPercent = uint8(math.Min(memory.UsedPercent, 100))
	} else {
		addErr(fmt.Errorf("getting memory info: %v", err))
	}

	swapMemory, err := mem.SwapMemory()
	if err == nil {
		info.Memory.SwapIsAvailable = true
		info.Memory.SwapTotalMB = swapMemory.Total / 1024 / 1024
		info.Memory.SwapUsedMB = swapMemory.Used / 1024 / 1024
		info.Memory.SwapUsedPercent = uint8(math.Min(swapMemory.UsedPercent, 100))
	} else {
		addErr(fmt.Errorf("getting swap memory info: %v", err))
	}

	// currently disabled on Windows because it requires elevated privilidges, otherwise
	// keeps returning a single sensor with key "ACPI\\ThermalZone\\TZ00_0" which
	// doesn't seem to be the CPU sensor or correspond to anything useful when
	// compared against the temperatures Libre Hardware Monitor reports.
	// Also disabled on the bsd's because it's not implemented by go-psutil for them
	if runtime.GOOS != "windows" && runtime.GOOS != "openbsd" && runtime.GOOS != "netbsd" && runtime.GOOS != "freebsd" {
		sensorReadings, err := sensors.SensorsTemperatures()
		_, errIsWarning := err.(*sensors.Warnings)
		if err == nil || errIsWarning {
			if req.CPUTempSensor != "" {
				for i := range sensorReadings {
					if sensorReadings[i].SensorKey == req.CPUTempSensor {
						info.CPU.TemperatureIsAvailable = true
						info.CPU.TemperatureC = uint8(sensorReadings[i].Temperature)
						break
					}
				}

				if !info.CPU.TemperatureIsAvailable {
					addErr(fmt.Errorf("CPU temperature sensor %s not found", req.CPUTempSensor))
				}
			} else if cpuTempSensor := inferCPUTempSensor(sensorReadings); cpuTempSensor != nil {
				info.CPU.TemperatureIsAvailable = true
				info.CPU.TemperatureC = uint8(cpuTempSensor.Temperature)
			}
		} else {
			addErr(fmt.Errorf("getting sensor readings: %v", err))
		}
	}

	addedMountpoints := map[string]struct{}{}
	addMountpointInfo := func(requestedPath string, mpReq MointpointRequest) {
		if _, exists := addedMountpoints[requestedPath]; exists {
			return
		}

		isHidden := req.HideMountpointsByDefault
		if mpReq.Hide != nil {
			isHidden = *mpReq.Hide
		}
		if isHidden {
			return
		}

		usage, err := disk.Usage(requestedPath)
		if err == nil {
			mpInfo := MountpointInfo{
				Path:        requestedPath,
				Name:        mpReq.Name,
				TotalMB:     usage.Total / 1024 / 1024,
				UsedMB:      usage.Used / 1024 / 1024,
				UsedPercent: uint8(math.Min(usage.UsedPercent, 100)),
			}

			info.Mountpoints = append(info.Mountpoints, mpInfo)
			addedMountpoints[requestedPath] = struct{}{}
		} else {
			addErr(fmt.Errorf("getting filesystem usage for %s: %v", requestedPath, err))
		}
	}

	if !req.HideMountpointsByDefault {
		// disk.Partitions(false) asks gopsutil for "physical" filesystems only, but its
		// notion of physical excludes "overlay" -- the storage driver Docker uses for a
		// container's own root filesystem by default. In a plain container with no extra
		// host mounts, that leaves auto-detection with nothing to report except whatever
		// tiny /etc/* config-file bind mounts Docker injects (which get hidden by callers
		// like glanceapp/agent's default container mountpoint list anyway), so the net
		// result is an empty mountpoints list even though there's a real, meaningful disk
		// behind the overlay root worth reporting on. disk.Partitions(true) plus an
		// allowlist of genuinely disk-backed filesystem types (keeping "overlay") reports
		// that root correctly while still excluding proc/sysfs/tmpfs/etc noise.
		filesystems, err := disk.Partitions(true)
		if err == nil {
			for _, fs := range filesystems {
				if !isPhysicalFilesystemType(fs.Fstype) {
					continue
				}
				addMountpointInfo(fs.Mountpoint, req.Mountpoints[fs.Mountpoint])
			}
		} else {
			addErr(fmt.Errorf("getting filesystems: %v", err))
		}
	}

	for mountpoint, mpReq := range req.Mountpoints {
		addMountpointInfo(mountpoint, mpReq)
	}

	sort.Slice(info.Mountpoints, func(a, b int) bool {
		return info.Mountpoints[a].UsedPercent > info.Mountpoints[b].UsedPercent
	})

	return info, errs
}

// physicalFilesystemTypes lists the filesystem types treated as real, disk-backed storage
// worth reporting usage for by default -- an allowlist rather than a denylist of virtual
// types (proc, sysfs, tmpfs, devpts, mqueue, cgroup, cgroup2, ...) since the set of virtual
// filesystem types grows over time while real disk filesystem types are comparatively stable.
// "overlay" is deliberately included: it's technically a union filesystem rather than a disk
// filesystem, but it's what Docker uses for a container's own root by default, and reports
// accurate usage of the underlying host disk -- excluding it is what left auto-detection
// finding nothing to report in a plain container with no extra host mounts.
var physicalFilesystemTypes = map[string]bool{
	"overlay":  true,
	"ext2":     true,
	"ext3":     true,
	"ext4":     true,
	"xfs":      true,
	"btrfs":    true,
	"zfs":      true,
	"ntfs":     true,
	"vfat":     true,
	"exfat":    true,
	"hfs":      true,
	"hfsplus":  true,
	"apfs":     true,
	"f2fs":     true,
	"jfs":      true,
	"reiserfs": true,
	"udf":      true,
}

func isPhysicalFilesystemType(fstype string) bool {
	return physicalFilesystemTypes[fstype]
}

func inferCPUTempSensor(sensors []sensors.TemperatureStat) *sensors.TemperatureStat {
	for i := range sensors {
		switch sensors[i].SensorKey {
		case
			"coretemp_package_id_0", // intel / linux
			"coretemp",              // intel / linux
			"k10temp",               // amd / linux
			"zenpower",              // amd / linux
			"cpu_thermal":           // raspberry pi / linux
			return &sensors[i]
		}
	}

	return nil
}
