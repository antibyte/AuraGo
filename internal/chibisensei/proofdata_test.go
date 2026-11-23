package chibisensei

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var proofLabels = []string{"account", "billing", "delivery", "technical"}

var proofGoals = map[string]string{
	RecipeAssistant:  "Answer guests of the Lantern Corner café in one or two friendly sentences.",
	RecipeSpecialist: "Sort customer messages into account, billing, delivery or technical.",
	RecipeFree:       "Write logbook entries in the voice of the Gull Point lighthouse keeper.",
}

// proofDefaultRecipe is the recipe of each profile's proof run (contract, "Proof runs" step 6).
func proofDefaultRecipe(profileID string) string {
	if profileID == "gemma3-270m" {
		return RecipeSpecialist
	}
	return RecipeAssistant
}

type proofItem struct {
	name, base, temp, sweet, sizes, when, tip, other string
	price                                            int
}

func proofItems() []proofItem {
	adjectives := []string{"Amber", "Copper", "Misty", "Velvet", "Harbor"}
	nouns := []string{"Fizz", "Latte", "Cocoa", "Tonic"}
	bases := []string{"oat milk", "black tea", "pressed apple juice", "dark chocolate", "roasted barley",
		"sparkling water", "honey and lemon"}
	temps := []string{"hot", "iced", "warm", "chilled"}
	sweets := []string{"not sweet at all", "lightly sweet", "quite sweet"}
	sizes := []string{"small and large cups", "one standard cup", "small, medium and large cups"}
	whens := []string{"all day", "only in the morning", "from noon until closing", "on weekends"}
	tips := []string{"sip it slowly while it is fresh", "pair it with a slice of seed cake",
		"ask for a cinnamon stick on top", "enjoy it at the window seats by the harbor lights",
		"add a pinch of sea salt if you like a stronger taste"}
	items := make([]proofItem, 20)
	for i := range items {
		items[i] = proofItem{name: adjectives[i/4] + " " + nouns[i%4], base: bases[i%7], temp: temps[(i/2)%4],
			sweet: sweets[i%3], sizes: sizes[(i+1)%3], when: whens[(i/3)%4], tip: tips[i%5], price: 3 + (i*7)%6}
	}
	for i := range items {
		items[i].other = items[(i+5)%20].name
	}
	return items
}

var proofAssistantQA = []func(it proofItem) (string, string){
	func(it proofItem) (string, string) {
		return "What is in the " + it.name + "?", "The " + it.name + " is made with " + it.base + ", and it is " + it.sweet + "."
	},
	func(it proofItem) (string, string) {
		return "How much does the " + it.name + " cost?", fmt.Sprintf("The %s costs %d coins at Lantern Corner.", it.name, it.price)
	},
	func(it proofItem) (string, string) {
		return "Is the " + it.name + " served hot or cold?", "We serve the " + it.name + " " + it.temp + "."
	},
	func(it proofItem) (string, string) {
		return "Can you recommend something like the " + it.name + "?", "If you enjoy the " + it.name + ", try the " + it.other + " next time; many guests like both."
	},
	func(it proofItem) (string, string) {
		return "Is the " + it.name + " sweet?", "The " + it.name + " is " + it.sweet + "."
	},
	func(it proofItem) (string, string) {
		return "What sizes does the " + it.name + " come in?", "You can order the " + it.name + " in " + it.sizes + "."
	},
	func(it proofItem) (string, string) {
		return "Can I take the " + it.name + " away?", "Yes, we pack the " + it.name + " in a paper cup to take away."
	},
	func(it proofItem) (string, string) {
		return "When can I order the " + it.name + "?", "The " + it.name + " is available " + it.when + "."
	},
	func(it proofItem) (string, string) {
		return "Do you have a tip for enjoying the " + it.name + "?", "For the " + it.name + ", " + it.tip + "."
	},
	func(it proofItem) (string, string) {
		return "Does the " + it.name + " contain " + it.base + "?", "Yes, " + it.base + " is the main ingredient of the " + it.name + "."
	},
}

const proofAssistantSystem = "You are the friendly helper of Lantern Corner, a small café in a story world. Answer in one or two short sentences."

const proofSpecialistSystem = "Classify the customer message as account, billing, delivery or technical. Answer with the label only."

var proofObjects = []string{"the garden lamp", "my winter boots", "the tea sampler", "a set of brushes", "the travel mug",
	"my puzzle box", "the desk plant", "a pair of headphones", "the wool scarf", "my board game"}

var proofTemplates = map[string][]string{
	"account": {"I forgot the password for the account I used to buy %s.",
		"Please change the email address on my profile; I ordered %s last week.",
		"How do I delete my customer account after returning %s?",
		"My account is locked since I tried to review %s.",
		"Can I merge two customer accounts? One of them has the order for %s."},
	"billing": {"I was charged twice for %s.", "Where can I download the invoice for %s?",
		"The price of %s changed after I paid. Can you refund the difference?",
		"My card was declined when I tried to pay for %s.", "Please explain the extra fee on my bill for %s."},
	"delivery": {"My parcel with %s has not arrived yet.", "The courier left %s at the wrong door.",
		"Can I change the delivery address for %s?", "The tracking page for %s has not updated for four days.",
		"The box with %s arrived damaged."},
	"technical": {"The shop app crashes when I open the page for %s.", "The product photos of %s do not load in my browser.",
		"I cannot scan the QR code on the manual for %s.", "The checkout button does nothing when %s is in my basket.",
		"The website shows an error message when I search for %s."},
}

var (
	proofWeather = []string{"The morning began with a thin fog that lifted slowly over the cliffs.",
		"A steady west wind pushed grey clouds across the sky all day.", "Bright sunshine warmed the stone steps before noon.",
		"Light rain tapped on the lantern room windows until the evening.", "A cold north wind made the old door rattle in its frame.",
		"The sky stayed clear and pale blue from sunrise to sunset.", "Heavy clouds gathered in the afternoon, but no storm arrived."}
	proofSea = []string{"The sea was calm, and the fishing boats left the harbor early.",
		"Long swells rolled in from the open water and broke white on the rocks.",
		"The tide ran unusually low, showing the green weed on the lower ledges.",
		"Small choppy waves kept the ferry waiting at the pier for an hour.",
		"The water was so still that the cliffs were mirrored near the shore."}
	proofVisitors = []string{"No visitors climbed the path today.",
		"A family of four came to see the lamp and asked many questions about the clockwork.",
		"The postman brought two letters and a parcel of lamp wicks.",
		"A painter set up an easel near the fence and stayed until the light changed.",
		"Two hikers rested on the bench and shared their apples with the keeper.",
		"A school class visited and counted all one hundred and twelve steps aloud.",
		"The harbor master stopped by to compare notes on the tides.",
		"A birdwatcher reported a pair of puffins on the southern ledge.",
		"An old sailor told a long story about a winter storm from his youth."}
	proofTasks = []string{"The keeper polished the brass fittings until they shone.",
		"The lens was cleaned with a soft cloth and a little vinegar water.",
		"Fresh oil was carried up in two trips and the reservoir was filled.",
		"The weather vane was oiled because it had started to squeak.",
		"Loose paint was scraped from the railing and a new coat was applied.",
		"The logbook from last month was copied neatly for the archive.",
		"The clockwork that turns the lamp was wound and checked for wear.",
		"The rain gutter was cleared of leaves and gull feathers.",
		"New putty was pressed around a draughty window in the stairwell.",
		"The signal flags were counted, folded and stored in the dry chest.",
		"The garden by the cottage was weeded and the beans were watered."}
	proofClosings = []string{"The lamp was lit at dusk and burned steadily through the night.",
		"At sunset the lamp was lit, and the keeper watched the first ships pass.",
		"By evening everything was in order, and the beam swept the water as usual.",
		"The night watch was quiet, with only the sound of the waves below."}
)

// proofRows returns the 200 deterministic rows of a recipe.
func proofRows(recipe string) []RowInput {
	var out []RowInput
	switch recipe {
	case RecipeAssistant:
		for _, it := range proofItems() {
			for _, qa := range proofAssistantQA {
				q, a := qa(it)
				out = append(out, RowInput{Messages: []Message{{"system", proofAssistantSystem}, {"user", q}, {"assistant", a}}})
			}
		}
	case RecipeSpecialist:
		for _, obj := range proofObjects {
			for ti := 0; ti < 5; ti++ {
				for _, label := range proofLabels {
					out = append(out, RowInput{Messages: []Message{{"system", proofSpecialistSystem},
						{"user", fmt.Sprintf(proofTemplates[label][ti], obj)}, {"assistant", label}}})
				}
			}
		}
	case RecipeFree:
		for d := 1; d <= 200; d++ {
			text := fmt.Sprintf("Day %d at the Gull Point lighthouse. %s %s %s %s The barometer read %d millibars at noon, and the keeper counted %d gulls on the roof of the oil store. %s",
				d, proofWeather[d%7], proofSea[d%5], proofVisitors[d%9], proofTasks[d%11], 990+(d*13)%40, 3+(d*7)%23, proofClosings[d%4])
			out = append(out, RowInput{Text: &text})
		}
	}
	return out
}

func proofJSONL(rows []RowInput) []byte {
	var buf bytes.Buffer
	for _, r := range rows {
		row := Row{Kind: rowKindChat, Messages: r.Messages}
		if r.Text != nil {
			row = Row{Kind: rowKindText, Text: r.Text}
		}
		buf.Write(ExportLine(row))
	}
	return buf.Bytes()
}

// readProofRows reads the committed proof file of a recipe.
func readProofRows(t *testing.T, recipe string) []RowInput {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "proof", recipe+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []RowInput
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var w struct {
			Messages []Message `json:"messages"`
			Text     *string   `json:"text"`
		}
		if err := json.Unmarshal([]byte(line), &w); err != nil {
			t.Fatal(err)
		}
		out = append(out, RowInput{Messages: w.Messages, Text: w.Text})
	}
	return out
}

func TestProofData(t *testing.T) {
	for _, recipe := range []string{RecipeAssistant, RecipeSpecialist, RecipeFree} {
		rows := proofRows(recipe)
		if len(rows) != 200 {
			t.Fatalf("%s: %d rows", recipe, len(rows))
		}
		hashes, labels, tokens := map[string]bool{}, map[string]int{}, 0
		for i, r := range rows {
			if is := ValidateRowInput(r); len(is) > 0 {
				t.Fatalf("%s row %d: %+v", recipe, i, is)
			}
			kind, text := rowKindChat, ""
			if r.Text != nil {
				kind, text = rowKindText, *r.Text
			}
			if p := StructureProblems(kind, r.Messages, recipe); len(p) > 0 {
				t.Fatalf("%s row %d: %v", recipe, i, p)
			}
			h := ContentHash(kind, r.Messages, text)
			if hashes[h] {
				t.Fatalf("%s row %d duplicates an earlier row", recipe, i)
			}
			hashes[h] = true
			tokens += EstimateTokens(r.Messages, text)
			if recipe == RecipeSpecialist {
				labels[LastAssistant(r.Messages)]++
			}
		}
		if recipe == RecipeSpecialist {
			for _, l := range proofLabels {
				if labels[l] != 50 {
					t.Fatalf("label %s has %d rows", l, labels[l])
				}
			}
		}
		if recipe == RecipeFree && tokens < freeMinTokens {
			t.Fatalf("free rows hold %d estimated tokens, want >= %d", tokens, freeMinTokens)
		}
		got := proofJSONL(rows)
		path := filepath.Join("testdata", "proof", recipe+".jsonl")
		if *updateGolden {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if want, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s differs from the generator (err %v); run with -update", path, err)
		}
	}
}

// proofRun is a project filled with the proof rows of a recipe, split, planned and checked.
type proofRun struct {
	proj   Project
	prof   *Profile
	plan   Plan
	checks CheckResult
	flags  map[int64]Flag
}

func proofPrepare(t *testing.T, st *Store, c *Catalog, profileID, recipe, preset string, timeout int) proofRun {
	t.Helper()
	ctx := context.Background()
	prof, ok := c.Get(profileID)
	if !ok {
		t.Fatalf("%q is not a profile", profileID)
	}
	in := ProjectInput{Name: "Proof " + profileID, Recipe: recipe, GoalText: proofGoals[recipe], ProfileID: profileID}
	if recipe == RecipeSpecialist {
		in.TaskKind, in.Labels = "classify", proofLabels
	}
	proj, err := st.CreateProject(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	rows := readProofRows(t, recipe)
	res, err := st.AddRows(ctx, proj.ID, rows, AddOptions{Source: "file", BatchKind: "import", MaxRows: 50000, Strict: true})
	if err != nil || res.Added != len(rows) {
		t.Fatalf("add rows: added %d of %d, err %v", res.Added, len(rows), err)
	}
	crows, proj, err := st.LoadCheckRows(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	srows := make([]SplitRow, 0, len(crows))
	for _, r := range crows {
		srows = append(srows, SplitRow{ID: r.ID, Split: r.Split, Label: r.Label, EstTokens: r.EstTokens})
	}
	split := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	plan, err := ComputePlan(PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split, Preset: preset,
		RemoteTimeoutMinutes: timeout, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	checks, flags := RunChecks(CheckInput{Project: proj, Profile: prof, Rows: crows, Split: split,
		MaxSeqLen: plan.Hyper.MaxSeqLen}, time.Now().UTC())
	return proofRun{proj: proj, prof: prof, plan: plan, checks: checks, flags: flags}
}

func (r proofRun) blocks() []string {
	var blocks []string
	for _, ch := range r.checks.Checks {
		if ch.Level == "block" {
			blocks = append(blocks, ch.ID)
		}
	}
	return blocks
}

// TestProofDataPassesChecks: every proof dataset is ready for training under its recipe.
func TestProofDataPassesChecks(t *testing.T) {
	c := mustCatalog(t)
	for _, tc := range []struct{ profile, recipe string }{
		{"gemma3-270m", RecipeSpecialist}, {"qwen35-0p8b", RecipeAssistant}, {"lfm25-1p2b", RecipeFree}} {
		st, err := OpenStore(filepath.Join(t.TempDir(), "chibi_sensei.db"))
		if err != nil {
			t.Fatal(err)
		}
		run := proofPrepare(t, st, c, tc.profile, tc.recipe, "quick", 60)
		st.Close()
		if b := run.blocks(); len(b) > 0 || !run.checks.Ready {
			t.Errorf("%s: blocking checks %v (ready %v)", tc.recipe, b, run.checks.Ready)
		}
	}
}
