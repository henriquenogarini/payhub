// [estudo] Arquivos terminados em _test.go só são compilados pelo "go test".
// Este arquivo está no MESMO pacote (config), então enxerga a função privada load().
package config

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requiredEnv returns the smallest environment that produces a valid configuration.
func requiredEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://payhub:payhub@localhost:5432/payhub?sslmode=disable",
		"REDIS_ADDR":   "localhost:6379",
		"JWT_SECRET":   "dev-secret",
	}
}

// withEnv returns a copy of requiredEnv with the given overrides applied.
// An override with an empty key value is kept as an empty string (variable set but blank).
func withEnv(overrides map[string]string) map[string]string {
	environ := requiredEnv()
	// [estudo] maps.Copy copia todas as chaves de overrides para environ (sobrescrevendo).
	maps.Copy(environ, overrides)
	return environ
}

// without returns a copy of requiredEnv without the given key.
func without(key string) map[string]string {
	environ := requiredEnv()
	delete(environ, key)
	return environ
}

// [estudo] Toda função de teste começa com "Test" e recebe *testing.T.
// Não há anotação @Test: o "go test" encontra os testes pelo nome.
func TestLoad_AppliesDefaults(t *testing.T) {
	// [estudo] t.Parallel() permite rodar este teste junto com outros.
	// É seguro porque não mexemos nas variáveis de ambiente reais do processo.
	t.Parallel()

	cfg, err := load(requiredEnv())

	// [estudo] require para o teste na hora se falhar (como assertThat + return);
	// assert registra a falha e continua, mostrando todas as diferenças de uma vez.
	require.NoError(t, err)
	assert.Equal(t, EnvDev, cfg.AppEnv)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, 15*time.Minute, cfg.JWT.TTL)
	assert.Equal(t, 5, cfg.Worker.PoolSize)
	assert.Equal(t, 100, cfg.Worker.QueueSize)
	assert.Equal(t, 3, cfg.Worker.MaxRetries)
	assert.Equal(t, 120, cfg.RateLimit.PerMinute)
}

func TestLoad_ReadsEveryVariable(t *testing.T) {
	t.Parallel()

	environ := map[string]string{
		"APP_ENV":               "prod",
		"HTTP_PORT":             "9090",
		"DATABASE_URL":          "postgres://user:pass@db:5432/payhub",
		"REDIS_ADDR":            "redis:6379",
		"JWT_SECRET":            strings.Repeat("s", minProdJWTSecretLen),
		"JWT_TTL":               "1h30m",
		"WORKER_POOL_SIZE":      "10",
		"WORKER_QUEUE_SIZE":     "500",
		"WORKER_MAX_RETRIES":    "0",
		"RATE_LIMIT_PER_MINUTE": "60",
	}

	cfg, err := load(environ)

	require.NoError(t, err)
	// [estudo] Comparar structs inteiras com == / assert.Equal funciona em Go:
	// a igualdade é campo a campo, sem precisar implementar equals().
	assert.Equal(t, Config{
		AppEnv:    EnvProd,
		HTTP:      HTTPConfig{Port: 9090},
		Database:  DatabaseConfig{URL: "postgres://user:pass@db:5432/payhub"},
		Redis:     RedisConfig{Addr: "redis:6379"},
		JWT:       JWTConfig{Secret: strings.Repeat("s", minProdJWTSecretLen), TTL: 90 * time.Minute},
		Worker:    WorkerConfig{PoolSize: 10, QueueSize: 500, MaxRetries: 0},
		RateLimit: RateLimitConfig{PerMinute: 60},
	}, cfg)
}

// [estudo] Table-driven test: cada caso é um elemento de um slice de structs anônimas.
// Equivale a um @ParameterizedTest com @MethodSource do JUnit 5, mas sem anotações:
// é só um loop chamando t.Run, que cria um subteste com nome próprio.
func TestLoad_ReturnsErrorForInvalidEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		environ map[string]string
		// wantErr lists fragments that must all appear in the error message.
		wantErr []string
	}{
		{
			name:    "missing DATABASE_URL",
			environ: without("DATABASE_URL"),
			wantErr: []string{"DATABASE_URL"},
		},
		{
			name:    "missing REDIS_ADDR",
			environ: without("REDIS_ADDR"),
			wantErr: []string{"REDIS_ADDR"},
		},
		{
			name:    "missing JWT_SECRET",
			environ: without("JWT_SECRET"),
			wantErr: []string{"JWT_SECRET"},
		},
		{
			name:    "empty DATABASE_URL",
			environ: withEnv(map[string]string{"DATABASE_URL": ""}),
			wantErr: []string{"DATABASE_URL"},
		},
		// [estudo] Quando não consegue converter um valor, a biblioteca cita o nome do CAMPO Go (Port, TTL)
		// e o valor recebido, não o nome da variável de ambiente.
		{
			name:    "non numeric HTTP_PORT",
			environ: withEnv(map[string]string{"HTTP_PORT": "abc"}),
			wantErr: []string{`field "Port"`, `"abc"`},
		},
		{
			name:    "invalid JWT_TTL duration",
			environ: withEnv(map[string]string{"JWT_TTL": "15 minutes"}),
			wantErr: []string{`field "TTL"`, `"15 minutes"`},
		},
		{
			name:    "unknown APP_ENV",
			environ: withEnv(map[string]string{"APP_ENV": "staging"}),
			wantErr: []string{`APP_ENV must be "dev" or "prod"`},
		},
		{
			name:    "HTTP_PORT out of range",
			environ: withEnv(map[string]string{"HTTP_PORT": "70000"}),
			wantErr: []string{"HTTP_PORT must be between 1 and 65535"},
		},
		{
			name:    "non positive JWT_TTL",
			environ: withEnv(map[string]string{"JWT_TTL": "0s"}),
			wantErr: []string{"JWT_TTL must be positive"},
		},
		{
			name:    "short JWT_SECRET in prod",
			environ: withEnv(map[string]string{"APP_ENV": "prod", "JWT_SECRET": "change-me-in-dev"}),
			wantErr: []string{"JWT_SECRET must have at least 32 characters in prod"},
		},
		{
			name:    "zero WORKER_POOL_SIZE",
			environ: withEnv(map[string]string{"WORKER_POOL_SIZE": "0"}),
			wantErr: []string{"WORKER_POOL_SIZE must be at least 1"},
		},
		{
			name:    "zero WORKER_QUEUE_SIZE",
			environ: withEnv(map[string]string{"WORKER_QUEUE_SIZE": "0"}),
			wantErr: []string{"WORKER_QUEUE_SIZE must be at least 1"},
		},
		{
			name:    "negative WORKER_MAX_RETRIES",
			environ: withEnv(map[string]string{"WORKER_MAX_RETRIES": "-1"}),
			wantErr: []string{"WORKER_MAX_RETRIES must not be negative"},
		},
		{
			name:    "zero RATE_LIMIT_PER_MINUTE",
			environ: withEnv(map[string]string{"RATE_LIMIT_PER_MINUTE": "0"}),
			wantErr: []string{"RATE_LIMIT_PER_MINUTE must be at least 1"},
		},
		{
			name: "reports every validation error at once",
			environ: withEnv(map[string]string{
				"HTTP_PORT":        "0",
				"WORKER_POOL_SIZE": "0",
			}),
			wantErr: []string{"HTTP_PORT", "WORKER_POOL_SIZE"},
		},
	}

	// [estudo] range sobre um slice devolve (índice, valor); "_" descarta o índice.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := load(tt.environ)

			require.Error(t, err)
			for _, fragment := range tt.wantErr {
				assert.ErrorContains(t, err, fragment)
			}
			assert.Equal(t, Config{}, cfg, "a failed load must return the zero value")
		})
	}
}

func TestLoad_AcceptsStrongJWTSecretInProd(t *testing.T) {
	t.Parallel()

	cfg, err := load(withEnv(map[string]string{
		"APP_ENV":    "prod",
		"JWT_SECRET": strings.Repeat("x", minProdJWTSecretLen),
	}))

	require.NoError(t, err)
	assert.Equal(t, EnvProd, cfg.AppEnv)
}
