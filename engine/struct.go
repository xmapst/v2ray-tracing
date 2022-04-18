package engine

type Output struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

const (
	OutputTypeAccess  = "access"
	OutputTypeConsole = "console"
	OutputTypeStats   = "stats"
	OutputTypeTraffic = "traffic"
)

type Access struct {
	Timestamp   int64  `json:"timestamp"`
	Src         string `json:"src"`
	SrcPort     int64  `json:"src_port"`
	SrcProtocol string `json:"src_protocol"`
	Dst         string `json:"dst"`
	DstPort     int64  `json:"dst_port"`
	DstProtocol string `json:"dst_protocol"`
	Outbound    string `json:"outbound"`
}

type Console struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
}

type Stats struct {
	Timestamp    int64  `json:"timestamp"`
	Uptime       uint32 `json:"up_time"`
	Sys          uint64 `json:"sys"`
	NumGoroutine uint32 `json:"num_goroutine"`
	Alloc        uint64 `json:"alloc"`
	LiveObjects  uint64 `json:"live_objects"`
	TotalAlloc   uint64 `json:"total_alloc"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	NumGC        uint32 `json:"num_gc"`
	PauseTotalNs uint64 `json:"pause_total_ns"`
}

type Traffic struct {
	Timestamp int64  `json:"timestamp"`
	Bound     string `json:"bound"`
	Name      string `json:"name"`
	//Tag       string `json:"tag"`
	Link  string `json:"link"`
	Value int64  `json:"value"`
}
