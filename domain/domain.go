package domain

// Raporlar için veri yapıları
type TopPair struct {
	IPAddress     string `json:"ip_address"`
	Endpoint      string `json:"endpoint"`
	TotalRequests int    `json:"total_requests"`
}

type TopEndpoint struct {
	Endpoint string `json:"endpoint"`
	Count    int    `json:"count"`
}

// Geçmiş analitiği için veri yapısı
type IPStat struct {
	Endpoint     string `json:"endpoint"`
	StatusCode   int    `json:"status_code"`
	RequestCount int    `json:"request_count"`
}
