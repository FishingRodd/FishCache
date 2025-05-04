package consistent

import "time"

const (
	DefaultServiceName = "fishcache"
)

var Conf *Config

type Config struct {
	Etcd *Etcd
}

type Etcd struct {
	Address     []string
	Timeout     time.Duration
	ServiceName string
}
