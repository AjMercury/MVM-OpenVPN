package domain

import "time"

type Role string

const (
	Super       Role = "super_admin"
	RouterAdmin Role = "router_admin"
	Operator    Role = "vpn_operator"
	Auditor     Role = "auditor"
	EndUser     Role = "end_user"
)

type Account struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Hash     string   `json:"password_hash,omitempty"`
	Role     Role     `json:"role"`
	Routers  []string `json:"routers"`
}

func (a Account) Can(action, router string) bool {
	if a.Role == Super {
		return true
	}
	if a.Role == EndUser {
		return action == "profile"
	}
	scoped := router == ""
	for _, id := range a.Routers {
		if id == router {
			scoped = true
		}
	}
	if !scoped {
		return false
	}
	if action == "read" {
		return true
	}
	if a.Role == RouterAdmin {
		return action == "router" || action == "vpn"
	}
	return a.Role == Operator && action == "vpn"
}

type Router struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Username     string    `json:"username"`
	Password     string    `json:"password,omitempty"`
	CA           string    `json:"ca_pem,omitempty"`
	Group        string    `json:"group"`
	Status       string    `json:"status"`
	LastSuccess  time.Time `json:"last_success"`
	Observed     Snapshot  `json:"observed"`
	PreviousHash string    `json:"previous_hash"`
	Drift        bool      `json:"drift"`
}
type Snapshot struct {
	Version      string            `json:"version"`
	Architecture string            `json:"architecture"`
	Model        string            `json:"model"`
	Resources    map[string]string `json:"resources"`
	Capabilities map[string]bool   `json:"capabilities"`
	ConfigHash   string            `json:"config_hash"`
	Warnings     []string          `json:"warnings"`
}
type ServerSpec struct {
	RouterID      string   `json:"router_id"`
	Name          string   `json:"name"`
	Endpoint      string   `json:"endpoint"`
	Port          int      `json:"port"`
	Transport     string   `json:"transport"`
	Subnet        string   `json:"subnet"`
	Start         string   `json:"pool_start"`
	End           string   `json:"pool_end"`
	Gateway       string   `json:"gateway"`
	DNS           []string `json:"dns"`
	Allowed       []string `json:"allowed"`
	InputAnchor   string   `json:"input_anchor"`
	ForwardAnchor string   `json:"forward_anchor"`
}
type Server struct {
	ID          string     `json:"id"`
	Spec        ServerSpec `json:"spec"`
	JobID       string     `json:"job_id"`
	CA          string     `json:"ca"`
	Certificate string     `json:"certificate"`
	Pool        string     `json:"pool"`
	Profile     string     `json:"profile"`
	RemoteID    string     `json:"remote_id"`
}
type VPNUser struct {
	ID          string `json:"id"`
	RouterID    string `json:"router_id"`
	ServerID    string `json:"server_id"`
	AccountID   string `json:"account_id"`
	Username    string `json:"username"`
	Address     string `json:"address"`
	Certificate string `json:"certificate"`
	RemoteID    string `json:"remote_id"`
	JobID       string `json:"job_id"`
	Disabled    bool   `json:"disabled"`
}
type Resource struct {
	Path    string            `json:"path"`
	ID      string            `json:"id"`
	Marker  string            `json:"marker"`
	Desired map[string]string `json:"desired"`
}
type Step struct {
	Path   string            `json:"path"`
	Method string            `json:"method"`
	Values map[string]string `json:"values"`
	Secret bool              `json:"secret"`
}
type Job struct {
	ID          string     `json:"id"`
	RouterID    string     `json:"router_id"`
	Actor       string     `json:"actor"`
	Kind        string     `json:"kind"`
	Key         string     `json:"key"`
	Fingerprint string     `json:"fingerprint"`
	State       string     `json:"state"`
	Error       string     `json:"error,omitempty"`
	Created     time.Time  `json:"created"`
	Updated     time.Time  `json:"updated"`
	Steps       []Step     `json:"steps"`
	Owned       []Resource `json:"owned"`
	Server      *Server    `json:"server,omitempty"`
	User        *VPNUser   `json:"user,omitempty"`
	InFlight    bool       `json:"in_flight"`
}
type Audit struct {
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Target string    `json:"target"`
	Result string    `json:"result"`
}
type Session struct {
	AccountID string
	CSRF      string
	Expires   time.Time
}
type Delivery struct {
	ID        string
	UserID    string
	AccountID string
	Expires   time.Time
	Consumed  bool
	Profile   []byte
}
