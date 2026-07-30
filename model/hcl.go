package model

import "github.com/hashicorp/hcl/v2"

type Config struct {
	Workflows []*Workflow `hcl:"workflow,block"`
}

type Workflow struct {
	Id          string      `hcl:"id,label"`
	Description string      `hcl:"description"`
	Variables   []*Variable `hcl:"variable,block"`
	Steps       []*Step     `hcl:"step,block"`
}

type Variable struct {
	Name    string `hcl:",label"`
	Default string `hcl:"default,optional"`
}

type Step struct {
	Id          string         `hcl:"id,label"`
	DependsOn   []string       `hcl:"depends_on,optional"`
	Type        string         `hcl:"type"`
	Description string         `hcl:"description"`
	When        hcl.Expression `hcl:"when,optional"`
	Remain      hcl.Body       `hcl:",remain"`
}
