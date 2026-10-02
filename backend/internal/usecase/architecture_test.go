package usecase_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cada contexto de usecase só pode depender dos que estão abaixo dele; ninguém depende de "insights" (a fachada).
// Se este teste falhar, o novo import cria um acoplamento entre contextos: mova o código compartilhado para domain/ ou
// um pacote utilitário, em vez de liberar a dependência aqui.
var allowed = map[string][]string{
	"planning":    {},
	"balances":    {},
	"ledger":      {},
	"openfinance": {},
	"review":      {"planning"},
	"coach":       {"planning", "review"},
	"insights":    {"planning", "review", "balances"},
}

func TestContextDependencies(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ok, known := allowed[e.Name()]
		if !known {
			t.Errorf("pacote %q novo em usecase/: declare as dependências permitidas em allowed", e.Name())
			continue
		}
		files, _ := filepath.Glob(filepath.Join(e.Name(), "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range af.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				i := strings.Index(path, "internal/usecase/")
				if i < 0 {
					continue
				}
				dep := strings.TrimPrefix(path[i:], "internal/usecase/")
				if !contains(ok, dep) {
					t.Errorf("%s importa usecase/%s, que não é permitido a %q (permitidos: %v)", f, dep, e.Name(), ok)
				}
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
