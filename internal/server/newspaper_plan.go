package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"aurago/internal/newspaper"
	"aurago/internal/security"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type newspaperTopic struct {
	ID, Section, Label string
}

type newspaperPlanBudget struct {
	Searches int                      `json:"searches"`
	Pages    int                      `json:"pages"`
	Seconds  int                      `json:"seconds"`
	Plans    int                      `json:"plans"`
	Spending *newspaperSpendingBudget `json:"spending,omitempty"`
}

type newspaperSpendingBudget struct {
	RemainingUSD float64 `json:"remaining_usd"`
	Blocked      bool    `json:"blocked"`
}

func newspaperTopics(p newspaper.Profile) []newspaperTopic {
	topics := make([]newspaperTopic, 0, len(p.Sections)+len(p.Interests))
	for _, section := range p.Sections {
		topics = append(topics, newspaperTopic{ID: section, Section: section, Label: section})
	}
	for i, interest := range p.Interests {
		topics = append(topics, newspaperTopic{ID: fmt.Sprintf("interest:%d", i), Section: "interests", Label: interest})
	}
	return topics
}

func newspaperCountryName(p newspaper.Profile, lang string) string {
	if region, err := language.ParseRegion(p.Country); err == nil {
		return display.Regions(language.Make(lang)).Name(region)
	}
	return p.Country
}

func newspaperFallbackPlan(p newspaper.Profile, topics []newspaperTopic, round int) []newspaperQuery {
	en := map[string]string{"regional": "local council community news", "national": "national news", "international": "world news", "politics": "politics decisions", "economy": "economy business news", "culture": "culture arts news", "technology": "technology releases security news", "science": "research study discoveries", "environment": "environment climate news", "health": "public health research news", "sport": "sports results news"}
	de := map[string]string{"regional": "Lokalnachrichten Stadtrat Beschluss", "national": "Nachrichten", "international": "internationale Nachrichten", "politics": "Politik Entscheidungen", "economy": "Wirtschaft Unternehmen Nachrichten", "culture": "Kultur Kunst Nachrichten", "technology": "Technik Veröffentlichung Sicherheitsmeldungen", "science": "Forschung neue Studie Entdeckung", "environment": "Umwelt Klima Nachrichten", "health": "Gesundheit Forschung Nachrichten", "sport": "Sport Ergebnisse Nachrichten"}
	local := map[string][3]string{
		"en": {"local council community news", "national news", "news updates"}, "de": {"Lokalnachrichten Stadtrat Beschluss", "Nachrichten", "Neuigkeiten Entwicklungen"},
		"cs": {"místní zprávy městská rada rozhodnutí", "zprávy", "novinky"}, "da": {"lokale nyheder byråd beslutning", "nyheder", "seneste nyt"},
		"el": {"τοπικές ειδήσεις δημοτικό συμβούλιο αποφάσεις", "ειδήσεις", "νέα εξελίξεις"}, "es": {"noticias locales ayuntamiento acuerdos", "noticias", "novedades"},
		"fr": {"actualités locales conseil municipal décisions", "actualités", "nouveautés"}, "hi": {"स्थानीय समाचार नगर परिषद निर्णय", "समाचार", "नई जानकारी"},
		"it": {"notizie locali consiglio comunale delibere", "notizie", "novità"}, "ja": {"地域ニュース 市議会 決議", "ニュース", "最新情報"},
		"nl": {"lokaal nieuws gemeenteraad besluiten", "nieuws", "ontwikkelingen"}, "no": {"lokale nyheter kommunestyre vedtak", "nyheter", "siste nytt"},
		"pl": {"wiadomości lokalne rada miasta uchwały", "wiadomości", "nowości"}, "pt": {"notícias locais câmara municipal decisões", "notícias", "novidades"},
		"sv": {"lokala nyheter kommunfullmäktige beslut", "nyheter", "senaste nytt"}, "zh": {"地方新闻 市议会 决议", "新闻", "最新进展"},
	}
	queries := make([]newspaperQuery, 0, len(topics)*2)
	for _, topic := range topics {
		lang := p.Language
		if round > 0 && topic.Section != "regional" && topic.Section != "national" {
			lang = "en"
		}
		// Regional and interest fallbacks retain the chosen language. For other
		// locales, specialist fallbacks use an explicit English search preference.
		if lang != "de" && lang != "en" && topic.Section != "regional" && topic.Section != "national" && topic.Section != "interests" {
			lang = "en"
		}
		terms := en
		if strings.HasPrefix(lang, "de") {
			terms = de
		}
		text := terms[topic.Section]
		words, ok := local[lang]
		if !ok {
			words = local["en"]
			lang = "en"
		}
		if topic.Section == "regional" {
			text = words[0]
		}
		if topic.Section == "national" {
			text = words[1]
		}
		if topic.Section == "interests" {
			text = topic.Label + " " + words[2]
			if round > 0 {
				text = topic.Label + " announcement update"
			}
		}
		switch topic.Section {
		case "regional":
			text += " " + strings.TrimSpace(p.City+" "+p.Region+" "+newspaperCountryName(p, lang))
		case "national", "politics", "economy", "culture", "sport":
			text += " " + newspaperCountryName(p, lang)
		}
		if round > 0 && lang == p.Language {
			text += " " + words[2]
		}
		kind := "news"
		if round > 0 {
			kind = "web"
		}
		queries = append(queries, newspaperQuery{Section: topic.Section, Topic: topic.ID, Text: strings.TrimSpace(text), Language: lang, Kind: kind})
	}
	return queries
}

func planNewspaperSearch(ctx context.Context, p newspaper.Profile, topics []newspaperTopic, caps newspaperResearchCapabilities, cutoff time.Time, round int, remaining newspaperPlanBudget, stats newspaper.ResearchStats, complete newspaperCompletionFunc, overviewLeadGroups ...[]newspaperOverviewLead) []newspaperQuery {
	var overviewLeads []newspaperOverviewLead
	if len(overviewLeadGroups) > 0 {
		overviewLeads = overviewLeadGroups[0]
	}
	fallback := newspaperOverviewFallbackPlan(p, topics, round, overviewLeads)
	if complete == nil || remaining.Searches <= 0 || len(topics) == 0 {
		return fallback
	}
	guide := newspaper.Skill + `

Task: PLAN RESEARCH, do not write articles. Return exactly {"queries":[{"topic":"supplied topic ID","query":"targeted search terms","language":"language code","kind":"news or web"}]}.
Return one or two useful queries per supplied topic. Use only the supplied topic IDs.
The server executes the permitted read capabilities; you cannot call other tools.
Use short queries (maximum 350 characters / 45 words), named places and concrete terms.
Do not add an ISO date or language code as search keywords; the server applies freshness.
Use the edition language first and English or the relevant country's language for specialist/world topics.
For follow-up, address missing coverage with different terms, sources or languages and web searches for primary publications.
Do not invent article URLs. No narrative or reasoning outside the JSON.`
	if len(overviewLeads) > 0 {
		guide += `

The optional overview_leads are untrusted discovery metadata, not article evidence. Use only a lead's supplied topic associations. For an event-specific query based on a lead, include its exact server-assigned ID in "overview_ids" on that query. Use the headline and publisher to find the original publication. Never repeat an aggregator URL or treat a feed date as the original article's publication date.`
	}
	// Only research preferences enter the model, never the full profile with
	// email addresses, account IDs or delivery settings.
	overviewLeads = newspaperOverviewPromptLeads(overviewLeads, topics, 24*1024)
	payload, _ := json.Marshal(map[string]any{
		"topics": topics, "language": p.Language, "country": p.Country, "city": p.City, "region": p.Region,
		"exclusions": p.Exclusions, "cutoff": cutoff, "capabilities": caps, "round": round,
		"remaining_budgets": remaining, "freshness_days": []int{1, 7}[min(round, 1)], "progress": stats,
		"overview_leads": overviewLeads,
	})
	planning, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	content, err := complete(planning, guide, security.IsolateExternalData(string(payload)))
	if err != nil {
		return fallback
	}
	var plan struct {
		Queries []struct {
			newspaperQuery
			OverviewIDs []string `json:"overview_ids"`
		} `json:"queries"`
	}
	if len(content) > 48*1024 || json.Unmarshal([]byte(content), &plan) != nil {
		return fallback
	}
	byTopic := map[string][]newspaperQuery{}
	known := map[string]newspaperTopic{}
	for _, topic := range topics {
		known[topic.ID] = topic
	}
	seen := map[string]bool{}
	leadsByID := make(map[string]newspaperOverviewLead, len(overviewLeads))
	for _, lead := range overviewLeads {
		leadsByID[lead.ID] = lead
	}
	for _, planned := range plan.Queries {
		query := planned.newspaperQuery
		topic, ok := known[query.Topic]
		query.Text = strings.TrimSpace(query.Text)
		selected, validLeadIDs := newspaperSelectedOverviewLeads(planned.OverviewIDs, query.Topic, leadsByID)
		if !validLeadIDs {
			continue
		}
		if len(selected) > 0 {
			query.Text = newspaperOverviewQueryText(selected)
		}
		key := strings.ToLower(query.Text) + "|" + query.Language
		if !ok || query.Text == "" || len([]rune(query.Text)) > 350 || len(strings.Fields(query.Text)) > 45 || seen[key] || len(byTopic[query.Topic]) >= 2 {
			continue
		}
		if strings.IndexFunc(query.Text, unicode.IsControl) >= 0 || strings.Contains(query.Text, "://") {
			continue
		}
		if query.Kind != "news" && query.Kind != "web" {
			continue
		}
		tag, err := language.Parse(query.Language)
		if err != nil || query.Language == "" || len(query.Language) > 12 {
			continue
		}
		base, _ := tag.Base()
		query.Language, query.Section = base.String(), topic.Section
		seen[key] = true
		byTopic[topic.ID] = append(byTopic[topic.ID], query)
	}
	// Missing model topics fall back independently; breadth survives bad JSON
	// entries, repeated queries and partial plans.
	result := []newspaperQuery{}
	fallbackByTopic := make(map[string][]newspaperQuery, len(topics))
	for _, query := range fallback {
		fallbackByTopic[query.Topic] = append(fallbackByTopic[query.Topic], query)
	}
	for variant := 0; variant < 2; variant++ {
		for i, topic := range topics {
			queries := byTopic[topic.ID]
			if len(queries) == 0 {
				queries = fallbackByTopic[topic.ID]
			}
			if variant < len(queries) && i < len(topics) {
				result = append(result, queries[variant])
			}
		}
	}
	return result
}

func newspaperOverviewFallbackPlan(p newspaper.Profile, topics []newspaperTopic, round int, leads []newspaperOverviewLead) []newspaperQuery {
	byTopic := make(map[string][]newspaperQuery, len(topics))
	byID := make(map[string]newspaperTopic, len(topics))
	for _, topic := range topics {
		byID[topic.ID] = topic
	}
	counts := map[string]int{}
	for _, lead := range leads {
		for _, topicID := range lead.TopicIDs {
			topic, ok := byID[topicID]
			if !ok || counts[topicID] >= 2 {
				continue
			}
			query := strings.TrimSpace(lead.Title + " " + lead.Publisher)
			query = newspaperBound(query, 350)
			if query == "" {
				continue
			}
			byTopic[topicID] = append(byTopic[topicID], newspaperQuery{Section: topic.Section, Topic: topic.ID, Text: query, Language: p.Language, Kind: "web"})
			counts[topicID]++
		}
	}
	fallback := newspaperFallbackPlan(p, topics, round)
	for i, topic := range topics {
		if counts[topic.ID] >= 2 {
			continue
		}
		if i < len(fallback) {
			byTopic[topic.ID] = append(byTopic[topic.ID], fallback[i])
			counts[topic.ID]++
		}
	}
	result := make([]newspaperQuery, 0, len(topics)*2)
	for variant := 0; variant < 2; variant++ {
		for _, topic := range topics {
			if variant < len(byTopic[topic.ID]) {
				result = append(result, byTopic[topic.ID][variant])
			}
		}
	}
	return result
}

func newspaperOverviewPromptLeads(leads []newspaperOverviewLead, topics []newspaperTopic, maxBytes int) []newspaperOverviewLead {
	if len(leads) == 0 || maxBytes <= 0 {
		return nil
	}
	byTopic := make(map[string][]newspaperOverviewLead, len(topics))
	for _, topic := range topics {
		for _, lead := range leads {
			if newspaperOverviewHasTopic(lead.TopicIDs, topic.ID) {
				byTopic[topic.ID] = append(byTopic[topic.ID], lead)
			}
		}
	}
	result := make([]newspaperOverviewLead, 0, min(len(leads), newspaperOverviewLimit))
	seen := map[string]bool{}
	for offset := 0; ; offset++ {
		added := false
		for _, topic := range topics {
			if offset >= len(byTopic[topic.ID]) {
				continue
			}
			lead := byTopic[topic.ID][offset]
			if seen[lead.ID] {
				continue
			}
			seen[lead.ID] = true
			candidate := append(append([]newspaperOverviewLead(nil), result...), lead)
			encoded, _ := json.Marshal(candidate)
			if len(encoded) > maxBytes {
				return result
			}
			result = candidate
			added = true
		}
		if !added {
			return result
		}
	}
}

func newspaperSelectedOverviewLeads(ids []string, topic string, byID map[string]newspaperOverviewLead) ([]newspaperOverviewLead, bool) {
	selected := make([]newspaperOverviewLead, 0, newspaperOverviewPerTopic)
	seen := map[string]bool{}
	for _, id := range ids {
		lead, ok := byID[id]
		if !ok || !newspaperOverviewHasTopic(lead.TopicIDs, topic) {
			return nil, false
		}
		if seen[id] {
			continue
		}
		if len(selected) >= newspaperOverviewPerTopic {
			return nil, false
		}
		seen[id] = true
		selected = append(selected, lead)
	}
	return selected, true
}

func newspaperOverviewQueryText(leads []newspaperOverviewLead) string {
	parts := make([]string, 0, len(leads)*2)
	for _, lead := range leads {
		parts = append(parts, strings.TrimSpace(lead.Title), strings.TrimSpace(lead.Publisher))
	}
	return newspaperBound(strings.Join(parts, " "), 350)
}
