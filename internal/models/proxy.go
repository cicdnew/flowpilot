package models

import "time"

// ProxyStatus indicates the health of a proxy.
type ProxyStatus string

const (
	ProxyStatusHealthy   ProxyStatus = "healthy"
	ProxyStatusUnhealthy ProxyStatus = "unhealthy"
	ProxyStatusUnknown   ProxyStatus = "unknown"
)

// ProxyProtocol is the supported proxy type.
type ProxyProtocol string

const (
	ProxyHTTP   ProxyProtocol = "http"
	ProxyHTTPS  ProxyProtocol = "https"
	ProxySOCKS5 ProxyProtocol = "socks5"
)

// Proxy represents a proxy server in the pool.
type Proxy struct {
	ID                   string        `json:"id"`
	Server               string        `json:"server"` // host:port
	Protocol             ProxyProtocol `json:"protocol"`
	Username             string        `json:"username,omitempty"`
	Password             string        `json:"password,omitempty"`
	Geo                  string        `json:"geo,omitempty"` // country code
	Status               ProxyStatus   `json:"status"`
	Latency              int           `json:"latency"` // ms, last measured
	SuccessRate          float64       `json:"successRate"`
	TotalUsed            int           `json:"totalUsed"`
	MaxRequestsPerMinute int           `json:"maxRequestsPerMinute,omitempty"`
	LastChecked          *time.Time    `json:"lastChecked,omitempty"`
	CreatedAt            time.Time     `json:"createdAt"`
	LocalEndpoint        string        `json:"localEndpoint,omitempty"`
	LocalEndpointOn      bool          `json:"localEndpointOn,omitempty"`
	LocalAuthEnabled     bool          `json:"localAuthEnabled,omitempty"`
	ActiveLocalUsers     int           `json:"activeLocalUsers,omitempty"`
}

// ProxyCountryStats summarizes proxy capacity and pressure for one country pool.
type ProxyCountryStats struct {
	Country              string `json:"country"`
	Total                int    `json:"total"`
	Healthy              int    `json:"healthy"`
	ActiveReservations   int    `json:"activeReservations"`
	TotalUsed            int    `json:"totalUsed"`
	FallbackAssignments  int    `json:"fallbackAssignments"`
	ActiveLocalEndpoints int    `json:"activeLocalEndpoints"`
}

// ProxyRoutingPreset stores a reusable routing profile.
type ProxyRoutingPreset struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	RandomByCountry bool                 `json:"randomByCountry"`
	Country         string               `json:"country,omitempty"`
	Fallback        ProxyRoutingFallback `json:"fallback,omitempty"`
	CreatedAt       time.Time            `json:"createdAt"`
}

// LocalProxyGatewayStats summarizes runtime local gateway health.
type LocalProxyGatewayStats struct {
	ActiveEndpoints   int    `json:"activeEndpoints"`
	EndpointCreations int64  `json:"endpointCreations"`
	EndpointReuses    int64  `json:"endpointReuses"`
	AuthFailures      int64  `json:"authFailures"`
	UpstreamFailures  int64  `json:"upstreamFailures"`
	LastError         string `json:"lastError,omitempty"`
}

// RotationStrategy controls how proxies are selected.
type RotationStrategy string

const (
	RotationRoundRobin    RotationStrategy = "round_robin"
	RotationRandom        RotationStrategy = "random"
	RotationLeastUsed     RotationStrategy = "least_used"
	RotationLowestLatency RotationStrategy = "lowest_latency"
)

// ToProxyConfig converts a pool Proxy to a task-level ProxyConfig.
func (p *Proxy) ToProxyConfig() ProxyConfig {
	return ProxyConfig{
		Server:   p.Server,
		Protocol: p.Protocol,
		Username: p.Username,
		Password: p.Password,
		Geo:      p.Geo,
	}
}

// ProxyPoolConfig configures the proxy pool behavior.
type ProxyPoolConfig struct {
	Strategy            RotationStrategy `json:"strategy"`
	HealthCheckInterval int              `json:"healthCheckInterval"` // seconds
	MaxFailures         int              `json:"maxFailures"`
	HealthCheckURL      string           `json:"healthCheckUrl"`
}
