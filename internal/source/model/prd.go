package model

import (
	"encoding/json"
	"fmt"
)

type PRD struct {
	Version   int            `json:"version"`
	Project   string         `json:"project"`
	Overview  string         `json:"overview"`
	Goals     []string       `json:"goals"`
	NonGoals  []string       `json:"nonGoals"`
	Success   []string       `json:"successMetrics"`
	Questions []string       `json:"openQuestions"`
	Stack     PRDStack       `json:"stack"`
	Routes    []PRDRoute     `json:"routes"`
	UI        string         `json:"uiNotes"`
	DataModel []PRDEntity    `json:"dataModel"`
	Import    string         `json:"importFormat"`
	Rules     []string       `json:"rules"`
	Gates     []string       `json:"qualityGates"`
	Stories   []PRDStory     `json:"stories"`
}

type PRDStack struct {
	Framework string `json:"framework"`
	Hosting   string `json:"hosting"`
	Database  string `json:"database"`
	Auth      string `json:"auth"`
}

type PRDRoute struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type PRDEntity struct {
	Name   string     `json:"name"`
	Fields []PRDField `json:"fields"`
}

type PRDField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Note string `json:"note,omitempty"`
}

type PRDStory struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Status             string   `json:"status"` // always "open" initially
	DependsOn          []string `json:"dependsOn,omitempty"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

func (p *PRD) Validate() error {
	if p.Project == "" {
		return fmt.Errorf("project name is required")
	}
	if p.Overview == "" {
		return fmt.Errorf("overview is required")
	}
	if len(p.Stories) == 0 {
		return fmt.Errorf("at least one user story is required")
	}
	for i, s := range p.Stories {
		if s.ID == "" {
			return fmt.Errorf("story %d: id is required", i+1)
		}
		if s.Title == "" {
			return fmt.Errorf("story %d (%s): title is required", i+1, s.ID)
		}
		if s.Description == "" {
			return fmt.Errorf("story %d (%s): description is required", i+1, s.ID)
		}
		if len(s.AcceptanceCriteria) == 0 {
			return fmt.Errorf("story %d (%s): at least one acceptance criterion is required", i+1, s.ID)
		}
	}
	return nil
}

func ParsePRDJSON(data []byte) (*PRD, error) {
	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, err
	}
	return &prd, nil
}
