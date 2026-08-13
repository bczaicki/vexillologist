package ld

// SemanticPatch is the body of a LaunchDarkly semantic-patch request
// (Content-Type: application/json; domain-model=launchdarkly.semanticpatch).
type SemanticPatch struct {
	// EnvironmentKey scopes environment-level instructions; flag-wide
	// instructions (e.g. archiveFlag) omit it.
	EnvironmentKey string        `json:"environmentKey,omitempty"`
	Comment        string        `json:"comment,omitempty"`
	Instructions   []Instruction `json:"instructions"`
}

// Instruction is one semantic-patch instruction.
type Instruction struct {
	Kind string `json:"kind"`
}

// ToggleFlagPatch builds the semantic patch that turns a flag's targeting on
// or off in one environment.
func ToggleFlagPatch(envKey string, on bool, comment string) SemanticPatch {
	kind := "turnFlagOff"
	if on {
		kind = "turnFlagOn"
	}
	return SemanticPatch{
		EnvironmentKey: envKey,
		Comment:        comment,
		Instructions:   []Instruction{{Kind: kind}},
	}
}

// ArchiveFlagPatch builds the flag-wide semantic patch that archives a flag.
func ArchiveFlagPatch(comment string) SemanticPatch {
	return SemanticPatch{
		Comment:      comment,
		Instructions: []Instruction{{Kind: "archiveFlag"}},
	}
}
