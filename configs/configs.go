package configs

import "github.com/spf13/viper"

type Conf struct {
	DBDriver                 string `mapstructure:"DB_DRIVER"`
	DBHost                   string `mapstructure:"DB_HOST"`
	DBPort                   string `mapstructure:"DB_PORT"`
	DBUser                   string `mapstructure:"DB_USER"`
	DBPassword               string `mapstructure:"DB_PASSWORD"`
	DBName                   string `mapstructure:"DB_NAME"`
	RedisHost                string `mapstructure:"REDIS_HOST"`
	RedisPort                string `mapstructure:"REDIS_PORT"`
	WebServerPort            string `mapstructure:"WEB_SERVER_PORT"`
	GRPCPort                 string `mapstructure:"GRPC_PORT"`
	AMQPort                  string `mapstructure:"AMQ_PORT"`
	RabbitMQHost             string `mapstructure:"RABBITMQ_HOST"`
	FleetHost                string `mapstructure:"FLEET_HOST"`
	FleetPort                string `mapstructure:"FLEET_PORT"`
	OtelServiceName          string `mapstructure:"OTEL_SERVICE_NAME"`
	OtelExporterOTLPEndpoint string `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OtelExporterOTLPInsecure string `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
	OtelTracesSampler        string `mapstructure:"OTEL_TRACES_SAMPLER"`
}

func LoadConfig(path string, defaultServiceName string) (*Conf, error) {
	var cfg *Conf

	viper.AddConfigPath(path)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	viper.SetDefault("OTEL_SERVICE_NAME", defaultServiceName)

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
