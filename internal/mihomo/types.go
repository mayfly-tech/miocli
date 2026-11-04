package mihomo

import "context"

type Version struct {
	Meta    bool   `json:"meta"`
	Version string `json:"version"`
}

type Proxy struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Now     string         `json:"now"`
	All     []string       `json:"all"`
	History []DelayHistory `json:"history"`
}

type SubscriptionInfo struct {
	Upload   *uint64 `json:"Upload"`
	Download *uint64 `json:"Download"`
	Total    *uint64 `json:"Total"`
	Expire   *int64  `json:"Expire"`
}

type Provider struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	VehicleType      string            `json:"vehicleType"`
	UpdatedAt        string            `json:"updatedAt"`
	Proxies          []Proxy           `json:"proxies"`
	SubscriptionInfo *SubscriptionInfo `json:"subscriptionInfo"`
}

type ProviderClient interface {
	Providers(context.Context) (map[string]Provider, error)
	UpdateProvider(context.Context, string) error
}

type ProviderDelayClient interface {
	DelayProvider(context.Context, string, string, string, int) (int, error)
}

type Connection struct {
	Chains []string `json:"chains"`
}

type ConnectionClient interface {
	Connections(context.Context) ([]Connection, error)
}

type DelayHistory struct {
	Delay int    `json:"delay"`
	Time  string `json:"time"`
}

type Proxies struct {
	Items map[string]Proxy
}

type Mode string

const (
	ModeRule   Mode = "rule"
	ModeGlobal Mode = "global"
	ModeDirect Mode = "direct"
)

type Client interface {
	Version(context.Context) (Version, error)
	Proxies(context.Context) (Proxies, error)
	Mode(context.Context) (Mode, error)
	Select(context.Context, string, string) error
	SetMode(context.Context, Mode) error
	Delay(context.Context, string, string, int) (int, error)
}

type ErrorKind string

const (
	ErrorNetwork  ErrorKind = "network"
	ErrorAuth     ErrorKind = "authentication"
	ErrorNotFound ErrorKind = "not_found"
	ErrorAPI      ErrorKind = "api"
	ErrorDecode   ErrorKind = "decode"
	ErrorTimeout  ErrorKind = "timeout"
)

type APIError struct {
	Kind   ErrorKind
	Status int
	Op     string
	Err    error
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return e.Op + ": Mihomo API returned HTTP " + httpStatusText(e.Status)
	}
	if e.Err != nil {
		return e.Op + ": " + e.Err.Error()
	}
	return e.Op + ": Mihomo API request failed"
}

func (e *APIError) Unwrap() error { return e.Err }
