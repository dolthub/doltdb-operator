package config

const (
	// LogLevel defines the logging level for the application.
	LogLevel = "trace"

	// MaxConnections is the maximum number of connections allowed.
	MaxConnections int32 = 128

	// RemotesAPIPort is the port number for the remotes API.
	RemotesAPIPort int32 = 50051

	// DatabasePort is the port number for the database.
	DatabasePort int32 = 3306
	// DatabasePortName is the name of the port used in the Service.
	DatabasePortName string = "mysql"
)
