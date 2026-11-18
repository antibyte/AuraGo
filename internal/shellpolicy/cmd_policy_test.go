package shellpolicy

import "testing"

func TestValidateCommandRecognizesCmdExecutionOptions(t *testing.T) {
	for _, policyInput := range []string{`cmd /k ver`, `cmd.exe "/c" ver`, `cmd.exe "/K" ver`} {
		if err := ValidateCommandWithEscape(policyInput, 0x60); err == nil {
			t.Errorf("execution option accepted in %q", policyInput)
		}
	}
	for _, policyInput := range []string{`cmd /?`, `echo "cmd /k"`} {
		if err := ValidateCommandWithEscape(policyInput, 0x60); err != nil {
			t.Errorf("help or literal text blocked in %q: %v", policyInput, err)
		}
	}
}
