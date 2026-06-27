package server

import (
	"net/http"

	"go-stock-prediction/pkg/version"
)

// versionResponse is the JSON body for GET /api/version.
type versionResponse struct {
	Service    string        `json:"service"`
	GitSHA     string        `json:"git_sha"`
	BuildTime  string        `json:"build_time"`
	Dirty      string        `json:"dirty"`
	StartedAt  string        `json:"started_at"`
	Consistent bool          `json:"consistent"`
	Mismatched []string      `json:"mismatched"`
	Services   []version.Info `json:"services"`
}

// GetVersionHandler godoc
//
//	@Summary      Get version information
//	@Description  Returns git SHA, build time, and dirty flag for api-svc and all co-deployed services. consistent=true when every service's git_sha (ignoring "unknown"/empty) matches api-svc's git_sha.
//	@Tags         Version
//	@Produce      json
//	@Success      200  {object}  versionResponse
//	@Router       /api/version [get]
func GetVersionHandler(w http.ResponseWriter, r *http.Request) {
	own := version.ReadOwn()
	all := version.ReadAll()

	mismatched := []string{}
	for _, s := range all {
		if s.Service == version.ServiceName {
			continue
		}
		sha := s.GitSHA
		if sha == "" || sha == "unknown" {
			continue
		}
		ownSHA := own.GitSHA
		if ownSHA != "" && ownSHA != "unknown" && sha != ownSHA {
			mismatched = append(mismatched, s.Service)
		}
	}

	resp := versionResponse{
		Service:    own.Service,
		GitSHA:     own.GitSHA,
		BuildTime:  own.BuildTime,
		Dirty:      own.Dirty,
		StartedAt:  own.StartedAt,
		Consistent: len(mismatched) == 0,
		Mismatched: mismatched,
		Services:   all,
	}

	ResponseSuccess(w, http.StatusOK, resp)
}
