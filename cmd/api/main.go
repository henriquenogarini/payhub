// Package main is the entrypoint of the PayHub HTTP API.
//
// [estudo] Todo executável Go vive num pacote chamado "main" e começa pela
// função main(). Não existe classe envolvendo: é só uma função solta no pacote.
// O nome da pasta (cmd/api) vira o nome do binário gerado por "go build".
package main

// [estudo] Imports são caminhos de pacote. Pacotes da stdlib têm caminhos curtos
// ("fmt", "os"); pacotes externos usam a URL do módulo (ex.: "github.com/gin-gonic/gin").
// Import não usado é ERRO de compilação em Go, não apenas um warning.
import (
	"fmt"
	"os"

	"github.com/henriquenogarini/payhub/internal/config"
)

// [estudo] main() fica o mais curta possível: só chama run() e decide o código de saída.
// Motivo: os.Exit encerra o processo na hora, SEM executar os "defer" pendentes.
// Deixando toda a lógica em run(), qualquer defer de limpeza (fechar banco, flush de log)
// roda antes de voltarmos para cá. Em Java, uma exceção não tratada subiria até a JVM;
// em Go, o erro é um valor comum que devolvemos e tratamos explicitamente.
func main() {
	if err := run(); err != nil {
		// [estudo] Erros vão para stderr; código de saída != 0 sinaliza falha
		// para quem chamou o processo (terminal, Docker, CI).
		fmt.Fprintf(os.Stderr, "payhub api: %v\n", err)
		os.Exit(1)
	}
}

// run wires the application dependencies and starts it.
// It returns an error instead of exiting so that deferred cleanups always run.
//
// [estudo] "run" começa com letra minúscula, então é privada ao pacote main.
// Em Go a visibilidade é definida pela primeira letra: Maiúscula = exportado (public),
// minúscula = não exportado (package-private). Não existem as palavras public/private.
func run() error {
	// [estudo] Nas próximas subpartes este é o lugar do wiring manual:
	// criar logger (0.3), subir o servidor HTTP (0.4), conectar no Postgres (0.5)...
	cfg, err := config.Load()
	if err != nil {
		// [estudo] Envolvemos o erro com contexto ("load config: ...") e devolvemos para o main.
		// A mensagem final fica encadeada: "payhub api: load config: validate config: ...".
		return fmt.Errorf("load config: %w", err)
	}

	// [estudo] Nunca imprimimos a config inteira: ela contém segredos (JWT_SECRET, senha do banco).
	fmt.Printf("payhub api: starting (env=%s, port=%d)\n", cfg.AppEnv, cfg.HTTP.Port)

	// [estudo] nil é o "zero value" de error: significa "deu tudo certo".
	return nil
}
