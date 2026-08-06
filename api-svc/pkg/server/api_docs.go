package server

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-stock-prediction/pkg/logger"
)

// DocEntry represents a single markdown document discovered under DOCS_ROOT.
type DocEntry struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	Group string `json:"group"`
	Order int    `json:"order"`
}

// DocContent is the response body for GET /api/docs/raw.
type DocContent struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// getDocsRoot returns the directory that contains project markdown docs.
// Reads the DOCS_ROOT environment variable, falling back to "/repo".
func getDocsRoot() string {
	dir := os.Getenv("DOCS_ROOT")
	if dir == "" {
		return "/repo"
	}
	return dir
}

// docGroupAndOrder maps a relative path (forward-slash separated) to its display
// group and sort order as specified in the API contract.
func docGroupAndOrder(relPath string) (string, int) {
	// Normalise to forward slashes (just in case filepath.WalkDir uses OS sep).
	rel := filepath.ToSlash(relPath)

	switch {
	case !strings.Contains(rel, "/"):
		// Root-level files: CLAUDE.md, README.md, etc.
		return "Tổng quan", 0
	case strings.HasPrefix(rel, "docs/claude/"):
		return "Kiến trúc & tham chiếu", 1
	case strings.HasPrefix(rel, "docs/algos/"):
		return "Thuật toán", 2
	case strings.HasPrefix(rel, "docs/tactics/"):
		return "Chiến thuật bot", 3
	case strings.HasPrefix(rel, "docs/incidents/"):
		return "Sự cố", 4
	case strings.HasPrefix(rel, "docs/superpowers/plans/"):
		return "Kế hoạch", 5
	case strings.HasPrefix(rel, "docs/superpowers/specs/"):
		return "Thiết kế (specs)", 6
	default:
		return "Khác", 9
	}
}

// extractTitle reads at most the first 4 KB of the file and returns the text of
// the first Markdown H1 heading ("# Title"). If none is found the file's base
// name without extension is returned.
func extractTitle(absPath string) string {
	f, err := os.Open(absPath)
	if err != nil {
		return strings.TrimSuffix(filepath.Base(absPath), ".md")
	}
	defer f.Close()

	// Read up to 4 KB — enough to find any heading at the top of the file.
	buf := make([]byte, 4096)
	n, _ := io.ReadFull(f, buf)
	scanner := bufio.NewScanner(bytes.NewReader(buf[:n]))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return strings.TrimSuffix(filepath.Base(absPath), ".md")
}

// ListDocsHandler godoc
//
//	@Summary      List project documentation files
//	@Description  Walks DOCS_ROOT and returns metadata for every *.md file found. Requires admin role.
//	@Tags         Docs
//	@Produce      json
//	@Success      200  {object}  map[string][]DocEntry  "docs array"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      403  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/docs [get]
func ListDocsHandler(w http.ResponseWriter, r *http.Request) {
	docsRoot := getDocsRoot()

	// If the root directory does not exist, return an empty list (not an error).
	if _, err := os.Stat(docsRoot); os.IsNotExist(err) {
		ResponseSuccess(w, http.StatusOK, map[string][]DocEntry{"docs": {}})
		return
	}

	var docs []DocEntry

	err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable entries gracefully.
			return nil
		}

		name := d.Name()

		// Skip hidden directories and node_modules to keep the walk fast.
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only markdown files.
		if !strings.HasSuffix(name, ".md") {
			return nil
		}

		// Build relative path with forward slashes.
		rel, relErr := filepath.Rel(docsRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		// Curate: only serve project docs — top-level *.md (CLAUDE.md, README.md, …)
		// and anything under docs/. Skip stray *.md scattered across service
		// subdirectories (node READMEs, tooling notes) so the viewer stays clean.
		if strings.Contains(rel, "/") && !strings.HasPrefix(rel, "docs/") {
			return nil
		}

		title := extractTitle(path)
		group, order := docGroupAndOrder(rel)

		docs = append(docs, DocEntry{
			Path:  rel,
			Title: title,
			Group: group,
			Order: order,
		})
		return nil
	})

	if err != nil {
		logger.Ctx(r.Context()).Errorf("ListDocsHandler: walk %s failed: %v", docsRoot, err)
		ResponseError(w, http.StatusInternalServerError, "failed to list documentation files")
		return
	}

	// Sort: order ascending, then title A→Z.
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Order != docs[j].Order {
			return docs[i].Order < docs[j].Order
		}
		return docs[i].Title < docs[j].Title
	})

	if docs == nil {
		docs = []DocEntry{}
	}

	ResponseSuccess(w, http.StatusOK, map[string][]DocEntry{"docs": docs})
}

// GetDocHandler godoc
//
//	@Summary      Get raw content of a documentation file
//	@Description  Returns the raw Markdown content of a single file from DOCS_ROOT. Requires admin role.
//	@Tags         Docs
//	@Produce      json
//	@Param        path  query  string  true  "Relative path to the markdown file (e.g. docs/claude/database.md)"
//	@Success      200  {object}  DocContent
//	@Failure      400  {object}  ResponseFailure
//	@Failure      401  {object}  ResponseFailure
//	@Failure      403  {object}  ResponseFailure
//	@Failure      404  {object}  ResponseFailure
//	@Failure      413  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/docs/raw [get]
func GetDocHandler(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")

	// --- Input validation ---
	if relPath == "" {
		ResponseError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}
	if strings.Contains(relPath, "..") {
		ResponseError(w, http.StatusBadRequest, "invalid path: path traversal not allowed")
		return
	}
	if !strings.HasSuffix(relPath, ".md") {
		ResponseError(w, http.StatusBadRequest, "invalid path: only .md files are served")
		return
	}

	docsRoot := getDocsRoot()

	// Resolve absolute paths and verify the resolved file stays within docsRoot.
	absRoot, err := filepath.Abs(docsRoot)
	if err != nil {
		logger.Ctx(r.Context()).Errorf("GetDocHandler: cannot resolve docsRoot %s: %v", docsRoot, err)
		ResponseError(w, http.StatusInternalServerError, "internal error resolving docs root")
		return
	}

	// filepath.Clean("/"+relPath) strips any leading "../" sequences before joining.
	absResolved, err := filepath.Abs(filepath.Join(absRoot, filepath.Clean("/"+relPath)))
	if err != nil {
		logger.Ctx(r.Context()).Errorf("GetDocHandler: cannot resolve path %s: %v", relPath, err)
		ResponseError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Enforce containment: resolved path must be strictly inside docsRoot.
	sep := string(filepath.Separator)
	if absResolved != absRoot && !strings.HasPrefix(absResolved, absRoot+sep) {
		ResponseError(w, http.StatusBadRequest, "invalid path: path traversal not allowed")
		return
	}

	// --- File size guard (2 MB) ---
	const maxSize = 2 * 1024 * 1024
	info, err := os.Stat(absResolved)
	if err != nil {
		if os.IsNotExist(err) {
			ResponseError(w, http.StatusNotFound, "documentation file not found")
			return
		}
		logger.Ctx(r.Context()).Errorf("GetDocHandler: stat %s: %v", absResolved, err)
		ResponseError(w, http.StatusInternalServerError, "failed to read file")
		return
	}
	if info.Size() > maxSize {
		ResponseError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MB)")
		return
	}

	// --- Read content ---
	raw, err := os.ReadFile(absResolved)
	if err != nil {
		logger.Ctx(r.Context()).Errorf("GetDocHandler: read %s: %v", absResolved, err)
		ResponseError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Build relative path with forward slashes for the response.
	rel, _ := filepath.Rel(absRoot, absResolved)
	rel = filepath.ToSlash(rel)

	title := extractTitle(absResolved)

	ResponseSuccess(w, http.StatusOK, DocContent{
		Path:    rel,
		Title:   title,
		Content: string(raw),
	})
}
