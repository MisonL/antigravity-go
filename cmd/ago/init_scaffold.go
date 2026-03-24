package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type initProjectProfile struct {
	DisplayName  string
	ModulePath   string
	ProjectSlug  string
	FrontendName string
}

type initScaffold struct {
	root    string
	profile initProjectProfile
}

func newInitScaffold(root string, moduleOverride string) (initScaffold, error) {
	if err := ensureDirIsEmpty(root); err != nil {
		return initScaffold{}, err
	}

	displayName := filepath.Base(root)
	if displayName == "." || displayName == string(filepath.Separator) || strings.TrimSpace(displayName) == "" {
		displayName = "app"
	}

	projectSlug := sanitizeSlug(displayName, "app")
	modulePath := strings.TrimSpace(moduleOverride)
	if modulePath == "" {
		modulePath = projectSlug
	}
	modulePath = sanitizeModulePath(modulePath, projectSlug)

	return initScaffold{
		root: root,
		profile: initProjectProfile{
			DisplayName:  displayName,
			ModulePath:   modulePath,
			ProjectSlug:  projectSlug,
			FrontendName: projectSlug + "-web",
		},
	}, nil
}

func (s initScaffold) write() ([]string, error) {
	files := buildInitFiles(s.profile)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	createdDirs := make([]string, 0, len(paths))
	seenDirs := map[string]struct{}{}
	written := make([]string, 0, len(paths))
	for _, relPath := range paths {
		if err := ensureParentDirs(s.root, relPath, seenDirs, &createdDirs); err != nil {
			return append(createdDirs, written...), err
		}
		target := filepath.Join(s.root, relPath)
		if err := os.WriteFile(target, []byte(files[relPath]), 0644); err != nil {
			return append(createdDirs, written...), err
		}
		written = append(written, relPath)
	}
	return append(createdDirs, written...), nil
}

func ensureDirIsEmpty(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".DS_Store" {
			continue
		}
		return fmt.Errorf("当前目录不是空目录，发现 %q", name)
	}
	return nil
}

func ensureParentDirs(root string, relPath string, seen map[string]struct{}, created *[]string) error {
	dir := filepath.Dir(relPath)
	if dir == "." {
		return nil
	}
	current := ""
	for _, segment := range strings.Split(filepath.ToSlash(dir), "/") {
		current = filepath.Join(current, segment)
		if _, ok := seen[current]; ok {
			continue
		}
		if err := os.Mkdir(filepath.Join(root, current), 0755); err != nil {
			if !os.IsExist(err) {
				return err
			}
		} else {
			*created = append(*created, current)
		}
		seen[current] = struct{}{}
	}
	return nil
}

func sanitizeModulePath(input string, fallback string) string {
	var builder strings.Builder
	prevSlash := false
	for _, r := range strings.TrimSpace(input) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			prevSlash = false
		case r == '/' || r == '.' || r == '-' || r == '_':
			if r == '/' {
				if prevSlash || builder.Len() == 0 {
					continue
				}
				prevSlash = true
			} else {
				prevSlash = false
			}
			builder.WriteRune(r)
		default:
			if builder.Len() == 0 || prevSlash {
				continue
			}
			builder.WriteRune('-')
			prevSlash = false
		}
	}
	modulePath := strings.Trim(builder.String(), "/.-_")
	if modulePath == "" {
		return fallback
	}
	return modulePath
}

func sanitizeSlug(input string, fallback string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(input) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(unicode.ToLower(r))
			lastDash = false
			continue
		}
		if builder.Len() == 0 || lastDash {
			continue
		}
		builder.WriteByte('-')
		lastDash = true
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = fallback
	}
	if slug[0] >= '0' && slug[0] <= '9' {
		slug = fallback + "-" + slug
	}
	return slug
}
