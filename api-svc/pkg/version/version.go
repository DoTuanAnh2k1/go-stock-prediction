package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-stock-prediction/pkg/logger"
)

const (
	ServiceName  = "api-svc"
	versionsDir  = "/versions"
	versionFile  = "/versions/api-svc.json"
)

// Info holds version information for a single service.
type Info struct {
	Service   string `json:"service"`
	GitSHA    string `json:"git_sha"`
	BuildTime string `json:"build_time"`
	Dirty     string `json:"dirty"`
	StartedAt string `json:"started_at"`
}

var (
	mu      sync.RWMutex
	ownInfo Info
)

// Init reads the three env vars injected by Docker, logs one startup line,
// and writes /versions/api-svc.json. If the directory is not mounted the
// write error is logged as a WARNING and the process continues.
func Init() {
	gitSHA := getenv("GIT_SHA", "unknown")
	buildTime := getenv("BUILD_TIME", "unknown")
	dirty := getenv("GIT_DIRTY", "unknown")
	startedAt := time.Now().Format(time.RFC3339)

	logger.Logger.Infof("version git_sha=%s build_time=%s dirty=%s", gitSHA, buildTime, dirty)

	info := Info{
		Service:   ServiceName,
		GitSHA:    gitSHA,
		BuildTime: buildTime,
		Dirty:     dirty,
		StartedAt: startedAt,
	}

	mu.Lock()
	ownInfo = info
	mu.Unlock()

	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		logger.Logger.Warnf("version: cannot create %s: %v — skipping version file write", versionsDir, err)
		return
	}

	data, _ := json.Marshal(info)
	if err := os.WriteFile(versionFile, data, 0644); err != nil {
		logger.Logger.Warnf("version: cannot write %s: %v — skipping version file write", versionFile, err)
	}
}

// ReadOwn returns the version info for this service as set during Init.
func ReadOwn() Info {
	mu.RLock()
	defer mu.RUnlock()
	return ownInfo
}

// ReadAll reads every *.json file in /versions and returns the parsed entries.
// If /versions is missing or empty, returns an empty slice without error.
func ReadAll() []Info {
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return []Info{}
	}
	infos := make([]Info, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(versionsDir, e.Name()))
		if err != nil {
			continue
		}
		var info Info
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		infos = append(infos, info)
	}
	return infos
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
