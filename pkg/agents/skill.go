package agents

type Skill struct {
	Name         string `json:"name"`          // Skill name from SKILL.md frontmatter
	Description  string `json:"description"`   // Skill description from SKILL.md frontmatter
	FileLocation string `json:"file_location"` // Path to SKILL.md file relative to sandbox-data
}
