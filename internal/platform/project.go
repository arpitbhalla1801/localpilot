package platform

import "github.com/localpilot/localpilot/internal/models"

// DetectProject is exported for use by the agent and CLI layers.
func DetectProject(cwd string) *models.Project {
	return detectProject(cwd)
}
