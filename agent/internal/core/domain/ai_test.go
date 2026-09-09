package domain

import (
	"testing"
)

func TestSlotPolicy_Precedence(t *testing.T) {
	// 1. Defaults when empty
	emptyPolicy := SlotPolicy{}
	fastModel := GetAssignedModel(SlotFast, emptyPolicy)
	if fastModel.ID != "openrouter/free" {
		t.Errorf("expected default model 'openrouter/free', got '%s'", fastModel.ID)
	}

	deepModel := GetAssignedModel(SlotDeep, emptyPolicy)
	if deepModel.ID != "openrouter/free" {
		t.Errorf("expected default model 'openrouter/free', got '%s'", deepModel.ID)
	}

	// 2. User override precedence
	customFast := ModelRef{
		ID:          "qwen2.5-coder",
		DisplayName: "Qwen 2.5 Coder (Local)",
		ProviderID:  ProviderOllama,
		IsFree:      true,
		EvalOK:      true,
	}

	overridePolicy := SlotPolicy{
		Assignments: map[DiagnosisSlot]ModelRef{
			SlotFast: customFast,
		},
	}

	resolvedFast := GetAssignedModel(SlotFast, overridePolicy)
	if resolvedFast.ID != "qwen2.5-coder" {
		t.Errorf("expected override model 'qwen2.5-coder', got '%s'", resolvedFast.ID)
	}

	// SlotDeep was not overridden, should return default
	resolvedDeep := GetAssignedModel(SlotDeep, overridePolicy)
	if resolvedDeep.ID != "openrouter/free" {
		t.Errorf("expected default model 'openrouter/free' for unoverridden slot, got '%s'", resolvedDeep.ID)
	}
}
