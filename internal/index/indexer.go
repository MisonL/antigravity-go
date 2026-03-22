package index

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Symbol represents a code element like a function or type
type Symbol struct {
	Name string `json:"name"`
	Type string `json:"type"` // func, type, value
	File string `json:"file"`
	Line int    `json:"line"`
}

// Indexer maintains a mapping from keywords to symbols and files
type Indexer struct {
	symbols  []Symbol
	keywords map[string][]string // keyword -> list of files
	files    map[string]bool     // indexed files
	mu       sync.RWMutex
	root     string
}

func NewIndexer(root string) *Indexer {
	return &Indexer{
		symbols:  []Symbol{},
		keywords: make(map[string][]string),
		files:    make(map[string]bool),
		root:     root,
	}
}

// ScanProject crawls the root directory and indexes supported files
func (idx *Indexer) ScanProject(ctx context.Context) error {
	fdPath := "./bin/fd"
	if _, err := os.Stat(fdPath); err != nil {
		return idx.scanProjectLegacy(ctx)
	}

	// Use fd for high-speed scanning
	args := []string{
		"--type", "f",
		"-E", ".git", "-E", "node_modules", "-E", ".gemini", "-E", "dist", "-E", "build",
		".", idx.root,
	}
	cmd := exec.Command(fdPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return idx.scanProjectLegacy(ctx)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		// Context cancellation check
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return ctx.Err()
		default:
		}

		path := scanner.Text()
		ext := filepath.Ext(path)
		if ext == ".go" {
			idx.indexGoFile(path)
		} else if isTextFile(ext) {
			idx.indexTextFile(path)
		}
	}

	return cmd.Wait()
}

func (idx *Indexer) scanProjectLegacy(ctx context.Context) error {
	return filepath.Walk(idx.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Context cancellation check
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == ".gemini" || info.Name() == "dist" || info.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".go" {
			return idx.indexGoFile(path)
		} else if isTextFile(ext) {
			return idx.indexTextFile(path)
		}

		return nil
	})
}

func isTextFile(ext string) bool {
	texts := map[string]bool{
		".js":   true,
		".ts":   true,
		".tsx":  true,
		".jsx":  true,
		".html": true,
		".css":  true,
		".json": true,
		".md":   true,
		".yaml": true,
		".yml":  true,
		".mod":  true,
		".sum":  true,
	}
	return texts[ext]
}

func (idx *Indexer) indexTextFile(path string) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear previous keywords for this file
	for k, paths := range idx.keywords {
		var newPaths []string
		for _, p := range paths {
			if p != path {
				newPaths = append(newPaths, p)
			}
		}
		if len(newPaths) == 0 {
			delete(idx.keywords, k)
		} else {
			idx.keywords[k] = newPaths
		}
	}

	// Simple word extraction
	re := regexp.MustCompile(`[a-zA-Z0-9_]{3,}`)
	matches := re.FindAllString(string(content), -1)

	uniqueKeys := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 50 {
			continue
		}
		uniqueKeys[strings.ToLower(m)] = true
	}

	for k := range uniqueKeys {
		idx.keywords[k] = append(idx.keywords[k], path)
	}

	idx.files[path] = true
	return nil
}

func (idx *Indexer) indexGoFile(path string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear previous symbols for this file
	var newSymbols []Symbol
	for _, s := range idx.symbols {
		if s.File != path {
			newSymbols = append(newSymbols, s)
		}
	}
	idx.symbols = newSymbols

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			idx.symbols = append(idx.symbols, Symbol{
				Name: x.Name.Name,
				Type: "func",
				File: path,
				Line: fset.Position(x.Pos()).Line,
			})
		case *ast.TypeSpec:
			idx.symbols = append(idx.symbols, Symbol{
				Name: x.Name.Name,
				Type: "type",
				File: path,
				Line: fset.Position(x.Pos()).Line,
			})
		case *ast.ValueSpec:
			for _, name := range x.Names {
				idx.symbols = append(idx.symbols, Symbol{
					Name: name.Name,
					Type: "value",
					File: path,
					Line: fset.Position(name.Pos()).Line,
				})
			}
		}
		return true
	})

	idx.files[path] = true
	return nil
}

// Search returns symbols and files matching the query
func (idx *Indexer) Search(query string) ([]Symbol, []string) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	query = strings.ToLower(query)
	var symResults []Symbol
	for _, s := range idx.symbols {
		if strings.Contains(strings.ToLower(s.Name), query) {
			symResults = append(symResults, s)
		}
	}

	// Get files from keywords index
	fileResultsMap := make(map[string]bool)
	if paths, ok := idx.keywords[query]; ok {
		for _, p := range paths {
			fileResultsMap[p] = true
		}
	}

	// Also search for substrings in filenames
	for f := range idx.files {
		if strings.Contains(strings.ToLower(f), query) {
			fileResultsMap[f] = true
		}
	}

	var fileResults []string
	for f := range fileResultsMap {
		fileResults = append(fileResults, f)
	}

	return symResults, fileResults
}

func (idx *Indexer) GetSummary() string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return fmt.Sprintf("Project Summary:\n- Files: %d\n- Symbols: %d\n- Keywords: %d",
		len(idx.files), len(idx.symbols), len(idx.keywords))
}
