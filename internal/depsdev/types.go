package depsdev

import "time"

// VersionResponse is the response from GET /v3/systems/{system}/packages/{name}/versions/{version}.
type VersionResponse struct {
	VersionKey struct {
		System  string `json:"system"`
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"versionKey"`
	PublishedAt     time.Time        `json:"publishedAt"`
	IsDefault       bool             `json:"isDefault"`
	AdvisoryKeys    []AdvisoryKey    `json:"advisoryKeys"`
	RelatedProjects []RelatedProject `json:"relatedProjects"`
}

type AdvisoryKey struct {
	ID string `json:"id"`
}

type RelatedProject struct {
	ProjectKey struct {
		ID string `json:"id"`
	} `json:"projectKey"`
	RelationType       string `json:"relationType"`
	RelationProvenance string `json:"relationProvenance"`
}

// ProjectResponse is the response from GET /v3/projects/{id}.
type ProjectResponse struct {
	ProjectKey struct {
		ID string `json:"id"`
	} `json:"projectKey"`
	Scorecard *Scorecard `json:"scorecard"`
}

type Scorecard struct {
	OverallScore float64 `json:"overallScore"`
	Date         string  `json:"date"`
}
