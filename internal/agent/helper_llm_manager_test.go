package agent

import (
	"context"
	"testing"
)

func TestParseHelperTurnBatchResultStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"memory_analysis\":{\"facts\":[{\"content\":\"User runs Proxmox\",\"category\":\"infrastructure\",\"confidence\":0.93}],\"preferences\":[],\"corrections\":[],\"pending_actions\":[{\"title\":\"Review backup job\",\"summary\":\"Follow up on the failed nightly backup job.\",\"trigger_query\":\"backup job review\",\"confidence\":0.82}]},\"activity_digest\":{\"intent\":\"Check backup issue\",\"user_goal\":\"Resolve the failed nightly backup job\",\"actions_taken\":[\"Reviewed recent backup context\"],\"outcomes\":[\"Identified the failing job\"],\"important_points\":[\"Failure affects nightly backups\"],\"pending_items\":[\"Apply the backup fix\"],\"importance\":3,\"entities\":[\"proxmox\",\"backup-job\"]},\"personality_analysis\":{\"mood_analysis\":{\"user_sentiment\":\"concerned\",\"agent_appropriate_response_mood\":\"focused\",\"relationship_delta\":0.03,\"trait_deltas\":{\"empathy\":0.02},\"user_profile_updates\":[]},\"emotion_state\":{\"description\":\"Ich fuehle mich ruhig und aufmerksam.\",\"primary_mood\":\"focused\",\"secondary_mood\":\"steady\",\"valence\":0.1,\"arousal\":0.3,\"confidence\":0.8,\"cause\":\"klare technische stoerung\",\"recommended_response_style\":\"calm_and_clear\"}}}\n```"

	got, err := parseHelperTurnBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperTurnBatchResult: %v", err)
	}
	if len(got.MemoryAnalysis.Facts) != 1 || got.MemoryAnalysis.Facts[0].Content != "User runs Proxmox" {
		t.Fatalf("facts = %#v", got.MemoryAnalysis.Facts)
	}
	if got.ActivityDigest.Intent != "Check backup issue" {
		t.Fatalf("intent = %q", got.ActivityDigest.Intent)
	}
	if got.ActivityDigest.Importance != 3 {
		t.Fatalf("importance = %d", got.ActivityDigest.Importance)
	}
	if got.PersonalityAnalysis.MoodAnalysis.AgentMood != "focused" {
		t.Fatalf("agent mood = %q", got.PersonalityAnalysis.MoodAnalysis.AgentMood)
	}
}

func TestHelperLLMManagerAnalyzeTurnUsesSharedModel(t *testing.T) {
	client := &mockChatClient{
		response: `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{"intent":"Investigate alert","user_goal":"Investigate alert","actions_taken":["Checked recent events"],"outcomes":["Found the alert source"],"important_points":["Alert came from the NAS"],"pending_items":[],"importance":2,"entities":["nas"]},"personality_analysis":{"mood_analysis":{"user_sentiment":"alert","agent_appropriate_response_mood":"focused","relationship_delta":0.01,"trait_deltas":{"thoroughness":0.02},"user_profile_updates":[]},"emotion_state":{"description":"I feel calm and ready to help.","primary_mood":"focused","secondary_mood":"","valence":0.0,"arousal":0.3,"confidence":0.7,"cause":"clear troubleshooting task","recommended_response_style":"calm_and_clear"}}}`,
	}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-model",
		stats:  make(map[string]HelperLLMOperationStats),
	}

	got, err := manager.AnalyzeTurn(context.Background(), "Please check the NAS alert", "I found the source of the alert.", []string{"query_memory"}, []string{"query_memory: completed - found the alert source"}, &helperTurnPersonalityInput{Language: "English"})
	if err != nil {
		t.Fatalf("AnalyzeTurn: %v", err)
	}
	if client.lastReq.Model != "helper-model" {
		t.Fatalf("model = %q, want helper-model", client.lastReq.Model)
	}
	if got.ActivityDigest.UserGoal != "Investigate alert" {
		t.Fatalf("user_goal = %q", got.ActivityDigest.UserGoal)
	}
	stats := manager.SnapshotStats()["analyze_turn"]
	if stats.BatchedItems != 3 || stats.SavedCalls != 2 {
		t.Fatalf("stats = %#v, want batched_items=3 saved_calls=2", stats)
	}
}

func TestHelperLLMManagerAnalyzeTurnUsesCacheForIdenticalInput(t *testing.T) {
	client := &mockChatClient{
		response: `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{"intent":"Investigate alert","user_goal":"Investigate alert","actions_taken":["Checked recent events"],"outcomes":["Found the alert source"],"important_points":["Alert came from the NAS"],"pending_items":[],"importance":2,"entities":["nas"]},"personality_analysis":{"mood_analysis":{"user_sentiment":"alert","agent_appropriate_response_mood":"focused","relationship_delta":0.01,"trait_deltas":{"thoroughness":0.02},"user_profile_updates":[]},"emotion_state":{"description":"I feel calm and ready to help.","primary_mood":"focused","secondary_mood":"","valence":0.0,"arousal":0.3,"confidence":0.7,"cause":"clear troubleshooting task","recommended_response_style":"calm_and_clear"}}}`,
	}
	manager := &helperLLMManager{
		client:        client,
		model:         "helper-model",
		responseCache: make(map[string]string),
	}

	for i := 0; i < 2; i++ {
		if _, err := manager.AnalyzeTurn(context.Background(), "Please check the NAS alert", "I found the source of the alert.", []string{"query_memory"}, []string{"query_memory: completed - found the alert source"}, &helperTurnPersonalityInput{Language: "English"}); err != nil {
			t.Fatalf("AnalyzeTurn call %d: %v", i+1, err)
		}
	}
	if client.calls != 1 {
		t.Fatalf("calls = %d, want 1", client.calls)
	}
}

func TestParseHelperMaintenanceBatchResultStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"daily_summary\":\"Worked on backups and resolved a NAS alert.\",\"kg_extraction\":{\"nodes\":[{\"id\":\"nas\",\"label\":\"NAS\",\"properties\":{\"type\":\"device\"}}],\"edges\":[]}}\n```"

	got, err := parseHelperMaintenanceBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperMaintenanceBatchResult: %v", err)
	}
	if got.DailySummary != "Worked on backups and resolved a NAS alert." {
		t.Fatalf("daily_summary = %q", got.DailySummary)
	}
	if len(got.KGExtraction.Nodes) != 1 || got.KGExtraction.Nodes[0].ID != "nas" {
		t.Fatalf("nodes = %#v", got.KGExtraction.Nodes)
	}
}

func TestHelperLLMManagerAnalyzeMaintenanceSummaryAndKGUsesSharedModel(t *testing.T) {
	client := &mockChatClient{
		response: `{"daily_summary":"Worked on backups and resolved a NAS alert.","kg_extraction":{"nodes":[],"edges":[]}}`,
	}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-model",
	}

	got, err := manager.AnalyzeMaintenanceSummaryAndKG(context.Background(), "2026-04-01", "- [activity] Backup: Checked the failed job", "[user]: The NAS alert came from the backup target.")
	if err != nil {
		t.Fatalf("AnalyzeMaintenanceSummaryAndKG: %v", err)
	}
	if client.lastReq.Model != "helper-model" {
		t.Fatalf("model = %q, want helper-model", client.lastReq.Model)
	}
	if got.DailySummary == "" {
		t.Fatal("daily_summary is empty")
	}
}

func TestParseHelperConsolidationBatchResultStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"batches\":[{\"batch_id\":\"batch_1\",\"facts\":[{\"concept\":\"Backup schedule\",\"content\":\"The nightly backup runs against the NAS target and needs review.\"}]},{\"batch_id\":\"batch_2\",\"facts\":[]}]}\n```"

	got, err := parseHelperConsolidationBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperConsolidationBatchResult: %v", err)
	}
	if len(got.Batches) != 2 {
		t.Fatalf("len(batches) = %d, want 2", len(got.Batches))
	}
	if got.Batches[0].BatchID != "batch_1" {
		t.Fatalf("batch_id = %q", got.Batches[0].BatchID)
	}
	if len(got.Batches[0].Facts) != 1 || got.Batches[0].Facts[0].Concept != "Backup schedule" {
		t.Fatalf("facts = %#v", got.Batches[0].Facts)
	}
}

func TestHelperLLMManagerAnalyzeConsolidationBatchesUsesSharedModel(t *testing.T) {
	client := &mockChatClient{
		response: `{"batches":[{"batch_id":"batch_1","facts":[{"concept":"Backup target","content":"The backup target is the NAS device."}]},{"batch_id":"batch_2","facts":[]}]}`,
	}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-model",
	}

	got, err := manager.AnalyzeConsolidationBatches(context.Background(), []helperConsolidationBatchInput{
		{BatchID: "batch_1", Conversation: "[2026-04-01] user: The backup target is the NAS device."},
		{BatchID: "batch_2", Conversation: "[2026-04-01] assistant: Acknowledged."},
	})
	if err != nil {
		t.Fatalf("AnalyzeConsolidationBatches: %v", err)
	}
	if client.lastReq.Model != "helper-model" {
		t.Fatalf("model = %q, want helper-model", client.lastReq.Model)
	}
	if len(got.Batches) != 2 {
		t.Fatalf("len(batches) = %d, want 2", len(got.Batches))
	}
}

func TestParseHelperCompressionBatchResultStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"memories\":[{\"memory_id\":\"mem_1\",\"compressed\":\"Dense summary for memory one.\"},{\"memory_id\":\"mem_2\",\"compressed\":\"Dense summary for memory two.\"}]}\n```"

	got, err := parseHelperCompressionBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperCompressionBatchResult: %v", err)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("len(memories) = %d, want 2", len(got.Memories))
	}
	if got.Memories[0].MemoryID != "mem_1" {
		t.Fatalf("memory_id = %q", got.Memories[0].MemoryID)
	}
}

func TestHelperLLMManagerCompressMemoryBatchesUsesSharedModel(t *testing.T) {
	client := &mockChatClient{
		response: `{"memories":[{"memory_id":"mem_1","compressed":"Dense summary for memory one."},{"memory_id":"mem_2","compressed":"Dense summary for memory two."}]}`,
	}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-model",
	}

	got, err := manager.CompressMemoryBatches(context.Background(), []helperCompressionBatchInput{
		{MemoryID: "mem_1", Content: "Long memory content one"},
		{MemoryID: "mem_2", Content: "Long memory content two"},
	})
	if err != nil {
		t.Fatalf("CompressMemoryBatches: %v", err)
	}
	if client.lastReq.Model != "helper-model" {
		t.Fatalf("model = %q, want helper-model", client.lastReq.Model)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("len(memories) = %d, want 2", len(got.Memories))
	}
}

func TestParseHelperContentSummaryBatchResultStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"summaries\":[{\"batch_id\":\"item_1\",\"summary\":\"The page confirms the backup target is the NAS.\"},{\"batch_id\":\"item_2\",\"summary\":\"No relevant information found in the article.\"}]}\n```"

	got, err := parseHelperContentSummaryBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperContentSummaryBatchResult: %v", err)
	}
	if len(got.Summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(got.Summaries))
	}
	if got.Summaries[0].BatchID != "item_1" {
		t.Fatalf("batch_id = %q", got.Summaries[0].BatchID)
	}
}

func TestParseHelperRAGBatchResultStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"search_query\":\"backup target nas error\",\"search_terms\":[\"backup target\",\"nas\"],\"candidate_scores\":[{\"memory_id\":\"mem-1\",\"score\":9.5},{\"memory_id\":\"mem-2\",\"score\":2.0}]}\n```"

	got, err := parseHelperRAGBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperRAGBatchResult: %v", err)
	}
	if got.SearchQuery != "backup target nas error" {
		t.Fatalf("search_query = %q", got.SearchQuery)
	}
	if len(got.SearchTerms) != 2 {
		t.Fatalf("len(search_terms) = %d, want 2", len(got.SearchTerms))
	}
	if len(got.CandidateScores) != 2 || got.CandidateScores[0].MemoryID != "mem-1" {
		t.Fatalf("candidate_scores = %#v", got.CandidateScores)
	}
}

func TestHelperLLMManagerAnalyzeRAGUsesSharedModel(t *testing.T) {
	client := &mockChatClient{
		response: `{"search_query":"backup target nas error","search_terms":["backup target","nas"],"candidate_scores":[{"memory_id":"mem-1","score":9.1},{"memory_id":"mem-2","score":3.0}]}`,
	}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-model",
	}

	got, err := manager.AnalyzeRAG(context.Background(), "Please check why the NAS backup target fails", []rankedMemory{
		{text: "The backup target is the NAS appliance.", docID: "mem-1", score: 0.62},
		{text: "The user likes green dashboards.", docID: "mem-2", score: 0.44},
	})
	if err != nil {
		t.Fatalf("AnalyzeRAG: %v", err)
	}
	if client.lastReq.Model != "helper-model" {
		t.Fatalf("model = %q, want helper-model", client.lastReq.Model)
	}
	if got.SearchQuery != "backup target nas error" {
		t.Fatalf("search_query = %q", got.SearchQuery)
	}
	if len(got.CandidateScores) != 2 {
		t.Fatalf("len(candidate_scores) = %d, want 2", len(got.CandidateScores))
	}
}

func TestHelperLLMManagerSummarizeContentBatchesUsesSharedModel(t *testing.T) {
	client := &mockChatClient{
		response: `{"summaries":[{"batch_id":"item_1","summary":"The page confirms the backup target is the NAS."},{"batch_id":"item_2","summary":"No relevant information found in the article."}]}`,
	}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-model",
	}

	got, err := manager.SummarizeContentBatches(context.Background(), []helperContentSummaryBatchInput{
		{BatchID: "item_1", SourceName: "web page", SearchQuery: "What is the backup target?", Content: "The backup target is the NAS."},
		{BatchID: "item_2", SourceName: "Wikipedia article", SearchQuery: "What does it say about backups?", Content: "This article is about gardening."},
	})
	if err != nil {
		t.Fatalf("SummarizeContentBatches: %v", err)
	}
	if client.lastReq.Model != "helper-model" {
		t.Fatalf("model = %q, want helper-model", client.lastReq.Model)
	}
	if len(got.Summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(got.Summaries))
	}
}

func TestHelperLLMManagerSummarizeContentBatchesUsesCacheForIdenticalInput(t *testing.T) {
	client := &mockChatClient{
		response: `{"summaries":[{"batch_id":"item_1","summary":"The page confirms the backup target is the NAS."},{"batch_id":"item_2","summary":"No relevant information found in the article."}]}`,
	}
	manager := &helperLLMManager{
		client:        client,
		model:         "helper-model",
		responseCache: make(map[string]string),
	}

	items := []helperContentSummaryBatchInput{
		{BatchID: "item_1", SourceName: "web page", SearchQuery: "What is the backup target?", Content: "The backup target is the NAS."},
		{BatchID: "item_2", SourceName: "Wikipedia article", SearchQuery: "What does it say about backups?", Content: "This article is about gardening."},
	}

	for i := 0; i < 2; i++ {
		if _, err := manager.SummarizeContentBatches(context.Background(), items); err != nil {
			t.Fatalf("SummarizeContentBatches call %d: %v", i+1, err)
		}
	}
	if client.calls != 1 {
		t.Fatalf("calls = %d, want 1", client.calls)
	}
}

func TestHelperLLMManagerSnapshotStatsTracksRequestsCacheHitsAndCalls(t *testing.T) {
	client := &mockChatClient{
		response: `{"summaries":[{"batch_id":"item_1","summary":"The page confirms the backup target is the NAS."},{"batch_id":"item_2","summary":"No relevant information found in the article."}]}`,
	}
	manager := &helperLLMManager{
		client:        client,
		model:         "helper-model",
		responseCache: make(map[string]string),
		stats:         make(map[string]HelperLLMOperationStats),
	}

	items := []helperContentSummaryBatchInput{
		{BatchID: "item_1", SourceName: "web page", SearchQuery: "What is the backup target?", Content: "The backup target is the NAS."},
		{BatchID: "item_2", SourceName: "Wikipedia article", SearchQuery: "What does it say about backups?", Content: "This article is about gardening."},
	}
	for i := 0; i < 2; i++ {
		if _, err := manager.SummarizeContentBatches(context.Background(), items); err != nil {
			t.Fatalf("SummarizeContentBatches call %d: %v", i+1, err)
		}
	}

	stats := manager.SnapshotStats()
	got, ok := stats["content_summaries"]
	if !ok {
		t.Fatal("missing content_summaries stats")
	}
	if got.Requests != 2 {
		t.Fatalf("requests = %d, want 2", got.Requests)
	}
	if got.CacheHits != 1 {
		t.Fatalf("cache_hits = %d, want 1", got.CacheHits)
	}
	if got.LLMCalls != 1 {
		t.Fatalf("llm_calls = %d, want 1", got.LLMCalls)
	}
	if got.BatchedItems != 4 {
		t.Fatalf("batched_items = %d, want 4", got.BatchedItems)
	}
	if got.SavedCalls != 2 {
		t.Fatalf("saved_calls = %d, want 2", got.SavedCalls)
	}
}

func TestHelperLLMManagerObserveFallbackTracksDetail(t *testing.T) {
	manager := &helperLLMManager{
		stats: make(map[string]HelperLLMOperationStats),
	}

	manager.ObserveFallback("content_summaries", "batch failed")
	stats := manager.SnapshotStats()
	got, ok := stats["content_summaries"]
	if !ok {
		t.Fatal("missing content_summaries stats")
	}
	if got.Fallbacks != 1 {
		t.Fatalf("fallbacks = %d, want 1", got.Fallbacks)
	}
	if got.LastDetail != "batch failed" {
		t.Fatalf("last_detail = %q, want batch failed", got.LastDetail)
	}
}

func TestTrimJSONResponseStripsThinkBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "think block before JSON",
			in:   "<think>\nsome reasoning here\n</think>\n{\"key\":\"value\"}",
			want: `{"key":"value"}`,
		},
		{
			name: "think block with trailing whitespace",
			in:   "<Think>reasoning</Think>   {\"key\":\"value\"}",
			want: `{"key":"value"}`,
		},
		{
			name: "multiple think blocks",
			in:   "<think>first</think><think>second</think>{\"key\":\"value\"}",
			want: `{"key":"value"}`,
		},
		{
			name: "unclosed think block drops remainder",
			in:   "<think>unclosed reasoning",
			want: ``,
		},
		{
			name: "no think block passes through unchanged",
			in:   `{"key":"value"}`,
			want: `{"key":"value"}`,
		},
		{
			name: "code fence still stripped after think block",
			in:   "<think>reasoning</think>\n```json\n{\"key\":\"value\"}\n```",
			want: `{"key":"value"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := trimJSONResponse(tc.in)
			if got != tc.want {
				t.Errorf("trimJSONResponse(%q)\n got  %q\n want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseHelperRAGBatchResultStripsThinkBlock(t *testing.T) {
	raw := "<think>\nLet me find the best query.\n</think>\n{\"search_query\":\"nas backup error\",\"search_terms\":[\"nas\"],\"candidate_scores\":[]}"
	got, err := parseHelperRAGBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperRAGBatchResult with think block: %v", err)
	}
	if got.SearchQuery != "nas backup error" {
		t.Fatalf("search_query = %q", got.SearchQuery)
	}
}

func TestParseHelperTurnBatchResultStripsThinkBlock(t *testing.T) {
	json := `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{"intent":"test","user_goal":"","actions_taken":[],"outcomes":[],"important_points":[],"pending_items":[],"importance":1,"entities":[]},"personality_analysis":{"mood_analysis":{"user_sentiment":"neutral","agent_appropriate_response_mood":"calm","relationship_delta":0,"trait_deltas":{},"user_profile_updates":[]},"emotion_state":{"description":"","primary_mood":"calm","secondary_mood":"","valence":0,"arousal":0,"confidence":0,"cause":"","recommended_response_style":""}}}`
	raw := "<think>\nAnalyzing the conversation...\n</think>\n" + json
	got, err := parseHelperTurnBatchResult(raw)
	if err != nil {
		t.Fatalf("parseHelperTurnBatchResult with think block: %v", err)
	}
	if got.ActivityDigest.Intent != "test" {
		t.Fatalf("intent = %q", got.ActivityDigest.Intent)
	}
}
