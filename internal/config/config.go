// Package config loads and validates the application settings from environment variables.
//
// [estudo] Este é o primeiro pacote dentro de internal/. Ele é importado como
// "github.com/henriquenogarini/payhub/internal/config" e usado como config.Load().
// Por estar em internal/, nenhum outro módulo Go consegue importá-lo.
package config

// [estudo] "errors" e "fmt" são da stdlib; "github.com/caarlos0/env/v11" é externo.
// goimports separa os dois grupos com uma linha em branco (stdlib primeiro).
import (
	"errors"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// [estudo] Go não tem enum. O padrão é criar um tipo novo baseado em string
// e declarar constantes desse tipo. O compilador não aceita uma variável string
// onde se espera Env sem um cast explícito (Env(s)), mas não impede valores
// fora da lista — por isso Validate() checa o valor.

// Env identifies the environment the application is running in.
type Env string

// Supported environments.
const (
	EnvDev  Env = "dev"
	EnvProd Env = "prod"
)

// minProdJWTSecretLen is the minimum JWT secret length accepted in production.
//
// [estudo] Começa com minúscula: constante privada ao pacote config.
const minProdJWTSecretLen = 32

// [estudo] Struct tags são as strings entre crases depois do tipo do campo.
// São só metadados: o compilador ignora, e a biblioteca env as lê via reflection.
//   env:"HTTP_PORT"      -> nome da variável de ambiente
//   envDefault:"8080"    -> valor usado se a variável não existir
//   required,notEmpty    -> erro se a variável não existir ou estiver vazia
// Em Java seria @Value("${HTTP_PORT:8080}"), mas aqui ninguém injeta nada
// automaticamente: Load() precisa ser chamada explicitamente no main.

// Config holds every setting the application needs. It is loaded once at startup
// and treated as read-only afterwards.
//
// [estudo] Composição: Config CONTÉM outras structs (HTTP, Database...). Não há herança
// em Go. Campos de struct sem tag "env" são percorridos recursivamente pela biblioteca.
type Config struct {
	AppEnv    Env `env:"APP_ENV" envDefault:"dev"`
	HTTP      HTTPConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Worker    WorkerConfig
	RateLimit RateLimitConfig
}

// HTTPConfig holds the HTTP server settings.
type HTTPConfig struct {
	Port int `env:"HTTP_PORT" envDefault:"8080"`
}

// DatabaseConfig holds the Postgres connection settings.
type DatabaseConfig struct {
	URL string `env:"DATABASE_URL,required,notEmpty"`
}

// RedisConfig holds the Redis connection settings.
type RedisConfig struct {
	Addr string `env:"REDIS_ADDR,required,notEmpty"`
}

// JWTConfig holds the settings used to sign and validate access tokens.
type JWTConfig struct {
	Secret string `env:"JWT_SECRET,required,notEmpty"`
	// [estudo] time.Duration é um int64 em nanossegundos. A biblioteca converte
	// strings como "15m", "1h30m" ou "90s" usando time.ParseDuration.
	TTL time.Duration `env:"JWT_TTL" envDefault:"15m"`
}

// WorkerConfig holds the asynchronous event processing settings.
type WorkerConfig struct {
	PoolSize   int `env:"WORKER_POOL_SIZE" envDefault:"5"`
	QueueSize  int `env:"WORKER_QUEUE_SIZE" envDefault:"100"`
	MaxRetries int `env:"WORKER_MAX_RETRIES" envDefault:"3"`
}

// RateLimitConfig holds the per-merchant webhook rate limit settings.
type RateLimitConfig struct {
	PerMinute int `env:"RATE_LIMIT_PER_MINUTE" envDefault:"120"`
}

// Load reads the configuration from the process environment and validates it.
//
// [estudo] Retorno múltiplo: (Config, error). Não existe "throws" em Go; quem chama
// é obrigado a lidar com o segundo valor. Retornamos Config por VALOR (não *Config):
// a struct é copiada, mas como nunca muda depois de carregada, isso é seguro e
// elimina o risco de nil. Em Java todo objeto é referência; em Go você escolhe.
func Load() (Config, error) {
	// [estudo] nil faz a biblioteca usar as variáveis reais do processo (os.Environ).
	return load(nil)
}

// load parses the given environment (or the process environment when nil) into a Config
// and validates it. It exists so tests can provide a fake environment without touching
// global process state.
func load(environ map[string]string) (Config, error) {
	// [estudo] Generics: ParseAsWithOptions[Config] devolve um Config já preenchido.
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environ})
	if err != nil {
		// [estudo] %w "embrulha" o erro original, como o cause de uma exceção Java.
		// Quem receber pode inspecioná-lo com errors.Is / errors.As.
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		// [estudo] Em caso de erro, devolvemos o "zero value" Config{} para que
		// ninguém use acidentalmente uma config pela metade.
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// Validate checks the semantic rules that struct tags cannot express.
// It reports every problem found at once instead of stopping at the first one.
//
// [estudo] Primeiro método do projeto. "(c Config)" é o receiver: o equivalente ao
// "this" do Java, mas com nome explícito e declarado fora da struct.
// Receiver por VALOR (c Config) porque Validate só lê os campos; usaríamos
// ponteiro (c *Config) se o método precisasse alterar a struct ou se ela fosse
// grande demais para copiar a cada chamada.
func (c Config) Validate() error {
	// [estudo] Slice de errors começando vazio (nil). append cresce o slice conforme
	// necessário, parecido com um ArrayList<Exception>.
	var errs []error

	if c.AppEnv != EnvDev && c.AppEnv != EnvProd {
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDev, EnvProd, c.AppEnv))
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		errs = append(errs, fmt.Errorf("HTTP_PORT must be between 1 and 65535, got %d", c.HTTP.Port))
	}
	if c.JWT.TTL <= 0 {
		errs = append(errs, fmt.Errorf("JWT_TTL must be positive, got %s", c.JWT.TTL))
	}
	if c.AppEnv == EnvProd && len(c.JWT.Secret) < minProdJWTSecretLen {
		// [estudo] Nunca colocamos o valor do segredo na mensagem de erro, só o tamanho exigido.
		errs = append(errs, fmt.Errorf("JWT_SECRET must have at least %d characters in prod", minProdJWTSecretLen))
	}
	if c.Worker.PoolSize < 1 {
		errs = append(errs, fmt.Errorf("WORKER_POOL_SIZE must be at least 1, got %d", c.Worker.PoolSize))
	}
	if c.Worker.QueueSize < 1 {
		errs = append(errs, fmt.Errorf("WORKER_QUEUE_SIZE must be at least 1, got %d", c.Worker.QueueSize))
	}
	if c.Worker.MaxRetries < 0 {
		errs = append(errs, fmt.Errorf("WORKER_MAX_RETRIES must not be negative, got %d", c.Worker.MaxRetries))
	}
	if c.RateLimit.PerMinute < 1 {
		errs = append(errs, fmt.Errorf("RATE_LIMIT_PER_MINUTE must be at least 1, got %d", c.RateLimit.PerMinute))
	}

	// [estudo] errors.Join junta todos os erros em um só (mensagens separadas por "\n").
	// Se a lista estiver vazia, devolve nil — ou seja, "tudo válido".
	return errors.Join(errs...)
}
