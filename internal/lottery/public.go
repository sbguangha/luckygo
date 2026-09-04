package lottery

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (h *Hub) effectivePublicBase() string {
	if v := strings.TrimSpace(os.Getenv("LUCKYGO_PUBLIC_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := readPublicBaseFile(); v != "" {
		return v
	}
	return strings.TrimRight(strings.TrimSpace(h.conf.PublicBase), "/")
}

func (h *Hub) publicJoinURL() string {
	base := h.effectivePublicBase()
	if !isPublicHTTPBase(base) {
		return ""
	}
	return base + "/join"
}

func isPublicHTTPBase(base string) bool {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return !isPrivateOrLocal(u.Host)
}

func readPublicBaseFile() string {
	var candidates []string
	add := func(p string) {
		if p != "" {
			candidates = append(candidates, p)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 6; i++ {
			add(filepath.Join(dir, ".runtime", "public-base.url"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), ".runtime", "public-base.url"))
	}
	seen := map[string]struct{}{}
	for _, p := range candidates {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		s = strings.TrimRight(s, "/")
		if s != "" {
			return s
		}
	}
	return ""
}
