package harness

// JSON Schemas for structured output. Every role answers in a fixed shape,
// so no tokens go to prose and the harness never parses free text.

func obj(props map[string]any) map[string]any {
	req := make([]string, 0, len(props))
	for k := range props {
		req = append(req, k)
	}
	return map[string]any{"type": "object", "properties": props, "required": req, "additionalProperties": false}
}

var (
	str     = map[string]any{"type": "string"}
	strList = map[string]any{"type": "array", "items": str}
	boolean = map[string]any{"type": "boolean"}
	fileArr = map[string]any{"type": "array", "items": obj(map[string]any{"path": str, "content": str})}
)

type Requirements struct {
	Summary            string   `json:"summary"`
	Requirements       []string `json:"requirements"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Assumptions        []string `json:"assumptions"`
	OutOfScope         []string `json:"out_of_scope"`
}

var requirementsSchema = obj(map[string]any{"summary": str, "requirements": strList, "acceptance_criteria": strList, "assumptions": strList, "out_of_scope": strList})

type Plan struct {
	Approve      bool     `json:"approve"`
	Reason       string   `json:"reason"`
	FilesToRead  []string `json:"files_to_read"`
	FilesToWrite []string `json:"files_to_write"`
	Steps        []string `json:"steps"`
	TestPlan     []string `json:"test_plan"`
	Risk         string   `json:"risk"`
}

var planSchema = obj(map[string]any{"approve": boolean, "reason": str, "files_to_read": strList, "files_to_write": strList, "steps": strList, "test_plan": strList, "risk": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}}})

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Change struct {
	Files []File `json:"files"`
	Notes string `json:"notes"`
}

var changeSchema = obj(map[string]any{"files": fileArr, "notes": str})

type Finding struct {
	File     string `json:"file"`
	Severity string `json:"severity"`
	Issue    string `json:"issue"`
	Fix      string `json:"fix"`
}

type Review struct {
	Verdict  string    `json:"verdict"`
	Findings []Finding `json:"findings"`
}

var reviewSchema = obj(map[string]any{
	"verdict":  map[string]any{"type": "string", "enum": []string{"approve", "changes"}},
	"findings": map[string]any{"type": "array", "items": obj(map[string]any{"file": str, "severity": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}}, "issue": str, "fix": str})},
})

type DocsOut struct {
	PRTitle   string   `json:"pr_title"`
	PRBody    string   `json:"pr_body"`
	Changelog string   `json:"changelog_entry"`
	APIDocs   []string `json:"api_docs"`
}

var docsSchema = obj(map[string]any{"pr_title": str, "pr_body": str, "changelog_entry": str, "api_docs": strList})
