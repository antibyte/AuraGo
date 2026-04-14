package outputcompress

import (
	"fmt"
	"strings"
)

func compressAws(sub, output string) (string, string) {
	switch sub {
	case "ec2":
		return compressAwsTable(output), "aws-ec2"
	case "s3":
		return compressAwsTable(output), "aws-s3"
	case "lambda":
		return compressAwsTable(output), "aws-lambda"
	default:
		return compressAwsTable(output), "aws-generic"
	}
}

// compressAwsTable summarises AWS CLI table/JSON output.
func compressAwsTable(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// Try JSON compaction first
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		compacted, _ := compressAPIOutput("", result)
		return compacted
	}

	lines := strings.Split(result, "\n")
	if len(lines) <= 8 {
		return result
	}

	// For table output, keep header + count summary + error lines
	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	errorCount := 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
			strings.Contains(lower, "terminated") || strings.Contains(lower, "stopped") {
			sb.WriteString(line + "\n")
			errorCount++
		}
	}

	sb.WriteString(fmt.Sprintf("... %d total rows, %d with issues\n", len(lines)-1, errorCount))
	return sb.String()
}

// ─── Ansible Filter ─────────────────────────────────────────────────────────

// compressAnsible extracts task results and failures from Ansible output.
func compressAnsible(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	var sb strings.Builder
	changed, ok, failed, unreachable, skipped := 0, 0, 0, 0, 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// PLAY/PLAYBOOK headers
		if strings.HasPrefix(trimmed, "PLAY") || strings.HasPrefix(trimmed, "PLAYBOOK") ||
			strings.HasPrefix(trimmed, "TASK") {
			sb.WriteString(line + "\n")
			continue
		}

		// Failed/error/unreachable tasks
		if strings.Contains(lower, "fatal") || strings.Contains(lower, "failed") ||
			strings.Contains(lower, "unreachable") || strings.Contains(lower, "error") {
			sb.WriteString(line + "\n")
			continue
		}

		// Changed notifications
		if strings.Contains(lower, "changed") {
			changed++
		}
		if strings.Contains(lower, "ok") && !strings.Contains(lower, "ok=") {
			ok++
		}
		if strings.Contains(lower, "failed") && !strings.Contains(lower, "failed=") {
			failed++
		}
		if strings.Contains(lower, "unreachable") {
			unreachable++
		}
		if strings.Contains(lower, "skipped") {
			skipped++
		}

		// PLAY RECAP section
		if strings.Contains(trimmed, "ok=") || strings.Contains(trimmed, "PLAY RECAP") {
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d ok, %d changed, %d failed, %d unreachable, %d skipped\n",
		ok, changed, failed, unreachable, skipped))

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// ─── Docker Compose Filters ─────────────────────────────────────────────────

// compressDockerCompose routes docker compose subcommands.
func compressHelm(sub string, output string) (string, string) {
	switch sub {
	case "list", "ls":
		return compressHelmList(output), "helm-list"
	case "status":
		return compressHelmStatus(output), "helm-status"
	case "history":
		return compressHelmHistory(output), "helm-history"
	case "get":
		return compressGeneric(output), "helm-get"
	case "repo":
		return compressGeneric(output), "helm-repo"
	default:
		return compressGeneric(output), "helm-generic"
	}
}

// compressHelmList summarises helm list output.
func compressHelmList(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 8 {
		return result
	}

	deployed, failed, other := 0, 0, 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "deployed"):
			deployed++
		case strings.Contains(lower, "failed"):
			failed++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Releases: %d Deployed, %d Failed, %d Other\n", deployed, failed, other))
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n")
	}
	// Include failed releases
	for _, line := range lines[1:] {
		if strings.Contains(strings.ToLower(line), "failed") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// compressHelmStatus extracts key info from helm status output.
func compressHelmStatus(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "STATUS:") || strings.HasPrefix(trimmed, "REVISION:") ||
			strings.HasPrefix(trimmed, "CHART:") || strings.HasPrefix(trimmed, "NAMESPACE:") ||
			strings.HasPrefix(trimmed, "LAST DEPLOYED:") || strings.HasPrefix(trimmed, "NOTES:") {
			sb.WriteString(line + "\n")
			continue
		}
		// Resources section
		if strings.HasPrefix(trimmed, "==>") || strings.HasPrefix(trimmed, "NAME:") ||
			strings.HasPrefix(trimmed, "READY") {
			sb.WriteString(line + "\n")
		}
	}
	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressHelmHistory summarises helm history output.
func compressHelmHistory(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Keep header + failed/superseded revisions
	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n")
	}
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "failed") || strings.Contains(lower, "superseded") ||
			strings.Contains(lower, "pending") || strings.Contains(lower, "rollback") {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("... %d total revisions\n", len(lines)-1))
	return sb.String()
}

// ─── Terraform Filters ───────────────────────────────────────────────────────

// compressTerraform routes terraform subcommands.
func compressTerraform(sub string, output string) (string, string) {
	switch sub {
	case "plan":
		return compressTerraformPlan(output), "tf-plan"
	case "apply":
		return compressTerraformApply(output), "tf-apply"
	case "show":
		return compressTerraformShow(output), "tf-show"
	case "state":
		return compressTerraformStateList(output), "tf-state"
	case "output":
		return compressTerraformOutput(output), "tf-output"
	case "init":
		return compressGeneric(output), "tf-init"
	default:
		return compressGeneric(output), "tf-generic"
	}
}

// compressTerraformPlan extracts change summary from terraform plan output.
func compressTerraformPlan(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder
	inChanges := false
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)

		// Plan summary line
		if strings.Contains(trimmed, "Plan:") || strings.Contains(trimmed, "No changes") {
			sb.WriteString(line + "\n")
		}
		// Change markers
		if strings.Contains(trimmed, "will be created") || strings.Contains(trimmed, "will be destroyed") ||
			strings.Contains(trimmed, "will be updated") || strings.Contains(trimmed, "will be replaced") {
			sb.WriteString(line + "\n")
			inChanges = true
		}
		// Resource addresses (indented under change markers)
		if inChanges && (strings.HasPrefix(trimmed, "+ ") || strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "~ ") || strings.HasPrefix(trimmed, "-/+ ")) {
			sb.WriteString(line + "\n")
		}
		// Errors and warnings
		if strings.Contains(strings.ToLower(trimmed), "error") ||
			strings.Contains(strings.ToLower(trimmed), "warning") {
			sb.WriteString(line + "\n")
		}
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressTerraformApply extracts result summary from terraform apply output.
func compressTerraformApply(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)

		// Apply result
		if strings.Contains(trimmed, "Apply complete!") || strings.Contains(trimmed, "Resources:") {
			sb.WriteString(line + "\n")
		}
		// Errors
		if strings.Contains(strings.ToLower(trimmed), "error") {
			sb.WriteString(line + "\n")
		}
		// Outputs section: "Outputs:" header or indented lines with " = "
		if strings.HasPrefix(trimmed, "Outputs:") {
			sb.WriteString(line + "\n")
		} else if strings.Contains(trimmed, " = ") && !strings.Contains(trimmed, "Apply") {
			sb.WriteString(line + "\n")
		}
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressTerraformShow summarises terraform show output.
func compressTerraformShow(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 15 {
		return result
	}

	var sb strings.Builder
	resourceCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Resource headers: resource "type" "name"
		if strings.Contains(trimmed, "resource ") && strings.Contains(trimmed, "\"") {
			sb.WriteString(line + "\n")
			resourceCount++
		}
		// Data sources
		if strings.Contains(trimmed, "data ") && strings.Contains(trimmed, "\"") {
			sb.WriteString(line + "\n")
		}
		// Outputs
		if strings.HasPrefix(trimmed, "output ") {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("\n... %d resources total\n", resourceCount))
	return sb.String()
}

// compressTerraformStateList summarises terraform state list output.
func compressTerraformStateList(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 15 {
		return result
	}

	// Group by resource type
	types := make(map[string]int)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: type.name or module.type.name
		parts := strings.Split(line, ".")
		var resType string
		if len(parts) >= 2 && parts[0] == "module" && len(parts) >= 3 {
			resType = parts[1]
		} else if len(parts) >= 2 {
			resType = parts[0]
		} else {
			resType = line
		}
		types[resType]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d resources in %d types:\n", len(lines), len(types)))
	for typ, count := range types {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", typ, count))
	}
	return sb.String()
}

// compressTerraformOutput summarises terraform output output.
func compressTerraformOutput(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Keep output names and values, truncate long values
	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Output format: name = "value" or name = value
		if strings.Contains(trimmed, " = ") {
			parts := strings.SplitN(trimmed, " = ", 2)
			if len(parts) == 2 && len(parts[1]) > 200 {
				sb.WriteString(parts[0] + " = " + parts[1][:200] + "... (truncated)\n")
			} else {
				sb.WriteString(line + "\n")
			}
		} else {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// ─── SSH Diagnostic Filters ──────────────────────────────────────────────────

// compressDiskFree summarises df output, highlighting high-usage filesystems.
