package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	Username     string        `default:"root"`
	Password     string        `default:"my-secret"`
	Host         string        `default:"localhost"`
	Port         int           `default:"3306"`
	DBName       string        `default:"test_db"`
	MaxOpenConns int           `default:"10"`
	MaxIdleConns int           `default:"10"`
	MaxLifetime  time.Duration `default:"60s"`
}

func ConfigFromEnv() *Config {
	config := &Config{}
	envconfig.MustProcess("mysql", config)
	return config
}

func ConfigFromEnvPrefix(prefix string) *Config {
	config := &Config{}
	envconfig.MustProcess(prefix, config)
	return config
}

func RawMysqlConn(config *Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/", config.Username, config.Password, config.Host, config.Port))
	return db, err
}

func NewMysqlManager(config *Config) (Manager, error) {
	return NewMysqlManagerWithMetrics(config, nil, nil, nil)
}

func NewMysqlManagerWithMetrics(config *Config, gauge *prometheus.GaugeVec, execCounter *prometheus.CounterVec, execHistogram *prometheus.HistogramVec) (Manager, error) {
	if config == nil {
		config = ConfigFromEnv()
	}
	manager, err := newManagerWithMetrics(config, gauge, execCounter, execHistogram)
	if err != nil {
		return nil, err
	}
	err = manager.Ping()
	if err != nil {
		return nil, err
	}
	return manager, nil
}
