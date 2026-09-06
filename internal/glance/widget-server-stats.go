package glance

import (
	"context"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glanceapp/glance/pkg/proxmox"
	"github.com/glanceapp/glance/pkg/sysinfo"
)

var serverStatsWidgetTemplate = mustParseTemplate("server-stats.html", "widget-base.html")

type serverStatsWidget struct {
	widgetBase `yaml:",inline"`
	Servers    []serverStatsRequest `yaml:"servers"`
}

type serverStats struct {
	HostInfoIsAvailable bool      `json:"host_info_is_available"`
	BootTime            time.Time `json:"boot_time"`
	Hostname            string    `json:"hostname"`
	Platform            string    `json:"platform"`

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

	Mountpoints []serverStorageInfo `json:"mountpoints"`
}

type serverStorageInfo struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	TotalMB     uint64 `json:"total_mb"`
	UsedMB      uint64 `json:"used_mb"`
	UsedPercent uint8  `json:"used_percent"`
}

func (widget *serverStatsWidget) initialize() error {
	widget.withTitle("Server Stats").withCacheDuration(15 * time.Second)
	widget.widgetBase.WIP = true

	if len(widget.Servers) == 0 {
		widget.Servers = []serverStatsRequest{{Type: "local"}}
	}

	for i := range widget.Servers {
		widget.Servers[i].URL = strings.TrimRight(widget.Servers[i].URL, "/")

		if widget.Servers[i].Timeout == 0 {
			widget.Servers[i].Timeout = durationField(3 * time.Second)
		}
	}

	return nil
}

func (widget *serverStatsWidget) update(context.Context) {
	// Refactor later, most of it may change depending on feedback
	var wg sync.WaitGroup

	for i := range widget.Servers {
		serv := &widget.Servers[i]

		if serv.Type == "local" {
			info, errs := sysinfo.Collect(serv.SystemInfoRequest)

			if len(errs) > 0 {
				for i := range errs {
					slog.Warn("Getting system info: " + errs[i].Error())
				}
			}

			serv.IsReachable = true
			serv.Info = &serverStats{
				HostInfoIsAvailable: info.HostInfoIsAvailable,
				BootTime:            info.BootTime.Time,
				Hostname:            info.Hostname,
				Platform:            info.Platform,
				CPU:                 info.CPU,
				Memory:              info.Memory,
			}

			for _, mountPoint := range info.Mountpoints {
				serv.Info.Mountpoints = append(serv.Info.Mountpoints, serverStorageInfo{
					Path:        mountPoint.Path,
					Name:        mountPoint.Name,
					TotalMB:     mountPoint.TotalMB,
					UsedMB:      mountPoint.UsedMB,
					UsedPercent: mountPoint.UsedPercent,
				})
			}
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()

				if serv.Type == "proxmox" {
					info, err := fetchProxmoxServerInfo(serv)
					if err != nil {
						slog.Warn("Getting remote system info: " + err.Error())
						serv.IsReachable = false
						serv.Info = &serverStats{
							Hostname: "Unnamed server #" + strconv.Itoa(i+1),
						}
					} else {
						serv.IsReachable = true
						serv.Info = info
					}
				} else {
					info, err := fetchRemoteServerInfo(serv)
					if err != nil {
						slog.Warn("Getting remote system info: " + err.Error())
						serv.IsReachable = false
						serv.Info = &serverStats{
							Hostname: "Unnamed server #" + strconv.Itoa(i+1),
						}
					} else {
						serv.IsReachable = true
						serv.Info = &serverStats{
							HostInfoIsAvailable: info.HostInfoIsAvailable,
							BootTime:            info.BootTime.Time,
							Hostname:            info.Hostname,
							Platform:            info.Platform,
							CPU:                 info.CPU,
							Memory:              info.Memory,
						}

						for _, mountPoint := range info.Mountpoints {
							serv.Info.Mountpoints = append(serv.Info.Mountpoints, serverStorageInfo{
								Path:        mountPoint.Path,
								Name:        mountPoint.Name,
								TotalMB:     mountPoint.TotalMB,
								UsedMB:      mountPoint.UsedMB,
								UsedPercent: mountPoint.UsedPercent,
							})
						}
					}
				}
			}()
		}
	}

	wg.Wait()
	widget.withError(nil).scheduleNextUpdate()
}

func (widget *serverStatsWidget) Render() template.HTML {
	return widget.renderTemplate(widget, serverStatsWidgetTemplate)
}

type serverStatsRequest struct {
	*sysinfo.SystemInfoRequest `yaml:",inline"`
	Info                       *serverStats  `yaml:"-"`
	IsReachable                bool          `yaml:"-"`
	StatusText                 string        `yaml:"-"`
	Name                       string        `yaml:"name"`
	HideSwap                   bool          `yaml:"hide-swap"`
	Type                       string        `yaml:"type"`
	URL                        string        `yaml:"url"`
	Token                      string        `yaml:"token"`
	Username                   string        `yaml:"username"`
	Password                   string        `yaml:"password"`
	Timeout                    durationField `yaml:"timeout"`
	// Support for other agents
	// Provider                   string              `yaml:"provider"`
}

func fetchRemoteServerInfo(infoReq *serverStatsRequest) (*sysinfo.SystemInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(infoReq.Timeout))
	defer cancel()

	request, _ := http.NewRequestWithContext(ctx, "GET", infoReq.URL+"/api/sysinfo/all", nil)
	if infoReq.Token != "" {
		request.Header.Set("Authorization", "Bearer "+infoReq.Token)
	}

	info, err := decodeJsonFromRequest[*sysinfo.SystemInfo](defaultHTTPClient, request)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func fetchProxmoxServerInfo(infoReq *serverStatsRequest) (*serverStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(infoReq.Timeout))
	defer cancel()

	cli := proxmox.New(infoReq.URL, infoReq.Username, infoReq.Token, infoReq.Password)
	resources, err := cli.GetClusterResources(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: Add support for multiple nodes
	for _, node := range resources {
		if node.Type != "node" {
			continue
		}

		status, err := cli.GetNodeStatus(ctx, node.Node)
		if err != nil {
			// TODO: Log me!
			continue
		}

		var info serverStats
		info.Platform = status.PveVersion
		info.BootTime = time.Unix(time.Now().Unix()-node.Uptime, 0)
		info.HostInfoIsAvailable = true

		if len(status.LoadAvg) == 3 {
			info.CPU.LoadIsAvailable = true

			load1, _ := strconv.ParseFloat(status.LoadAvg[0], 64)
			info.CPU.Load1Percent = uint8(math.Min(load1*100/float64(status.CpuInfo.Cores), 100))

			load15, _ := strconv.ParseFloat(status.LoadAvg[2], 64)
			info.CPU.Load15Percent = uint8(math.Min(load15*100/float64(status.CpuInfo.Cores), 100))
		}

		info.Memory.IsAvailable = true
		info.Memory.TotalMB = status.Memory.Total / 1024 / 1024
		info.Memory.UsedMB = status.Memory.Used / 1024 / 1024

		if info.Memory.TotalMB > 0 {
			info.Memory.UsedPercent = uint8(math.Min(float64(info.Memory.UsedMB)/float64(info.Memory.TotalMB)*100, 100))
		}

		info.Memory.SwapIsAvailable = true
		info.Memory.SwapTotalMB = status.Swap.Total / 1024 / 1024
		info.Memory.SwapUsedMB = status.Swap.Used / 1024 / 1024

		if info.Memory.SwapTotalMB > 0 {
			info.Memory.SwapUsedPercent = uint8(math.Min(float64(info.Memory.SwapUsedMB)/float64(info.Memory.SwapTotalMB)*100, 100))
		}

		for _, storage := range resources {
			if storage.Type != "storage" || storage.Node != node.Node {
				continue
			}

			storageInfo := serverStorageInfo{
				Path:    storage.ID,
				Name:    storage.Storage,
				TotalMB: storage.MaxDisk / 1024 / 1024,
				UsedMB:  storage.Disk / 1024 / 1024,
			}

			if storageInfo.TotalMB > 0 {
				storageInfo.UsedPercent = uint8(math.Min(float64(storageInfo.UsedMB)/float64(storageInfo.TotalMB)*100, 100))
			}

			info.Mountpoints = append(info.Mountpoints, storageInfo)
		}

		return &info, nil
	}

	return nil, nil
}
