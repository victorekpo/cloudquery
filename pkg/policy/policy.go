package policy

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

type Policies []*Policy

type Meta struct {
	Type      string `json:"type,omitempty"`
	Version   string `json:"version,omitempty"`
	SubPolicy string `json:"sub_policy,omitempty"`
	Directory string `json:"directory,omitempty"`
}

type View struct {
	Name  string `hcl:"name,label"`
	Title string `hcl:"title,optional"`
	Query string `hcl:"query"`
}

type Configuration struct {
	Providers []*Provider `hcl:"provider,block"`
}

type Provider struct {
	Type    string `hcl:"type,label"`
	Version string `hcl:"version,optional"`
}

type QueryType string

type Check struct {
	Name         string    `hcl:"name,label"`
	Title        string    `hcl:"title,optional"`
	Doc          string    `hcl:"doc,optional"`
	ExpectOutput bool      `hcl:"expect_output,optional"`
	Type         QueryType `hcl:"type,optional"`
	Query        string    `hcl:"query"`
	Reason       string    `hcl:"reason,optional"`
}

type Policy struct {
	// Name of the policy
	Name string `hcl:"name,label"`
	// Short human-readable title about the policy
	Title string `hcl:"title,optional"`
	// Full documentation about the policy, this will be shown in the hub
	Doc    string         `hcl:"doc,optional"`
	Config *Configuration `hcl:"configuration,block"`

	Policies Policies `hcl:"policy,block"`
	Checks   []*Check `hcl:"check,block"`
	Views    []*View  `hcl:"view,block"`

	// Link to policy in filesystem/hub/git etc' to use, if source flag is set, all other attributes aren't allowed.
	Source string `hcl:"source,optional"`

	// List of identifiers that all checks and sub-policies must have, unless sub-policy overrides.
	Identifiers []string

	// Meta is information added to the policy after it was loaded
	meta *Meta
}

const (
	ManualQuery    QueryType = "manual"
	AutomaticQuery QueryType = "automatic"
)

func (pp Policies) All() []string {
	policyNames := make([]string, len(pp))
	for i, p := range pp {
		policyNames[i] = p.Name
	}
	return policyNames
}

func (pp Policies) Properties() map[string]interface{} {
	usedCustom := 0
	policies := make([]*Meta, 0)
	for _, p := range pp {
		if p.meta == nil || p.meta.Type != "hub" {
			usedCustom++
			// we don't add info about custom policies that were executed
			continue
		}
		policies = append(policies, p.meta)
	}
	return map[string]interface{}{
		"used_custom": usedCustom,
		"policies":    policies,
	}
}

func (p Policy) String() string {
	if p.SubPolicy() != "" {
		return fmt.Sprintf("%s//%s", p.Name, p.SubPolicy())
	}
	return p.Name
}

func (p Policy) Version() string {
	if p.meta == nil {
		return "v0.0.0"
	}
	return p.meta.Version
}

func (p Policy) SubPolicy() string {
	if p.meta == nil {
		return ""
	}
	return p.meta.SubPolicy
}

func (p Policy) SourceType() string {
	if p.meta == nil {
		return ""
	}
	return p.meta.Type
}

func (p Policy) HasChecks() bool {
	for _, policy := range p.Policies {
		if policy.HasChecks() {
			return true
		}
	}
	return len(p.Checks) > 0
}

func (p Policy) TotalQueries() int {
	count := 0
	if len(p.Policies) > 0 {
		for _, inner := range p.Policies {
			count += inner.TotalQueries()
		}
	}
	return count + len(p.Checks)
}

// Path should not include the root of the policy
// If no policy or control matches selector then a shell policy is returned

func (p Policy) Filter(path string) Policy {
	if path == "" {
		return p
	}
	selectorPath := strings.SplitN(path, "/", 2)
	if len(selectorPath) == 0 {
		return p
	}
	var emptyPolicy Policy
	nextPolicy := ""
	if strings.Contains(path, "/") {
		nextPolicy = selectorPath[1]
	}
	for _, policy := range p.Policies {
		if policy.Name == selectorPath[0] {
			filtered := policy.Filter(nextPolicy)
			if filtered.Config == nil {
				filtered.Config = p.Config
			}
			return filtered
		}
	}

	for _, check := range p.Checks {
		if check.Name == selectorPath[0] {
			p.Checks = make([]*Check, 0)
			p.Checks = append(p.Checks, check)
			return p
		}
	}

	return emptyPolicy
}

func (p Policy) Sha256Hash() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", p.Policies)))
	return fmt.Sprintf("%x", h.Sum(nil))
}
