// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

// The user-facing remotes API port must not collide with any other port the server listens on.
// Defaults are spelled out because an omitted parent object is not defaulted.
// +kubebuilder:validation:XValidation:rule="!has(self.remotesAPI) || self.remotesAPI.port != (has(self.listener) && has(self.listener.port) ? self.listener.port : 3306)",message="remotesAPI.port must differ from listener.port"
// +kubebuilder:validation:XValidation:rule="!has(self.remotesAPI) || self.remotesAPI.port != (has(self.cluster) && has(self.cluster.remotesAPI) && has(self.cluster.remotesAPI.port) ? self.cluster.remotesAPI.port : 50051)",message="remotesAPI.port must differ from cluster.remotesAPI.port"
// +kubebuilder:validation:XValidation:rule="!has(self.remotesAPI) || !has(self.mcpServer) || self.remotesAPI.port != (has(self.mcpServer.port) ? self.mcpServer.port : 7007)",message="remotesAPI.port must differ from mcpServer.port"
// +kubebuilder:validation:XValidation:rule="!has(self.remotesAPI) || !has(self.metrics) || !has(self.metrics.enabled) || !self.metrics.enabled || self.remotesAPI.port != (has(self.metrics.port) ? self.metrics.port : 11228)",message="remotesAPI.port must differ from metrics.port"
// +kubebuilder:validation:XValidation:rule="!has(self.remotesAPI) || !has(self.profiler) || !has(self.profiler.enablePProf) || !self.profiler.enablePProf || self.remotesAPI.port != 6060",message="remotesAPI.port must differ from the profiler port 6060"
type Server struct {
	// Profiler defines the profiler configuration for the DoltDB server.
	// +optional
	Profiler Profiler `json:"profiler,omitempty"`
	// Behavior defines the behavior configuration for the DoltDB server.
	// +optional
	Behavior `json:"behavior,omitempty"`
	// LogLevel defines the logging level for the DoltDB server.
	// +kubebuilder:default:="trace"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`
	// Metrics defines the metrics configuration for the DoltDB server.
	// +optional
	Metrics *Metrics `json:"metrics,omitempty"`
	// Listener defines the listener configuration for the DoltDB server.
	// +optional
	Listener Listener `json:"listener,omitempty"`
	// Cluster defines the cluster configuration for the DoltDB server.
	// +optional
	Cluster Cluster `json:"cluster,omitempty"`
	// MCPServer defines the MCP server configuration. When set, an MCP HTTP
	// endpoint is started alongside the SQL server.
	// +optional
	MCPServer *MCPServer `json:"mcpServer,omitempty"`
	// RemotesAPI enables Dolt's user-facing remotes API, which serves clone, fetch
	// and push of the server's databases. When set, it is started alongside the SQL
	// server. It is separate from Cluster.RemotesAPI, which carries replication traffic.
	//
	// The port is exposed on the standalone Service, or on the primary Service when
	// replication is enabled. It is not exposed on the reader Service, because Dolt
	// does not refuse pushes to a standby through this API.
	//
	// Clients authenticate with HTTP Basic credentials of a SQL user. Reads require
	// the CLONE_ADMIN privilege and pushes require SUPER. Credentials are sent in
	// plaintext, so the port should only be reachable over an encrypted network
	// (e.g. a service mesh) and restricted with a NetworkPolicy.
	// +optional
	RemotesAPI *ServerRemotesAPI `json:"remotesAPI,omitempty"`
}

type Profiler struct {
	// Enabled is a flag to enable the profiler.
	// +kubebuilder:default:=false
	EnablePProf bool `json:"enablePProf,omitempty"`
}

// Behavior defines the behavior configuration for the DoltDB server.
type Behavior struct {
	// AutoGCBehavior defines the auto GC behavior for the DoltDB server.
	// +optional
	AutoGCBehavior AutoGCBehavior `json:"autoGCBehavior,omitempty"`
	// EventScheduler sets the server's event scheduler. When empty, Dolt's default
	// applies, which is ON.
	// +kubebuilder:validation:Enum=ON;OFF
	// +optional
	EventScheduler string `json:"eventScheduler,omitempty"`
}

type AutoGCBehavior struct {
	// Enable is a flag to enable the auto GC behavior.
	// +kubebuilder:default:=false
	// +optional
	Enable bool `json:"enabled,omitempty"`
}

// Metrics defines the metrics configuration for the DoltDB server.
type Metrics struct {
	// Enabled is a flag to enable the metrics server.
	// +kubebuilder:default:=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Labels defines the labels for the metrics server.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Host defines the host for the metrics server.
	// +kubebuilder:default:="0.0.0.0"
	// +optional
	Host string `json:"host,omitempty"`
	// Port defines the port for the metrics server.
	// +kubebuilder:default:=11228
	// +optional
	Port int32 `json:"port,omitempty"`
}

// Listener defines the listener configuration for the DoltDB server.
type Listener struct {
	// Host defines the host for the metrics server.
	// +kubebuilder:default:="0.0.0.0"
	// +optional
	Host string `json:"host,omitempty"`
	// Port defines the port for the metrics server.
	// +kubebuilder:default:=3306
	// +optional
	Port int32 `json:"port,omitempty"`
	// MaxConnections specifies the maximum number of connections for DoltDB.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:default:=1024
	// +optional
	MaxConnections int32 `json:"maxConnections,omitempty"`
}

// Cluster defines the cluster configuration for the DoltDB server.
type Cluster struct {
	// RemotesAPI defines the remotes API configuration for the DoltDB server.
	// +optional
	RemotesAPI RemotesAPI `json:"remotesAPI,omitempty"`
}

type RemotesAPI struct {
	// Port defines the port for the remotes API.
	// +kubebuilder:default:=50051
	// +optional
	Port int32 `json:"port,omitempty"`
}

// ServerRemotesAPI defines Dolt's user-facing remotes API.
type ServerRemotesAPI struct {
	// Port defines the port for the remotes API.
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default:=50052
	// +optional
	Port int32 `json:"port,omitempty"`
	// ReadOnly makes the remotes API refuse pushes.
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (r *ServerRemotesAPI) ApplyDefaults() {
	if r.Port == 0 {
		r.Port = 50052
	}
}

// MCPServer defines the MCP server configuration for the DoltDB server.
type MCPServer struct {
	// Port defines the HTTP port for the MCP server.
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=49151
	// +kubebuilder:default:=7007
	// +optional
	Port int32 `json:"port,omitempty"`
	// User defines the SQL user the MCP server uses to connect to Dolt.
	// Defaults to the root user configured for the DoltDB instance.
	// +optional
	User string `json:"user,omitempty"`
	// PasswordSecretKeyRef references a Secret key containing the SQL password
	// for the MCP user. If not set, the root password Secret is used.
	// +optional
	PasswordSecretKeyRef *SecretKeySelector `json:"passwordSecretKeyRef,omitempty"`
	// Database restricts the MCP server to a specific database.
	// If empty, all databases are accessible.
	// +optional
	Database string `json:"database,omitempty"`
}

func (m *MCPServer) ApplyDefaults() {
	if m.Port == 0 {
		m.Port = 7007
	}
}

func (s *Server) ApplyDefaults() {
	if s.LogLevel == "" {
		s.LogLevel = "trace"
	}
	if s.Metrics == nil {
		s.Metrics = &Metrics{}
	}

	s.Metrics.ApplyDefaults()
	s.Listener.ApplyDefaults()
	s.Cluster.ApplyDefaults()
	if s.MCPServer != nil {
		s.MCPServer.ApplyDefaults()
	}
	if s.RemotesAPI != nil {
		s.RemotesAPI.ApplyDefaults()
	}
}

func (m *Metrics) ApplyDefaults() {
	if !m.Enabled {
		m.Enabled = false
	}
	if m.Host == "" {
		m.Host = "0.0.0.0"
	}
	if m.Port == 0 {
		m.Port = 11228
	}
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
}

func (l *Listener) ApplyDefaults() {
	if l.Host == "" {
		l.Host = "0.0.0.0"
	}
	if l.Port == 0 {
		l.Port = 3306
	}
	if l.MaxConnections == 0 {
		l.MaxConnections = 1024
	}
}

func (c *Cluster) ApplyDefaults() {
	if c.RemotesAPI.Port == 0 {
		c.RemotesAPI.Port = 50051
	}
}
