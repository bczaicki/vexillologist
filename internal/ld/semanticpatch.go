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

// Instruction is one semantic-patch instruction. Value is the argument some
// kinds take (e.g. which SDK credential a client-side availability change
// applies to).
type Instruction struct {
	Kind  string `json:"kind"`
	Value string `json:"value,omitempty"`
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

// RestoreFlagPatch builds the flag-wide semantic patch that restores an
// archived flag.
func RestoreFlagPatch(comment string) SemanticPatch {
	return SemanticPatch{
		Comment:      comment,
		Instructions: []Instruction{{Kind: "restoreFlag"}},
	}
}

// FlagSettingsPatch builds the flag-wide semantic patch for a settings
// update: one instruction per field the update sets.
func FlagSettingsPatch(u FlagSettingsUpdate, comment string) SemanticPatch {
	var instructions []Instruction
	availability := func(on bool, credential string) Instruction {
		kind := "turnOffClientSideAvailability"
		if on {
			kind = "turnOnClientSideAvailability"
		}
		return Instruction{Kind: kind, Value: credential}
	}
	if u.ClientSide != nil {
		instructions = append(instructions, availability(*u.ClientSide, "usingEnvironmentId"))
	}
	if u.Mobile != nil {
		instructions = append(instructions, availability(*u.Mobile, "usingMobileKey"))
	}
	if u.Permanent != nil {
		kind := "makeFlagTemporary"
		if *u.Permanent {
			kind = "makeFlagPermanent"
		}
		instructions = append(instructions, Instruction{Kind: kind})
	}
	return SemanticPatch{
		Comment:      comment,
		Instructions: instructions,
	}
}
