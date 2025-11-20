// Semantic tools end-to-end demonstration on this repo.
// Requirements: gopls on PATH (or GOPATH/bin), Go toolchain.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/semantic"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	repoRoot := findRepoRoot()
	must(os.Chdir(repoRoot))

	lspCfg := config.LSPConfig{
		Command:     findGopls(),
		FileTypes:   []string{"go"},
		RootMarkers: []string{"go.mod"},
	}

	resolver := config.NewShellVariableResolver(env.New())
	client, err := lsp.New(ctx, "demo-gopls", lspCfg, resolver)
	must(err)

	if _, err := client.Initialize(ctx, repoRoot); err != nil {
		log.Fatalf("init LSP: %v", err)
	}
	if err := client.WaitForServerReady(ctx); err != nil {
		log.Fatalf("server not ready: %v", err)
	}

	lspMap := csync.NewMap[string, *lsp.Client]()
	lspMap.Set("go", client)

	retriever := semantic.NewRetriever(lspMap, repoRoot)
	editor := semantic.NewEditor(retriever)

	fixturePath := filepath.Join(repoRoot, "semantic_demo_fixture.go")
	defer os.Remove(fixturePath)
	writeFixture(fixturePath)

	fmt.Println("=== symbol_overview ===")
	overview, err := retriever.SymbolOverview(ctx, filepath.Base(fixturePath), semantic.DefaultMaxAnswerChars)
	must(err)
	fmt.Println(overview)

	fmt.Println("\n=== find_symbol greet ===")
	find, err := retriever.FindSymbols(ctx, "greet", filepath.Base(fixturePath), 1, true, nil, nil, false, 0, semantic.DefaultMaxAnswerChars)
	must(err)
	fmt.Println(find)

	fmt.Println("\n=== replace_symbol_body greet ===")
	newBody := "func greet(name string) string {\n\treturn format(\"patched \" + name)\n}\n"
	err = editor.ReplaceSymbolBody(ctx, "greet", filepath.Base(fixturePath), newBody)
	must(err)
	updated, err := retriever.FindSymbols(ctx, "greet", filepath.Base(fixturePath), 1, true, nil, nil, false, 0, semantic.DefaultMaxAnswerChars)
	must(err)
	fmt.Println(updated)

	fmt.Println("\n=== find_references format ===")
	refs, err := retriever.FindReferences(ctx, "format", filepath.Base(fixturePath), nil, nil, false, false, 0, semantic.DefaultMaxAnswerChars)
	must(err)
	fmt.Println(refs)
}

func must(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func findGopls() string {
	if path, err := exec.LookPath("gopls"); err == nil {
		return path
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
	}
	if candidate := filepath.Join(gopath, "bin", "gopls"); fileExists(candidate) {
		return candidate
	}
	log.Fatal("gopls not found; install with `go install golang.org/x/tools/gopls@latest`")
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot determine cwd: %v", err)
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("go.mod not found; run this demo from inside repo")
		}
		dir = parent
	}
}

func writeFixture(path string) {
	content := `
package main

import "fmt"

func greet(name string) string {
	return format(name)
}

func format(name string) string {
	return fmt.Sprintf("hi %s", name)
}
`
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		log.Fatalf("write fixture: %v", err)
	}
}
