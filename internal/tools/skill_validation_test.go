package tools

import "testing"

func TestValidateSkillNameRejectsReservedWindowsName(t *testing.T) {
	if _, err := validateSkillName("CON"); err == nil {
		t.Fatal("expected reserved Windows name to be rejected")
	}
}

func TestValidateSkillCodeRejectsOversizedCode(t *testing.T) {
	oversized := make([]byte, maxSkillCodeBytes+1)
	for i := range oversized {
		oversized[i] = 'a'
	}
	if err := validateSkillCode(string(oversized)); err == nil {
		t.Fatal("expected oversized skill code to be rejected")
	}
}
