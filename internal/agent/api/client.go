package api

// User represents a node user entry used in legacy template sync.
type User struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  *int64 `json:"speed_limit"`
	DeviceLimit *int   `json:"device_limit"`
}

// TrafficPayload carries per-user traffic deltas.
type TrafficPayload struct {
	UserID   int64  `json:"user_id"`
	UID      string `json:"uid,omitempty"`
	Upload   int64  `json:"u"`
	Download int64  `json:"d"`
}

// StatusPayload carries node-level metrics.
type StatusPayload struct {
	CPU             float64 `json:"cpu"`
	Mem             Stats   `json:"mem"`
	Swap            Stats   `json:"swap"`
	Disk            Stats   `json:"disk"`
	Uptime          uint64  `json:"uptime"`
	Load1           float64 `json:"load1"`
	Load5           float64 `json:"load5"`
	Load15          float64 `json:"load15"`
	TcpCount        int     `json:"tcp_count"`
	UdpCount        int     `json:"udp_count"`
	ProcessCount    int     `json:"process_count"`
	NetIO           NetIO   `json:"net_io"`
	TrafficUpload   uint64  `json:"traffic_upload"`
	TrafficDownload uint64  `json:"traffic_download"`
}

// Stats holds total/used values.
type Stats struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

// NetIO carries network io rates.
type NetIO struct {
	Up   uint64 `json:"up"`
	Down uint64 `json:"down"`
}
