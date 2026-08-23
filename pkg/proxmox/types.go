package proxmox

type singleResponse[T any] struct {
	Data T `json:"data"`
}

type multipleResponse[T any] struct {
	Data []T `json:"data"`
}

type ClusterResource struct {
	NetOut     int64   `json:"netout"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	NetIn      int64   `json:"netin"`
	MaxCPU     int     `json:"maxcpu"`
	DiskWrite  int64   `json:"diskwrite"`
	Template   int     `json:"template"`
	CPU        float64 `json:"cpu"`
	Uptime     int64   `json:"uptime"`
	Mem        int64   `json:"mem"`
	DiskRead   int64   `json:"diskread"`
	MaxMem     int64   `json:"maxmem"`
	MaxDisk    uint64  `json:"maxdisk"`
	ID         string  `json:"id"`
	VMID       int     `json:"vmid"`
	Type       string  `json:"type"`
	Node       string  `json:"node"`
	Disk       uint64  `json:"disk"`
	Level      string  `json:"level"`
	CgroupMode int     `json:"cgroup-mode"`
	PluginType string  `json:"plugintype"`
	Content    string  `json:"content"`
	Shared     int     `json:"shared"`
	Storage    string  `json:"storage"`
	SDN        string  `json:"sdn"`
}

type NodeStatus struct {
	PveVersion string   `json:"pveversion"`
	Kversion   string   `json:"kversion"`
	Wait       float64  `json:"wait"`
	Uptime     int64    `json:"uptime"`
	LoadAvg    []string `json:"loadavg"`
	Cpu        float64  `json:"cpu"`
	Idle       int      `json:"idle"`

	Swap struct {
		Free  uint64 `json:"free"`
		Used  uint64 `json:"used"`
		Total uint64 `json:"total"`
	} `json:"swap"`

	CpuInfo struct {
		Model   string `json:"model"`
		Cpus    int    `json:"cpus"`
		Hvm     string `json:"hvm"`
		UserHz  int    `json:"user_hz"`
		Flags   string `json:"flags"`
		Cores   int    `json:"cores"`
		Sockets int    `json:"sockets"`
		Mhz     string `json:"mhz"`
	} `json:"cpuinfo"`

	Memory struct {
		Free  uint64 `json:"free"`
		Used  uint64 `json:"used"`
		Total uint64 `json:"total"`
	} `json:"memory"`

	RootFs struct {
		Available int64 `json:"avail"`
		Total     int64 `json:"total"`
		Used      int64 `json:"used"`
		Free      int64 `json:"free"`
	} `json:"rootfs"`
}
