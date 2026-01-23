package ci

import "os"

type Info struct {
	Provider string `json:"provider"`
	RunID    string `json:"run_id,omitempty"`
	Workflow string `json:"workflow,omitempty"`
	Actor    string `json:"actor,omitempty"`
	URL      string `json:"url,omitempty"`
}

func Detect() *Info {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return &Info{
			Provider: "github-actions",
			RunID:    os.Getenv("GITHUB_RUN_ID"),
			Workflow: os.Getenv("GITHUB_WORKFLOW"),
			Actor:    os.Getenv("GITHUB_ACTOR"),
			URL:      os.Getenv("GITHUB_SERVER_URL") + "/" + os.Getenv("GITHUB_REPOSITORY") + "/actions/runs/" + os.Getenv("GITHUB_RUN_ID"),
		}
	}

	if os.Getenv("GITLAB_CI") == "true" {
		return &Info{
			Provider: "gitlab-ci",
			RunID:    os.Getenv("CI_PIPELINE_ID"),
			Workflow: os.Getenv("CI_JOB_NAME"),
			Actor:    os.Getenv("GITLAB_USER_LOGIN"),
			URL:      os.Getenv("CI_PIPELINE_URL"),
		}
	}

	return nil
}

func IsCI() bool {
	return Detect() != nil
}
