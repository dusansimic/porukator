package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Logging  LoggingConfig
	App      AppConfig
}

type HTTPConfig struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string `mapstructure:"addr"`
	// PublicHost is the externally reachable base URL advertised to client
	// devices (embedded in the CreateClient response / QR code), e.g.
	// "https://porukator.example.org".
	PublicHost string `mapstructure:"public_host"`
}

type PostgresConfig struct {
	URL string `mapstructure:"url"`
}

type LoggingConfig struct {
	Level  string
	Format string
}

type AppConfig struct {
	Env string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("PORUKATOR")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Bind every nested key explicitly. AutomaticEnv only looks up keys viper
	// already knows about; nested struct fields without defaults don't
	// register unless we bind them.
	for _, k := range []string{
		"http.addr", "http.public_host",
		"postgres.url",
		"logging.level", "logging.format",
		"app.env",
	} {
		_ = viper.BindEnv(k)
	}

	viper.SetDefault("http.addr", ":8080")
	viper.SetDefault("http.public_host", "http://localhost:8080")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("app.env", "production")

	if err := viper.ReadInConfig(); err != nil {
		// Config file is optional; env + defaults are enough for containers.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
