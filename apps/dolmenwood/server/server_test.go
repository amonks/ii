package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/engine"
)

func setupTest(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	d, err := db.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	srv := New(d)
	return srv, d
}

func parseStatSpeedValues(t *testing.T, body string) map[string]string {
	t.Helper()
	segments := strings.Split(body, "bg-stone-50 rounded p-1")
	if len(segments) < 2 {
		t.Fatalf("expected speed boxes in stats card, got %d segments", len(segments))
	}
	values := map[string]string{}
	for _, seg := range segments {
		labelIdx := strings.Index(seg, "text-stone-500")
		if labelIdx == -1 {
			continue
		}
		labelStart := strings.Index(seg[labelIdx:], ">")
		if labelStart == -1 {
			continue
		}
		labelStart += labelIdx + 1
		labelEnd := strings.Index(seg[labelStart:], "<")
		if labelEnd == -1 {
			continue
		}
		label := strings.TrimSpace(seg[labelStart : labelStart+labelEnd])
		valueIdx := strings.Index(seg, "font-bold")
		if valueIdx == -1 {
			continue
		}
		valueStart := strings.Index(seg[valueIdx:], ">")
		if valueStart == -1 {
			continue
		}
		valueStart += valueIdx + 1
		valueEnd := strings.Index(seg[valueStart:], "<")
		if valueEnd == -1 {
			continue
		}
		value := strings.TrimSpace(seg[valueStart : valueStart+valueEnd])
		value = strings.TrimSuffix(value, "&#39;")
		if label != "" {
			values[label] = value
		}
	}
	return values
}

func TestGetIndex(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Dolmenwood") {
		t.Error("response should contain 'Dolmenwood'")
	}
}

func TestIndexShowsClassAndKindredOptions(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	for _, class := range engine.ClassNames() {
		if !strings.Contains(body, fmt.Sprintf("value=\"%s\"", class)) {
			t.Errorf("expected class option %q", class)
		}
	}
	for _, kindred := range engine.KindredNames() {
		if !strings.Contains(body, fmt.Sprintf("value=\"%s\"", kindred)) {
			t.Errorf("expected kindred option %q", kindred)
		}
	}
}

func TestIndexShowsRetainerEmployerLabel(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:      "Alder",
		Class:     "Knight",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 10,
		HPMax:     10,
	}
	if err := d.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	retainer := &db.Character{
		Name:      "Bram",
		Class:     "Hunter",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 6,
		HPMax:     6,
	}
	if err := d.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}
	contract := &db.RetainerContract{
		EmployerID: employer.ID,
		RetainerID: retainer.ID,
		Active:     true,
	}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	expected := fmt.Sprintf("Retainer of %s", employer.Name)
	if !strings.Contains(body, expected) {
		t.Errorf("expected retainer label %q", expected)
	}
}

func TestDeleteCharacter(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Doomed", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add some items and companions to ensure cascade
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Longsword", Quantity: 1})
	d.CreateCompanion(&db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9})

	form := url.Values{}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/delete/", ch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should redirect to index
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusSeeOther, w.Body.String())
	}

	// Character should be gone
	chars, _ := d.ListCharacters()
	for _, c := range chars {
		if c.ID == ch.ID {
			t.Fatal("character should have been deleted")
		}
	}

	// Items should be gone
	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	// Companions should be gone
	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 0 {
		t.Errorf("expected 0 companions, got %d", len(comps))
	}
}

func TestCreateCharacter(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	form := url.Values{}
	form.Set("name", "Sir Galahad")
	form.Set("str", "16")
	form.Set("dex", "10")
	form.Set("con", "14")
	form.Set("int", "9")
	form.Set("wis", "12")
	form.Set("cha", "13")
	form.Set("class", "Knight")
	form.Set("kindred", "Human")
	form.Set("hp_max", "8")
	form.Set("alignment", "Lawful")

	req := httptest.NewRequest("POST", "/characters/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	loc := w.Header().Get("Location")
	if loc != "1/" {
		t.Errorf("Location = %q, want %q", loc, "1/")
	}
}

func TestCreateCharacterRejectsInvalidClass(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	form := url.Values{}
	form.Set("name", "Sir Galahad")
	form.Set("str", "16")
	form.Set("dex", "10")
	form.Set("con", "14")
	form.Set("int", "9")
	form.Set("wis", "12")
	form.Set("cha", "13")
	form.Set("class", "NotAClass")
	form.Set("kindred", "Human")
	form.Set("hp_max", "8")

	req := httptest.NewRequest("POST", "/characters/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateCharacterRejectsInvalidKindred(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	form := url.Values{}
	form.Set("name", "Sir Galahad")
	form.Set("str", "16")
	form.Set("dex", "10")
	form.Set("con", "14")
	form.Set("int", "9")
	form.Set("wis", "12")
	form.Set("cha", "13")
	form.Set("class", "Knight")
	form.Set("kindred", "NotAKindred")
	form.Set("hp_max", "8")

	req := httptest.NewRequest("POST", "/characters/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetCharacterSheet(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Sir Galahad", Class: "Knight", Kindred: "Human",
		Level: 1, STR: 16, DEX: 10, CON: 14, INT: 9, WIS: 12, CHA: 13,
		HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Sir Galahad") {
		t.Error("response should contain character name")
	}
	if !strings.Contains(body, "Advancement") {
		t.Error("response should contain advancement card")
	}
	if !strings.Contains(body, "Knight Advancement") {
		t.Error("response should contain advancement table title")
	}
	if !strings.Contains(body, "Level") {
		t.Error("response should contain advancement headers")
	}
	if !strings.Contains(body, "1d8") {
		t.Error("response should contain advancement table values")
	}
	advancementIndex := strings.Index(body, "Knight Advancement")
	notesIndex := strings.Index(body, "Notes")
	if advancementIndex == -1 || notesIndex == -1 {
		t.Fatalf("expected advancement and notes sections for ordering check")
	}
	if !(advancementIndex < notesIndex) {
		t.Errorf("expected advancement before notes, got advancement=%d notes=%d", advancementIndex, notesIndex)
	}
}

func TestTraitsCardShowsKindredAndClassTraits(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 5, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	for _, label := range []string{
		"Traits",
		"Kindred Traits",
		"Class Traits",
		"Decisiveness",
		"Monster Slayer",
	} {
		if !strings.Contains(body, label) {
			t.Errorf("response should contain %q", label)
		}
	}
}

func TestStatsCardTraitsAppearAfterStats(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 5, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	statsIndex := strings.Index(body, "id=\"stats\"")
	traitsIndex := strings.Index(body, "id=\"traits\"")
	knighthoodIndex := strings.Index(body, "Knighthood")
	if statsIndex == -1 || traitsIndex == -1 || knighthoodIndex == -1 {
		t.Fatalf("expected stats, traits, and knighthood markers in response")
	}
	if !(statsIndex < traitsIndex && traitsIndex < knighthoodIndex) {
		t.Errorf("expected ordering stats card -> traits card -> knighthood trait, got stats=%d traits=%d knighthood=%d", statsIndex, traitsIndex, knighthoodIndex)
	}
}

func TestCardDisclosureMarkup(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "card-disclosure-summary-body") {
		t.Error("expected card disclosure summary wrapper")
	}
	if !strings.Contains(body, "card-disclosure-toggle") {
		t.Error("expected card disclosure toggle bar")
	}
	if !strings.Contains(body, "card-disclosure-body") {
		t.Error("expected card disclosure body")
	}
}

func TestStatsCardShowsAlignmentSection(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name:       "Test",
		Class:      "Knight",
		Kindred:    "Human",
		Level:      1,
		HPCurrent:  8,
		HPMax:      8,
		Alignment:  "Lawful",
		Background: "Noble",
		Liege:      "Duke Maldric",
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Background & Alignment") {
		t.Error("response should include background + alignment section header")
	}
	for _, text := range []string{"Alignment", "Background", "Allegiance", "Lawful", "Noble", "Duke Maldric"} {
		if !strings.Contains(body, text) {
			t.Errorf("response should contain %q", text)
		}
	}
}

func TestStatsCardShowsSpeedBreakdown(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	for _, label := range []string{
		"Scores",
		"Saves",
		"Speed",
		"Encounter",
		"Exploration (unknown)",
		"Exploration (mapped)",
		"Running",
		"Overland",
	} {
		if !strings.Contains(body, label) {
			t.Errorf("response should contain %q", label)
		}
	}
	scoresIndex := strings.Index(body, "Scores")
	savesIndex := strings.Index(body, "Saves")
	speedIndex := strings.Index(body, "Speed")
	if scoresIndex == -1 || savesIndex == -1 || speedIndex == -1 {
		t.Fatalf("expected labels to be present for ordering check")
	}
	if !(scoresIndex < savesIndex && savesIndex < speedIndex) {
		t.Errorf("expected ordering Scores -> Saves -> Speed, got indexes scores=%d saves=%d speed=%d", scoresIndex, savesIndex, speedIndex)
	}
	values := parseStatSpeedValues(t, body)
	if len(values) == 0 {
		t.Fatalf("expected speed values to be parsed")
	}
	for label, value := range map[string]string{"Encounter": "40&#39; / rnd", "Exploration (unknown)": "120&#39; / turn", "Exploration (mapped)": "400&#39; / turn", "Running": "120&#39; / rnd", "Overland": "8 tp/day"} {
		if values[label] != value {
			t.Errorf("speed %s = %q, want %q", label, values[label], value)
		}
	}
}

func TestCharacterSheetShowsBirthdayAndMoonSign(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Birthday Test", Class: "Knight", Kindred: "Human",
		Level: 1, STR: 12, DEX: 10, CON: 12, INT: 10, WIS: 10, CHA: 10,
		HPCurrent: 8, HPMax: 8,
		BirthdayMonth: "Grimvold",
		BirthdayDay:   18,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "birthday_month") {
		t.Error("response should include birthday month selector")
	}
	if !strings.Contains(body, "birthday_day") {
		t.Error("response should include birthday day selector")
	}
	if !strings.Contains(body, "Moon Sign") {
		t.Error("response should include moon sign section")
	}
	if !strings.Contains(body, "Grinning moon") {
		t.Error("response should include moon sign name")
	}
}

func TestUpdateBirthday(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Birthday Update", Class: "Knight", Kindred: "Human",
		Level: 1, STR: 12, DEX: 10, CON: 12, INT: 10, WIS: 10, CHA: 10,
		HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("birthday_month", "Grimvold")
	form.Set("birthday_day", "18")

	req := httptest.NewRequest("POST", "/characters/1/birthday/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	got, _ := d.GetCharacter(ch.ID)
	if got.BirthdayMonth != "Grimvold" {
		t.Errorf("BirthdayMonth = %q, want %q", got.BirthdayMonth, "Grimvold")
	}
	if got.BirthdayDay != 18 {
		t.Errorf("BirthdayDay = %d, want %d", got.BirthdayDay, 18)
	}
}

func TestACDerivedFromEquippedItems(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, DEX: 10, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        "Chainmail",
		Quantity:    1,
		Location:    "equipped",
	})
	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        "Shield",
		Quantity:    1,
		Location:    "equipped",
	})

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, ">15<") {
		t.Error("response should show AC 15 for chainmail + shield")
	}
	if !strings.Contains(body, "Chainmail") {
		t.Error("response should show armor name 'Chainmail'")
	}
	if !strings.Contains(body, "(+14)") {
		t.Error("response should show chainmail AC contribution (+14)")
	}
	if !strings.Contains(body, "Shield") {
		t.Error("response should show shield name 'Shield'")
	}
	if !strings.Contains(body, "(+1)") {
		t.Error("response should show shield AC contribution (+1)")
	}
}

func TestWeaponDamageInStatBlock(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, DEX: 10, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add longsword as equipped weapon
	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        "Longsword",
		Quantity:    1,
		Location:    "equipped",
	})

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Longsword") {
		t.Error("response should show equipped weapon name")
	}
	if !strings.Contains(body, "1d8") {
		t.Error("response should show weapon damage 1d8")
	}
}

func TestAddTinyItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "tiny lock of hair")
	form.Set("location", "stowed")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "lock of hair" {
		t.Errorf("Name = %q, want %q", items[0].Name, "lock of hair")
	}
	if !items[0].IsTiny {
		t.Error("expected item to be marked tiny")
	}
}

func TestMoveItemToHorse(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	item := &db.Item{
		CharacterID: ch.ID,
		Name:        "Rope",
		Quantity:    1,
		Location:    "stowed",
	}
	d.CreateItem(item)

	// Move to horse
	form := url.Values{}
	form.Set("location", "horse")
	req := httptest.NewRequest("POST", "/characters/1/items/1/update/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if items[0].Location != "horse" {
		t.Errorf("Location = %q, want %q", items[0].Location, "horse")
	}
}

func TestAddItemToHorse(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "Tent")
	form.Set("location", "horse")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Location != "horse" {
		t.Errorf("Location = %q, want %q", items[0].Location, "horse")
	}
}

func TestAddCompanionByBreed(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "Bessie")
	form.Set("breed", "Mule")
	req := httptest.NewRequest("POST", "/characters/1/companions/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 1 {
		t.Fatalf("got %d companions, want 1", len(comps))
	}
	got := comps[0]
	if got.Name != "Bessie" {
		t.Errorf("Name = %q, want %q", got.Name, "Bessie")
	}
	if got.Breed != "Mule" {
		t.Errorf("Breed = %q, want %q", got.Breed, "Mule")
	}
	// Mule: HP 9 (stored), AC/Speed/Load derived from breed
	if got.HPMax != 9 {
		t.Errorf("HPMax = %d, want 9", got.HPMax)
	}
	if got.HPCurrent != 9 {
		t.Errorf("HPCurrent = %d, want 9", got.HPCurrent)
	}
}

func TestCompanionStatsDerivedFromBreed(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add a Charger companion
	form := url.Values{}
	form.Set("name", "Warhorse")
	form.Set("breed", "Charger")
	req := httptest.NewRequest("POST", "/characters/1/companions/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	// Should show breed-derived stats: AC 12, Speed 40, Load 40
	if !strings.Contains(body, "Charger") {
		t.Error("response should contain breed name")
	}
	// Should show saves
	if !strings.Contains(body, "Doom") {
		t.Error("response should show save labels")
	}
	// Charger attack: "2 hooves (+2, 1d6)"
	if !strings.Contains(body, "hooves") {
		t.Error("response should show attack info")
	}
}

func TestAddXP(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("xp_amount", "100")
	form.Set("description", "Quest reward")
	req := httptest.NewRequest("POST", "/characters/1/xp/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify XP was added to character
	got, _ := d.GetCharacter(ch.ID)
	if got.TotalXP != 100 {
		t.Errorf("TotalXP = %d, want 100", got.TotalXP)
	}

	// Verify XP log entry
	xpLog, err := d.ListXPLog(ch.ID)
	if err != nil {
		t.Fatalf("ListXPLog: %v", err)
	}
	if len(xpLog) != 1 {
		t.Fatalf("got %d XP log entries, want 1", len(xpLog))
	}
	if xpLog[0].Amount != 100 {
		t.Errorf("XPLogEntry.Amount = %d, want 100", xpLog[0].Amount)
	}
	if xpLog[0].Description != "Quest reward" {
		t.Errorf("XPLogEntry.Description = %q, want %q", xpLog[0].Description, "Quest reward")
	}

	// Verify audit log
	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "xp_add" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "xp_add")
	}
}

func TestUpdateCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID,
		Name:        "Old Horse",
		Breed:       "Mule",
		HPCurrent:   9,
		HPMax:       9,
	}
	d.CreateCompanion(comp)

	form := url.Values{}
	form.Set("name", "Shadowfax")
	form.Set("hp_current", "7")
	form.Set("hp_max", "12")
	req := httptest.NewRequest("POST", "/characters/1/companions/1/update/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, err := d.ListCompanions(ch.ID)
	if err != nil {
		t.Fatalf("ListCompanions: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("got %d companions, want 1", len(comps))
	}
	got := comps[0]
	if got.Name != "Shadowfax" {
		t.Errorf("Name = %q, want %q", got.Name, "Shadowfax")
	}
	if got.HPCurrent != 7 {
		t.Errorf("HPCurrent = %d, want 7", got.HPCurrent)
	}
	if got.HPMax != 12 {
		t.Errorf("HPMax = %d, want 12", got.HPMax)
	}
}

func TestUndoTransaction(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Create inventory coin item (purse transaction added 50gp)
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp"})

	// Create a transaction to undo
	tx := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          50,
		CoinType:        "gp",
		Description:     "dragon hoard",
		IsFoundTreasure: false,
	}
	d.CreateTransaction(tx)

	// Undo it
	req := httptest.NewRequest("POST", "/characters/1/treasure/1/undo/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify inverse transaction was created
	txs, err := d.ListTransactions(ch.ID)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("got %d transactions, want 2", len(txs))
	}
	// Most recent first
	undo := txs[0]
	if undo.Amount != -50 {
		t.Errorf("undo Amount = %d, want -50", undo.Amount)
	}
	if undo.CoinType != "gp" {
		t.Errorf("undo CoinType = %q, want %q", undo.CoinType, "gp")
	}
	if undo.Description != "undo dragon hoard" {
		t.Errorf("undo Description = %q, want %q", undo.Description, "undo dragon hoard")
	}

	// Verify coins were removed from inventory
	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (coins should be removed)", len(items))
	}
}

func TestUndoFoundTransaction(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		FoundSP: 100,
	}
	d.CreateCharacter(ch)

	tx := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          100,
		CoinType:        "sp",
		Description:     "loot",
		IsFoundTreasure: true,
	}
	d.CreateTransaction(tx)

	req := httptest.NewRequest("POST", "/characters/1/treasure/1/undo/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	got, _ := d.GetCharacter(ch.ID)
	if got.FoundSP != 0 {
		t.Errorf("FoundSP = %d, want 0", got.FoundSP)
	}
}

func TestAddTreasureCreatesCoinItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("entry", "50g dragon hoard")
	form.Set("type", "found")
	req := httptest.NewRequest("POST", "/characters/1/treasure/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Check a consolidated coin item was created
	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "Coins" {
		t.Errorf("item Name = %q, want %q", items[0].Name, "Coins")
	}
	if items[0].Quantity != 50 {
		t.Errorf("item Quantity = %d, want 50", items[0].Quantity)
	}
	if items[0].Notes != "50gp" {
		t.Errorf("item Notes = %q, want %q", items[0].Notes, "50gp")
	}
}

func TestAddTreasureMergesExistingCoinItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Existing consolidated coin item
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 100, Notes: "100gp"})

	form := url.Values{}
	form.Set("entry", "50g more gold")
	form.Set("type", "purse")
	req := httptest.NewRequest("POST", "/characters/1/treasure/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 150 {
		t.Errorf("item Quantity = %d, want 150", items[0].Quantity)
	}
	if items[0].Notes != "150gp" {
		t.Errorf("item Notes = %q, want %q", items[0].Notes, "150gp")
	}
}

func TestUndoTreasureRemovesCoinItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Existing consolidated coin item
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp"})

	// Create a transaction to undo
	tx := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          50,
		CoinType:        "gp",
		Description:     "dragon hoard",
		IsFoundTreasure: false,
	}
	d.CreateTransaction(tx)

	// Undo it
	req := httptest.NewRequest("POST", "/characters/1/treasure/1/undo/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Coin item should be removed (quantity reaches 0)
	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0 (coin item should be removed)", len(items))
	}
}

func TestDeleteCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID,
		Name:        "Old Nag",
		Breed:       "Mule",
		HPCurrent:   9,
		HPMax:       9,
	}
	d.CreateCompanion(comp)

	req := httptest.NewRequest("POST", "/characters/1/companions/1/delete/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 0 {
		t.Errorf("got %d companions, want 0", len(comps))
	}
}

func TestAddItemWithQuantityPrefix(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "5x Preserved Rations")
	form.Set("location", "stowed")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "Preserved Rations" {
		t.Errorf("Name = %q, want %q", items[0].Name, "Preserved Rations")
	}
	if items[0].Quantity != 5 {
		t.Errorf("Quantity = %d, want 5", items[0].Quantity)
	}
}

func TestMoveItemToContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Create a backpack (equipped on character)
	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	// Create a rope (equipped on character)
	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	// Move rope into backpack via move_to
	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Rope ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
}

func TestMoveBundledItemAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2, ContainerID: &backpack.ID}
	d.CreateItem(existing)

	loose := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2}
	d.CreateItem(loose)

	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", loose.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	var torchCount int
	for _, it := range items {
		if it.Name == "Torch" {
			torchCount++
			if it.Quantity != 4 {
				t.Errorf("Torch Quantity = %d, want 4", it.Quantity)
			}
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Torch ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
	if torchCount != 1 {
		t.Fatalf("got %d torch items, want 1", torchCount)
	}
}

func TestMoveItemToCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if items[0].CompanionID == nil || *items[0].CompanionID != comp.ID {
		t.Errorf("Rope CompanionID = %v, want %d", items[0].CompanionID, comp.ID)
	}
}

func TestMoveItemToEquipped(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(rope)

	// Move rope back to equipped
	form := url.Values{}
	form.Set("move_to", "equipped")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID != nil {
				t.Errorf("Rope ContainerID = %v, want nil", it.ContainerID)
			}
			if it.CompanionID != nil {
				t.Errorf("Rope CompanionID = %v, want nil", it.CompanionID)
			}
		}
	}
}

func TestMoveItemToCompanionContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Create a chest on the companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	// Create rope equipped on character
	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	// Move rope into the chest (which is on the companion)
	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", chest.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil || *it.ContainerID != chest.ID {
				t.Errorf("Rope ContainerID = %v, want %d", it.ContainerID, chest.ID)
			}
			if it.CompanionID != nil {
				t.Errorf("Rope CompanionID = %v, want nil (it's in a container, not directly on companion)", it.CompanionID)
			}
		}
	}
}

func TestAddItemToContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	form := url.Values{}
	form.Set("name", "Rope")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Rope ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
}

func TestAddBundledItemAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2, ContainerID: &backpack.ID}
	d.CreateItem(existing)

	form := url.Values{}
	form.Set("name", "Torch")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	var found bool
	for _, it := range items {
		if it.Name == "Torch" {
			found = true
			if it.Quantity != 3 {
				t.Errorf("Torch Quantity = %d, want 3", it.Quantity)
			}
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Torch ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
	if !found {
		t.Fatal("expected torch item to remain")
	}
}

func TestAddBundledItemInLocationAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	existing := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2, Location: "stowed"}
	d.CreateItem(existing)

	form := url.Values{}
	form.Set("name", "Torch")
	form.Set("location", "stowed")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 3 {
		t.Errorf("Torch Quantity = %d, want 3", items[0].Quantity)
	}
	if items[0].Location != "stowed" {
		t.Errorf("Torch Location = %q, want stowed", items[0].Location)
	}
}

func TestAddCoinItemAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp", ContainerID: &backpack.ID}
	d.CreateItem(existing)

	// Adding "50gp" via the item form should be recognized as coins and merge
	form := url.Values{}
	form.Set("name", "50gp")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 { // backpack + merged coins
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.Name == "Coins" {
			if it.Quantity != 100 {
				t.Errorf("Coins Quantity = %d, want 100", it.Quantity)
			}
			if it.Notes != "100gp" {
				t.Errorf("Coins Notes = %q, want %q", it.Notes, "100gp")
			}
		}
	}
}

func TestMoveCoinItemAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp", ContainerID: &backpack.ID}
	d.CreateItem(existing)

	loose := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 30, Notes: "30gp"}
	d.CreateItem(loose)

	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", loose.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 { // backpack + merged coins
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.Name == "Coins" {
			if it.Quantity != 80 {
				t.Errorf("Coins Quantity = %d, want 80", it.Quantity)
			}
			if it.Notes != "80gp" {
				t.Errorf("Coins Notes = %q, want %q", it.Notes, "80gp")
			}
		}
	}
}

func TestAddUnbundledItemDoesNotCombine(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(existing)

	form := url.Values{}
	form.Set("name", "Rope")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	var ropes int
	for _, it := range items {
		if it.Name == "Rope" {
			ropes++
		}
	}
	if ropes != 2 {
		t.Fatalf("got %d ropes, want 2", ropes)
	}
}

func TestSplitCoinsSingleDenomination(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 200, Notes: "200gp"}
	d.CreateItem(coins)

	form := url.Values{}
	form.Set("quantity", "50gp")
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	var onChar, onComp *db.Item
	for i := range items {
		if items[i].Name == "Coins" {
			if items[i].CompanionID != nil {
				onComp = &items[i]
			} else {
				onChar = &items[i]
			}
		}
	}
	if onChar == nil || onChar.Quantity != 150 {
		t.Errorf("coins on character = %v, want qty 150", onChar)
	}
	if onChar != nil && onChar.Notes != "150gp" {
		t.Errorf("coins on character notes = %q, want %q", onChar.Notes, "150gp")
	}
	if onComp == nil || onComp.Quantity != 50 {
		t.Errorf("coins on companion = %v, want qty 50", onComp)
	}
	if onComp != nil && onComp.Notes != "50gp" {
		t.Errorf("coins on companion notes = %q, want %q", onComp.Notes, "50gp")
	}
}

func TestSplitCoinsMultiDenomination(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Single consolidated coin item with mixed denominations
	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 300, Notes: "200gp 100sp"}
	d.CreateItem(coins)

	// Split "100gp 50sp" to companion
	form := url.Values{}
	form.Set("quantity", "100gp 50sp")
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	var onChar, onComp *db.Item
	for i := range items {
		if items[i].Name == "Coins" {
			if items[i].CompanionID != nil {
				onComp = &items[i]
			} else {
				onChar = &items[i]
			}
		}
	}
	if onChar == nil || onChar.Quantity != 150 {
		t.Errorf("coins on character qty = %v, want 150", onChar)
	}
	if onChar != nil && onChar.Notes != "100gp 50sp" {
		t.Errorf("coins on character notes = %q, want %q", onChar.Notes, "100gp 50sp")
	}
	if onComp == nil || onComp.Quantity != 150 {
		t.Errorf("coins on companion qty = %v, want 150", onComp)
	}
	if onComp != nil && onComp.Notes != "100gp 50sp" {
		t.Errorf("coins on companion notes = %q, want %q", onComp.Notes, "100gp 50sp")
	}
}

func TestSplitTorches(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	form := url.Values{}
	form.Set("quantity", "3")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", torches.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	var equippedTorches, stowedTorches int
	for _, it := range items {
		if it.Name == "Torches" {
			if it.ContainerID != nil {
				stowedTorches = it.Quantity
			} else {
				equippedTorches = it.Quantity
			}
		}
	}
	if equippedTorches != 3 {
		t.Errorf("torches on character = %d, want 3", equippedTorches)
	}
	if stowedTorches != 3 {
		t.Errorf("torches in backpack = %d, want 3", stowedTorches)
	}
}

func TestSplitAllBecomesMove(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	form := url.Values{}
	form.Set("quantity", "6")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", torches.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	var torchCount int
	for _, it := range items {
		if it.Name == "Torches" {
			torchCount++
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Torches should be in backpack, ContainerID = %v", it.ContainerID)
			}
			if it.Quantity != 6 {
				t.Errorf("Torches Quantity = %d, want 6", it.Quantity)
			}
		}
	}
	if torchCount != 1 {
		t.Errorf("got %d torch items, want 1", torchCount)
	}
}

func TestSplitEmptyQuantityMovesAll(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	// Split with empty quantity — should move all
	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", torches.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	var torchCount int
	for _, it := range items {
		if it.Name == "Torches" {
			torchCount++
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Torches should be in backpack, ContainerID = %v", it.ContainerID)
			}
			if it.Quantity != 6 {
				t.Errorf("Torches Quantity = %d, want 6", it.Quantity)
			}
		}
	}
	if torchCount != 1 {
		t.Errorf("got %d torch items, want 1", torchCount)
	}
}

func TestTransferItemGive(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Alder", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	retainer := &db.Character{Name: "Bram", Class: "Hunter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}
	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Rope", Quantity: 1}
	if err := d.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("direction", "give")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(retainer.ID)
	if len(items) != 1 {
		t.Fatalf("got %d retainer items, want 1", len(items))
	}
	if items[0].CharacterID != retainer.ID {
		t.Errorf("item CharacterID = %d, want %d", items[0].CharacterID, retainer.ID)
	}
}

func TestTransferItemTake(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Alder", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	retainer := &db.Character{Name: "Bram", Class: "Hunter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}
	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: retainer.ID, Name: "Torch", Quantity: 3}
	if err := d.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("direction", "take")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(employer.ID)
	if len(items) != 1 {
		t.Fatalf("got %d employer items, want 1", len(items))
	}
	if items[0].CharacterID != employer.ID {
		t.Errorf("item CharacterID = %d, want %d", items[0].CharacterID, employer.ID)
	}
}

func TestTransferItemPartial(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Alder", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	retainer := &db.Character{Name: "Bram", Class: "Hunter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}
	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Torches", Quantity: 5}
	if err := d.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("direction", "give")
	form.Set("quantity", "2")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	employerItems, _ := d.ListItems(employer.ID)
	retainerItems, _ := d.ListItems(retainer.ID)
	if len(employerItems) != 1 {
		t.Fatalf("got %d employer items, want 1", len(employerItems))
	}
	if employerItems[0].Quantity != 3 {
		t.Errorf("employer quantity = %d, want 3", employerItems[0].Quantity)
	}
	if len(retainerItems) != 1 {
		t.Fatalf("got %d retainer items, want 1", len(retainerItems))
	}
	if retainerItems[0].Quantity != 2 {
		t.Errorf("retainer quantity = %d, want 2", retainerItems[0].Quantity)
	}
}

func TestTransferItemRejectsInvalidDirection(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Alder", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	retainer := &db.Character{Name: "Bram", Class: "Hunter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}
	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Rope", Quantity: 1}
	if err := d.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("direction", "swap")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTransferItemRejectsWrongOwner(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Alder", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	retainer := &db.Character{Name: "Bram", Class: "Hunter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}
	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: retainer.ID, Name: "Rope", Quantity: 1}
	if err := d.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("direction", "give")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDecrementBundledItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/decrement/", torches.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 3 {
		t.Errorf("Torches Quantity = %d, want 3", items[0].Quantity)
	}
}

func TestBundledItemUsesDecrementButton(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 3}
	d.CreateItem(torches)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, fmt.Sprintf("items/%d/decrement/", torches.ID)) {
		t.Fatalf("expected decrement button for bundled item")
	}
	if strings.Contains(body, fmt.Sprintf("items/%d/delete/", torches.ID)) {
		t.Fatalf("expected bundled item to avoid delete button")
	}
}

func TestUnbundledItemUsesDeleteButton(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, fmt.Sprintf("items/%d/delete/", rope.ID)) {
		t.Fatalf("expected delete button for unbundled item")
	}
	if strings.Contains(body, fmt.Sprintf("items/%d/decrement/", rope.ID)) {
		t.Fatalf("expected unbundled item to avoid decrement button")
	}
}

func TestDeleteContainerCascadeServer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(rope)

	// Delete backpack
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/delete/", backpack.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (rope remains)", len(items))
	}
	if items[0].Name != "Rope" {
		t.Errorf("remaining item = %q, want Rope", items[0].Name)
	}
	if items[0].ContainerID != nil {
		t.Errorf("Rope ContainerID = %v, want nil", items[0].ContainerID)
	}
}

func TestDeleteCompanionMovesItemsServer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	bedroll := &db.Item{CharacterID: ch.ID, Name: "Bedroll", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(bedroll)

	// Delete companion
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/companions/%d/delete/", comp.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Companion should be gone
	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 0 {
		t.Errorf("got %d companions, want 0", len(comps))
	}

	// Item should remain, moved to equipped on character
	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].CompanionID != nil {
		t.Errorf("Bedroll CompanionID = %v, want nil", items[0].CompanionID)
	}
}

func TestUpdateHP(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("hp_current", "5")
	req := httptest.NewRequest("POST", "/characters/1/hp/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify HP was updated in DB
	got, _ := d.GetCharacter(ch.ID)
	if got.HPCurrent != 5 {
		t.Errorf("HPCurrent = %d, want 5", got.HPCurrent)
	}
	if got.HPMax != 8 {
		t.Errorf("HPMax = %d, want 8", got.HPMax)
	}
}

func TestBuildMoveTargetsIncludesCompanionContainers(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Backpack equipped on character
	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	// Chest on the companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	items, _ := d.ListItems(ch.ID)
	compViews := []CompanionView{{Companion: *comp, LoadCapacity: 25}}
	targets := buildMoveTargets(items, compViews, nil)

	// Should have: Equipped, Equipped > Backpack, Bessie (Mule) > Chest (wooden, large), Bessie (Mule)
	found := map[string]bool{}
	for _, t := range targets {
		found[t.Label] = true
	}

	if !found["Equipped"] {
		t.Error("missing Equipped target")
	}
	if !found["Equipped > Backpack"] {
		t.Error("missing Equipped > Backpack target")
	}
	if !found["Bessie (Mule) > Chest (wooden, large)"] {
		t.Errorf("missing companion container target, got targets: %v", targets)
	}
	if !found["Bessie (Mule)"] {
		t.Errorf("missing companion target, got targets: %v", targets)
	}
}

func TestUpdateItemCreatesAuditLog(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	item := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, Location: "stowed"}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("quantity", "3")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", item.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "item_update" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "item_update")
	}
}

func TestDecrementItemCreatesAuditLog(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/decrement/", torches.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "item_decrement" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "item_decrement")
	}
	if !strings.Contains(auditLog[0].Detail, "Torches") {
		t.Errorf("AuditLog.Detail = %q, want it to contain 'Torches'", auditLog[0].Detail)
	}
}

func TestUpdateCompanionCreatesAuditLog(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID, Name: "Bessie", Breed: "Mule",
		HPCurrent: 9, HPMax: 9,
	}
	d.CreateCompanion(comp)

	form := url.Values{}
	form.Set("name", "Bessie")
	form.Set("hp_current", "7")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/companions/%d/update/", comp.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "companion_update" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "companion_update")
	}
	if !strings.Contains(auditLog[0].Detail, "Bessie") {
		t.Errorf("AuditLog.Detail = %q, want it to contain 'Bessie'", auditLog[0].Detail)
	}
}

func TestSplitCoinsCreatesAuditLog(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp"}
	d.CreateItem(coins)

	form := url.Values{}
	form.Set("quantity", "25gp")
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	var found bool
	for _, entry := range auditLog {
		if entry.Action == "item_split" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action 'item_split'")
	}
}

func TestAddNoteCreatesAuditLog(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("content", "Remember to buy torches")
	req := httptest.NewRequest("POST", "/characters/1/notes/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "note_add" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "note_add")
	}
	if !strings.Contains(auditLog[0].Detail, "Remember to buy torches") {
		t.Errorf("AuditLog.Detail = %q, want it to contain note content", auditLog[0].Detail)
	}
}

func TestDeleteNoteCreatesAuditLog(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	note := &db.Note{CharacterID: ch.ID, Content: "Old note"}
	d.CreateNote(note)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/notes/%d/delete/", note.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "note_delete" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "note_delete")
	}
}

func TestAuditLogViewerRenderedOnCards(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Create audit log entries for different action types
	d.AddAuditLog(ch.ID, "hp_change", "HP 8 → 5", 0)
	d.AddAuditLog(ch.ID, "item_add", "Rope", 0)
	d.AddAuditLog(ch.ID, "companion_add", "Bessie", 0)
	d.AddAuditLog(ch.ID, "note_add", "Remember torches", 0)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Each card should have a details/summary log viewer
	if !strings.Contains(body, "Activity Log") {
		t.Error("stats card should contain 'Activity Log' viewer")
	}
	if !strings.Contains(body, "Item Log") {
		t.Error("inventory card should contain 'Item Log' viewer")
	}
	if !strings.Contains(body, "Companion Log") {
		t.Error("companions card should contain 'Companion Log' viewer")
	}
	if !strings.Contains(body, "Notes Log") {
		t.Error("notes card should contain 'Notes Log' viewer")
	}
}

func TestEncumbranceCompanionsSection(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "COMPANIONS") {
		t.Error("encumbrance card should contain 'COMPANIONS' section header")
	}
}

func TestBuildMoveTargetsIncludesNestedContainers(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Backpack equipped on character
	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	// Belt pouch inside the backpack
	beltPouch := &db.Item{CharacterID: ch.ID, Name: "Belt Pouch", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(beltPouch)

	// Chest on the companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	// Sack inside the chest on the companion
	sack := &db.Item{CharacterID: ch.ID, Name: "Sack", Quantity: 1, ContainerID: &chest.ID}
	d.CreateItem(sack)

	// Scroll case inside the sack inside the chest (3 levels deep)
	scrollCase := &db.Item{CharacterID: ch.ID, Name: "Scroll Case", Quantity: 1, ContainerID: &sack.ID}
	d.CreateItem(scrollCase)

	items, _ := d.ListItems(ch.ID)
	compViews := []CompanionView{{Companion: *comp, LoadCapacity: 25}}
	targets := buildMoveTargets(items, compViews, nil)

	found := map[string]bool{}
	for _, tgt := range targets {
		found[tgt.Label] = true
	}

	// All containers at every depth should appear as move targets
	for _, want := range []string{
		"Equipped",
		"Equipped > Backpack",
		"Equipped > Backpack > Belt Pouch",
		"Bessie (Mule) > Chest (wooden, large)",
		"Bessie (Mule) > Chest (wooden, large) > Sack",
		"Bessie (Mule) > Chest (wooden, large) > Sack > Scroll Case",
		"Bessie (Mule)",
	} {
		if !found[want] {
			t.Errorf("missing %q target, got targets: %v", want, targets)
		}
	}
}

func TestEquippedSpeedChart(t *testing.T) {
	cells := EquippedSpeedChart(5)
	if len(cells) != 10 {
		t.Fatalf("got %d cells, want 10", len(cells))
	}
	// First 5 should be filled, rest empty
	for i, c := range cells {
		if i < 5 && !c.Filled {
			t.Errorf("cell %d should be filled", i)
		}
		if i >= 5 && c.Filled {
			t.Errorf("cell %d should be empty", i)
		}
	}
	// Speed brackets: 0-2=40, 3-4=30, 5-6=20, 7-9=10
	if cells[0].Speed != 40 {
		t.Errorf("cell 0 speed = %d, want 40", cells[0].Speed)
	}
	if cells[3].Speed != 30 {
		t.Errorf("cell 3 speed = %d, want 30", cells[3].Speed)
	}
	if cells[5].Speed != 20 {
		t.Errorf("cell 5 speed = %d, want 20", cells[5].Speed)
	}
	if cells[7].Speed != 10 {
		t.Errorf("cell 7 speed = %d, want 10", cells[7].Speed)
	}
}

func TestStowedSpeedChart(t *testing.T) {
	cells := StowedSpeedChart(12)
	if len(cells) != 16 {
		t.Fatalf("got %d cells, want 16", len(cells))
	}
	// First 12 filled, rest empty
	for i, c := range cells {
		if i < 12 && !c.Filled {
			t.Errorf("cell %d should be filled", i)
		}
		if i >= 12 && c.Filled {
			t.Errorf("cell %d should be empty", i)
		}
	}
	// Speed brackets: 0-9=40, 10-11=30, 12-13=20, 14-15=10
	if cells[0].Speed != 40 {
		t.Errorf("cell 0 speed = %d, want 40", cells[0].Speed)
	}
	if cells[10].Speed != 30 {
		t.Errorf("cell 10 speed = %d, want 30", cells[10].Speed)
	}
	if cells[12].Speed != 20 {
		t.Errorf("cell 12 speed = %d, want 20", cells[12].Speed)
	}
	if cells[14].Speed != 10 {
		t.Errorf("cell 14 speed = %d, want 10", cells[14].Speed)
	}
}

func TestInventorySpacing(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "card-disclosure-summary-body") {
		t.Error("expected card disclosure summary wrapper")
	}
	if !strings.Contains(body, "card-disclosure-toggle") {
		t.Error("expected card disclosure toggle bar")
	}
	if !strings.Contains(body, "card-disclosure-body") {
		t.Error("expected card disclosure body")
	}
}

func TestCoinItemAppearsInInventory(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Create consolidated coin item
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 70, Notes: "50gp 20sp"})

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	// Should show consolidated coin item
	if !strings.Contains(body, "Coins") {
		t.Error("inventory should contain Coins item")
	}
	// Should show denomination breakdown from notes
	if !strings.Contains(body, "50gp") {
		t.Error("Coins item should show denomination breakdown")
	}
	// Should not render editable notes input for coins
	if strings.Contains(body, "name=\"notes\"") {
		t.Error("Coins item should not include notes input")
	}
	// Should have split form
	if !strings.Contains(body, "/split/") {
		t.Error("inventory should contain split form for coin items")
	}
}

func TestStoreCardCollapsedAndOrderedBelowInventory(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	inventoryIdx := strings.Index(body, "id=\"inventory\"")
	if inventoryIdx == -1 {
		t.Fatal("expected inventory card in response")
	}
	storeIdx := strings.Index(body, "id=\"store\"")
	if storeIdx == -1 {
		t.Fatal("expected store card in response")
	}
	encumbranceIdx := strings.Index(body, "id=\"encumbrance\"")
	if encumbranceIdx == -1 {
		t.Fatal("expected encumbrance card in response")
	}
	if !(inventoryIdx < storeIdx && storeIdx < encumbranceIdx) {
		t.Errorf("expected store card after inventory and before encumbrance")
	}
	end := len(body)
	if encumbranceIdx > storeIdx {
		end = encumbranceIdx
	}
	storeSegment := body[storeIdx:end]
	if !strings.Contains(storeSegment, "<details") {
		t.Error("store card should use a details element for collapse")
	}
	if strings.Contains(storeSegment, "<details open") {
		t.Error("store card should be collapsed by default")
	}
	if !strings.Contains(storeSegment, "Rope") {
		t.Error("store card should list rope")
	}
	if !strings.Contains(storeSegment, "Buy </button>") {
		t.Error("store card should include buy actions")
	}
}

func TestStoreCardSectionsAreCollapsible(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	storeIdx := strings.Index(body, "id=\"store\"")
	if storeIdx == -1 {
		t.Fatal("expected store card in response")
	}
	end := len(body)
	encumbranceIdx := strings.Index(body, "id=\"encumbrance\"")
	if encumbranceIdx > storeIdx {
		end = encumbranceIdx
	}
	storeSegment := body[storeIdx:end]
	for _, title := range []string{"Adventuring Gear", "Weapons", "Ammunition", "Armour", "Horses and Vehicles"} {
		if !strings.Contains(storeSegment, title) {
			t.Errorf("store card should contain %q section", title)
		}
	}
	summaryCount := strings.Count(storeSegment, "text-sm font-bold text-stone-700 cursor-pointer")
	const want = 5
	if summaryCount != want {
		t.Errorf("expected %d store group summaries, got %d", want, summaryCount)
	}
}

func TestStoreCardListsAdventuringGear(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Lantern (hooded)") {
		t.Error("store should list lantern (hooded)")
	}
	if !strings.Contains(body, "Caltrops") {
		t.Error("store should list caltrops")
	}
	if !strings.Contains(body, "Holy symbol (silver)") {
		t.Error("store should list holy symbol (silver)")
	}
	if !strings.Contains(body, "Winter cloak") {
		t.Error("store should list winter cloak")
	}
}

func TestStoreCardListsHorseSupplies(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Charger") {
		t.Error("store should list charger horse")
	}
	if !strings.Contains(body, "Horse barding") {
		t.Error("store should list horse barding")
	}
	if !strings.Contains(body, "Feed") {
		t.Error("store should list feed")
	}
	if !strings.Contains(body, "5cp") {
		t.Error("store should show feed cost in copper")
	}
}

func TestStoreCardShowsHorseBardingAC(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	start := strings.Index(body, "Horse barding")
	if start == -1 {
		t.Fatal("store should list horse barding")
	}
	segment := body[start:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected buy button after horse barding")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "AC 2") {
		t.Error("horse barding should show AC 2")
	}
}

func TestStoreCardShowsHorseBardingWeight(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	start := strings.Index(body, "Horse barding")
	if start == -1 {
		t.Fatal("store should list horse barding")
	}
	segment := body[start:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected buy button after horse barding")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "600 cn") {
		t.Error("horse barding should show weight")
	}
}

func TestStoreCardShowsCanoeWeight(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	start := strings.Index(body, "Canoe")
	if start == -1 {
		t.Fatal("store should list canoe")
	}
	segment := body[start:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected buy button after canoe")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "500 cn") {
		t.Error("canoe should show weight")
	}
}

func TestStoreCardShowsHorseAndVehicleStats(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Load 4000 cn") {
		t.Error("store should show charger load capacity")
	}
	cartStart := strings.Index(body, "Cart")
	if cartStart == -1 {
		t.Fatal("store should list cart")
	}
	segment := body[cartStart:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected buy button after cart")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "Cargo 10000 cn") {
		t.Error("store should show cart cargo capacity")
	}
	segmentWithoutCargo := strings.ReplaceAll(segment, "Cargo 10000 cn", "")
	if strings.Contains(segmentWithoutCargo, " cn") {
		t.Error("cart should not show weight when only cargo is defined")
	}
}

func TestStoreCardShowsContainerCapacity(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	start := strings.Index(body, "Backpack")
	if start == -1 {
		t.Fatal("store should list backpack")
	}
	segment := body[start:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected buy button after backpack")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "Capacity 10 slots") {
		t.Error("backpack should show capacity")
	}
}

func TestStoreCardShowsHolyWaterCombatStats(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	adventuringStart := strings.Index(body, "Adventuring Gear")
	if adventuringStart == -1 {
		t.Fatal("store should list adventuring gear")
	}
	weaponsStart := strings.Index(body, "Weapons")
	if weaponsStart == -1 {
		t.Fatal("store should list weapons")
	}
	adventuringSegment := body[adventuringStart:weaponsStart]
	waterStart := strings.Index(adventuringSegment, "Holy water")
	if waterStart == -1 {
		t.Fatal("store should list holy water under adventuring gear")
	}
	segment := adventuringSegment[waterStart:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected to find buy button after holy water")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "1d8") {
		t.Error("holy water should show damage")
	}
	if !strings.Contains(segment, "Missile") {
		t.Error("holy water should show missile quality")
	}
	if !strings.Contains(segment, "Splash") {
		t.Error("holy water should show splash quality")
	}
}

func TestStoreCardShowsTorchCombatStats(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	weaponsStart := strings.Index(body, "Weapons")
	if weaponsStart == -1 {
		t.Fatal("store should list weapons")
	}
	ammoStart := strings.Index(body, "Ammunition")
	if ammoStart == -1 {
		t.Fatal("store should list ammunition")
	}
	weaponSegment := body[weaponsStart:ammoStart]
	torchStart := strings.Index(weaponSegment, "Torches")
	if torchStart == -1 {
		t.Fatal("store should list torches")
	}
	segment := weaponSegment[torchStart:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected to find buy button after torches")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "1d4") {
		t.Error("torches should show damage")
	}
	if !strings.Contains(segment, "Melee") {
		t.Error("torches should show melee quality")
	}
}

func TestStoreCardShowsCrossbowRanges(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	weaponsStart := strings.Index(body, "Weapons")
	if weaponsStart == -1 {
		t.Fatal("store should list weapons")
	}
	ammoStart := strings.Index(body, "Ammunition")
	if ammoStart == -1 {
		t.Fatal("store should list ammunition")
	}
	weaponSegment := body[weaponsStart:ammoStart]
	crossbowStart := strings.Index(weaponSegment, "Crossbow")
	if crossbowStart == -1 {
		t.Fatal("store should list crossbow")
	}
	segment := weaponSegment[crossbowStart:]
	end := strings.Index(segment, "Buy </button>")
	if end == -1 {
		t.Fatal("expected to find buy button after crossbow")
	}
	segment = segment[:end]
	if !strings.Contains(segment, "Armour piercing") {
		t.Error("crossbow should show armour piercing quality")
	}
	if !strings.Contains(segment, "Missile (80′ / 160′ / 240′)") {
		t.Error("crossbow should show missile ranges")
	}
}

func TestStoreBuyDeductsCoinsAndAddsItem(t *testing.T) {
	if engine.ItemBundleSize("Rope") != 0 {
		t.Fatal("expected rope to have no bundle size")
	}

	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// 1pp 2gp = 700cp total
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 3, Notes: "1pp 2gp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("rope", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var coinNotes string
	var coinQty int
	var ropeQty int
	for _, item := range items {
		switch item.Name {
		case "Coins":
			coinNotes = item.Notes
			coinQty = item.Quantity
		case "Rope":
			ropeQty = item.Quantity
		}
	}
	if ropeQty != 1 {
		t.Fatalf("rope quantity = %d, want 1", ropeQty)
	}
	if coinNotes != "1pp 1gp" {
		t.Fatalf("coin notes = %q, want %q", coinNotes, "1pp 1gp")
	}
	if coinQty != 2 {
		t.Fatalf("coin quantity = %d, want 2", coinQty)
	}
}

func TestStoreBuyBundledItemUsesBundleSize(t *testing.T) {
	bundleSize := engine.ItemBundleSize("Torches")
	if bundleSize <= 0 {
		t.Fatalf("expected torches bundle size > 0, got %d", bundleSize)
	}

	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 1, Notes: "1gp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("torches", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var torchesQty int
	for _, item := range items {
		if item.Name == "Torches" {
			torchesQty = item.Quantity
		}
	}
	if torchesQty != bundleSize {
		t.Fatalf("torches quantity = %d, want %d", torchesQty, bundleSize)
	}
}

func TestStoreBuyDefaultsToStowedLocation(t *testing.T) {
	bundleSize := engine.ItemBundleSize("Torches")
	if bundleSize <= 0 {
		t.Fatalf("expected torches bundle size > 0, got %d", bundleSize)
	}

	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 1, Notes: "1gp"})
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 2, Location: "stowed"})

	form := url.Values{}
	form.Set("item_id", storeItemID("torches", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var torchesQty int
	var torchesLocation string
	var torchesCount int
	for _, item := range items {
		if item.Name == "Torches" {
			torchesQty = item.Quantity
			torchesLocation = item.Location
			torchesCount++
		}
	}
	if torchesCount != 1 {
		t.Fatalf("torches entries = %d, want 1", torchesCount)
	}
	if torchesQty != bundleSize+2 {
		t.Fatalf("torches quantity = %d, want %d", torchesQty, bundleSize+2)
	}
	if torchesLocation != "stowed" {
		t.Fatalf("torches location = %q, want %q", torchesLocation, "stowed")
	}
}

func TestStoreBuyBundleDisplayShowsTotals(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Torches") {
		t.Fatal("store should list torches")
	}
	if !strings.Contains(body, "30 cn") {
		t.Fatal("store should show bundle weight for torches")
	}
	if !strings.Contains(body, "Arrows") {
		t.Fatal("store should list arrows")
	}
	if !strings.Contains(body, "400 cn") {
		t.Fatal("store should show bundle weight for arrows")
	}
	if !strings.Contains(body, "Sling stones") {
		t.Fatal("store should list sling stones")
	}
	if !strings.Contains(body, "Free") {
		t.Fatal("store should label sling stones as free")
	}
}

func TestStoreBuyFreeItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("item_id", storeItemID("Sling stones", 0))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Name != "Sling stones" {
		t.Fatalf("item name = %q, want %q", items[0].Name, "Sling stones")
	}
	if items[0].Quantity != 20 {
		t.Fatalf("item quantity = %d, want %d", items[0].Quantity, 20)
	}
	if items[0].Location != "stowed" {
		t.Fatalf("item location = %q, want %q", items[0].Location, "stowed")
	}
}

func TestStoreBuyRejectsTamperedItemID(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 1, Notes: "1gp"})

	form := url.Values{}
	form.Set("item_id", "Plate mail|1")
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Notes != "1gp" {
		t.Fatalf("coin notes = %q, want %q", items[0].Notes, "1gp")
	}
}

func TestStoreBuyUsesPurseOnly(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		FoundGP: 1,
	}
	d.CreateCharacter(ch)

	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 1, Notes: "1gp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("rope", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Name != "Coins" {
		t.Fatalf("item name = %q, want %q", items[0].Name, "Coins")
	}
	if items[0].Notes != "1gp" {
		t.Fatalf("coin notes = %q, want %q", items[0].Notes, "1gp")
	}
}

func TestStoreBuyUsesChangeMaking(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// 1pp 1gp 6cp = 606cp total
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 8, Notes: "1pp 1gp 6cp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("rope", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var coinNotes string
	var coinQty int
	var ropeQty int
	for _, item := range items {
		switch item.Name {
		case "Coins":
			coinNotes = item.Notes
			coinQty = item.Quantity
		case "Rope":
			ropeQty = item.Quantity
		}
	}
	if ropeQty != 1 {
		t.Fatalf("rope quantity = %d, want 1", ropeQty)
	}
	if coinNotes != "1pp 6cp" {
		t.Fatalf("coin notes = %q, want %q", coinNotes, "1pp 6cp")
	}
	if coinQty != 7 {
		t.Fatalf("coin quantity = %d, want 7", coinQty)
	}
}

func TestStoreBuyUsesElectrumChange(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// 1gp 5sp = 150cp total
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 6, Notes: "1gp 5sp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("rope", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var coinNotes string
	var coinQty int
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
			coinQty = item.Quantity
		}
	}
	if coinNotes != "5sp" {
		t.Fatalf("coin notes = %q, want %q", coinNotes, "5sp")
	}
	if coinQty != 5 {
		t.Fatalf("coin quantity = %d, want 5", coinQty)
	}
}

func TestStoreBuyAcceptsElectrumInPurse(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// 1gp 1ep = 150cp total (EP converted to CP value during changemaking)
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 2, Notes: "1gp 1ep"})

	form := url.Values{}
	form.Set("item_id", storeItemID("rope", 100))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	var coinNotes string
	var coinQty int
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
			coinQty = item.Quantity
		}
	}
	// 150cp - 100cp = 50cp = 5sp (EP is converted away)
	if coinNotes != "5sp" {
		t.Fatalf("coin notes = %q, want %q", coinNotes, "5sp")
	}
	if coinQty != 5 {
		t.Fatalf("coin quantity = %d, want 5", coinQty)
	}
}

func TestCoinSlotsOnCharacterStowed(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Consolidated coin item on character
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 150, Notes: "100gp 50sp"})

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	// 150 coins = 2 slots
	if view.EquippedSlots != 2 {
		t.Errorf("EquippedSlots = %d, want 2 (150 coins)", view.EquippedSlots)
	}
}

func TestMoveCoinToCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 150, Notes: "150gp"}
	d.CreateItem(coins)

	// Move all coins to companion via split
	form := url.Values{}
	form.Set("quantity", "150gp")
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify encumbrance: coins on companion, not on character
	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}
	if view.EquippedSlots != 0 {
		t.Errorf("EquippedSlots = %d, want 0 (coins moved to companion)", view.EquippedSlots)
	}

	// Companion should show the coin slots
	if len(view.CompanionGroups) != 1 {
		t.Fatalf("got %d companion groups, want 1", len(view.CompanionGroups))
	}
	if view.CompanionGroups[0].UsedSlots != 2 {
		t.Errorf("companion UsedSlots = %d, want 2 (150 coins)", view.CompanionGroups[0].UsedSlots)
	}
}

func TestMoveCoinBackToCharacter(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Coins on companion
	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 150, Notes: "150gp", CompanionID: &comp.ID}
	d.CreateItem(coins)

	// Move coins back to character via split
	form := url.Values{}
	form.Set("quantity", "150gp")
	form.Set("move_to", "equipped")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}
	// 150 coins on character = 2 equipped slots
	if view.EquippedSlots != 2 {
		t.Errorf("EquippedSlots = %d, want 2 (coins back on character)", view.EquippedSlots)
	}
}

func TestDeleteCompanionWithCoinsResetsLocation(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Coins on companion
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 150, Notes: "150gp", CompanionID: &comp.ID})

	// Delete the companion
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/companions/%d/delete/", comp.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Coins should be back on character (companion deletion moves items to character)
	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}
	if view.EquippedSlots != 2 {
		t.Errorf("EquippedSlots = %d, want 2 (coins back on character)", view.EquippedSlots)
	}
}

func TestMoveCoinToContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Chest on the companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 150, Notes: "150gp"}
	d.CreateItem(coins)

	// Move coins into the chest via split
	form := url.Values{}
	form.Set("quantity", "150gp")
	form.Set("move_to", fmt.Sprintf("container:%d", chest.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Coins should not count toward character slots (they're in a chest on a companion)
	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}
	if view.EquippedSlots != 0 {
		t.Errorf("EquippedSlots = %d, want 0 (coins in chest on companion)", view.EquippedSlots)
	}
}

func TestSetItemNotes(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1}
	d.CreateItem(chest)

	// Set notes
	form := url.Values{}
	form.Set("notes", "locked")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", chest.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	item, _ := d.GetItem(chest.ID)
	if item.Notes != "locked" {
		t.Errorf("Notes = %q, want %q", item.Notes, "locked")
	}

	// Verify it renders
	body := w.Body.String()
	if !strings.Contains(body, "locked") {
		t.Error("response should contain notes text")
	}
}

func TestClearItemNotes(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, Notes: "locked"}
	d.CreateItem(chest)

	// Clear notes by sending empty string with has_notes marker
	form := url.Values{}
	form.Set("notes", "")
	form.Set("has_notes", "1")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", chest.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	item, _ := d.GetItem(chest.ID)
	if item.Notes != "" {
		t.Errorf("Notes = %q, want empty", item.Notes)
	}
}

func TestNestedContainerContentsBubbleUp(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Backpack equipped on character (capacity 10)
	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	// Sack inside the backpack
	sack := &db.Item{CharacterID: ch.ID, Name: "Sack", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(sack)

	// Rope directly in backpack (1 slot)
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID})

	// 6 torches in the sack (2 slots: 6/3 = 2 bundles)
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6, ContainerID: &sack.ID})

	// 5 preserved rations in the sack (1 slot: 5*20cn=100cn)
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Preserved Rations", Quantity: 5, ContainerID: &sack.ID})

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	// Find the backpack in the equipped tree
	var bp *InventoryItem
	for i := range view.EquippedItems {
		if view.EquippedItems[i].Name == "Backpack" {
			bp = &view.EquippedItems[i]
			break
		}
	}
	if bp == nil {
		t.Fatal("Backpack not found in equipped items")
	}

	// Find the sack inside the backpack
	var sk *InventoryItem
	for i := range bp.Children {
		if bp.Children[i].Name == "Sack" {
			sk = &bp.Children[i]
			break
		}
	}
	if sk == nil {
		t.Fatal("Sack not found in backpack children")
	}

	// Sack's own UsedSlots: 2 (torches) + 1 (rations) = 3
	if sk.UsedSlots != 3 {
		t.Errorf("Sack UsedSlots = %d, want 3", sk.UsedSlots)
	}

	// Backpack should include: sack slots + rope slots + sack's contents
	// Sack (stowed in backpack, not equipped) has a slot cost.
	// Rope = 1 slot. Sack contents = 3 slots.
	// So backpack UsedSlots = sack.Slots + 1 + 3
	sackSlots := sk.Slots
	wantBPUsed := sackSlots + 1 + 3 // sack + rope + sack contents
	if bp.UsedSlots != wantBPUsed {
		t.Errorf("Backpack UsedSlots = %d, want %d (sack %d slots + rope 1 + sack contents 3)", bp.UsedSlots, wantBPUsed, sackSlots)
	}
}

func TestCoinsBubbleUpThroughContainers(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Chest on companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	// Sack inside chest
	sack := &db.Item{CharacterID: ch.ID, Name: "Sack", Quantity: 1, ContainerID: &chest.ID}
	d.CreateItem(sack)

	// Consolidated coin item in the sack
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 200, Notes: "200gp", ContainerID: &sack.ID})

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	// Find the chest in the companion group
	if len(view.CompanionGroups) != 1 {
		t.Fatalf("got %d companion groups, want 1", len(view.CompanionGroups))
	}
	cg := view.CompanionGroups[0]

	var chestItem *InventoryItem
	for i := range cg.Items {
		if cg.Items[i].Name == "Chest (wooden, large)" {
			chestItem = &cg.Items[i]
			break
		}
	}
	if chestItem == nil {
		t.Fatal("Chest not found in companion items")
	}

	// Find the sack inside the chest
	var sackItem *InventoryItem
	for i := range chestItem.Children {
		if chestItem.Children[i].Name == "Sack" {
			sackItem = &chestItem.Children[i]
			break
		}
	}
	if sackItem == nil {
		t.Fatal("Sack not found in chest children")
	}

	// Sack should show coins' 2 slots (200 coins = 2 slots via weight-based)
	if sackItem.UsedSlots != 2 {
		t.Errorf("Sack UsedSlots = %d, want 2 (coins)", sackItem.UsedSlots)
	}

	// Chest should include: sack slots + sack contents (coins 2 slots)
	wantChestUsed := sackItem.Slots + 2
	if chestItem.UsedSlots != wantChestUsed {
		t.Errorf("Chest UsedSlots = %d, want %d (sack %d + coins 2)", chestItem.UsedSlots, wantChestUsed, sackItem.Slots)
	}
}

func TestAdvanceDay(t *testing.T) {
	t.Run("advances one day", func(t *testing.T) {
		srv, d := setupTest(t)
		mux := srv.Mux()

		ch := &db.Character{
			Name: "Test", Class: "Knight", Kindred: "Human",
			Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 1,
		}
		d.CreateCharacter(ch)

		form := url.Values{}
		form.Set("day_delta", "1")
		req := httptest.NewRequest("POST", "/characters/1/advance-day/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		got, _ := d.GetCharacter(ch.ID)
		if got.CurrentDay != 2 {
			t.Errorf("CurrentDay = %d, want 2", got.CurrentDay)
		}

		// Audit log should record the day advance
		logs, _ := d.ListAuditLog(ch.ID)
		if len(logs) == 0 {
			t.Fatal("expected audit log entry for day advance")
		}
		if logs[0].Action != "day_advance" {
			t.Errorf("audit action = %q, want %q", logs[0].Action, "day_advance")
		}
	})

	t.Run("advances by delta", func(t *testing.T) {
		srv, d := setupTest(t)
		mux := srv.Mux()

		ch := &db.Character{
			Name: "Test", Class: "Knight", Kindred: "Human",
			Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 3,
		}
		d.CreateCharacter(ch)

		form := url.Values{}
		form.Set("day_delta", "7")
		req := httptest.NewRequest("POST", "/characters/1/advance-day/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		got, _ := d.GetCharacter(ch.ID)
		if got.CurrentDay != 10 {
			t.Errorf("CurrentDay = %d, want 10", got.CurrentDay)
		}
	})

	t.Run("clamps to day one", func(t *testing.T) {
		srv, d := setupTest(t)
		mux := srv.Mux()

		ch := &db.Character{
			Name: "Test", Class: "Knight", Kindred: "Human",
			Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 2,
		}
		d.CreateCharacter(ch)

		form := url.Values{}
		form.Set("day_delta", "-5")
		req := httptest.NewRequest("POST", "/characters/1/advance-day/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		got, _ := d.GetCharacter(ch.ID)
		if got.CurrentDay != 1 {
			t.Errorf("CurrentDay = %d, want 1", got.CurrentDay)
		}
	})
}

func TestCalendarUpdate(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 10, CalendarStartDay: 1,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("calendar_day", "5")
	form.Set("calendar_month", "2")
	req := httptest.NewRequest("POST", "/characters/1/calendar/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	got, _ := d.GetCharacter(ch.ID)
	if got.CalendarStartDay != 26 {
		t.Errorf("CalendarStartDay = %d, want 26", got.CalendarStartDay)
	}

	logs, _ := d.ListAuditLog(ch.ID)
	if len(logs) == 0 {
		t.Fatal("expected audit log entry for calendar update")
	}
	if logs[0].Action != "calendar_update" {
		t.Errorf("audit action = %q, want %q", logs[0].Action, "calendar_update")
	}
	if logs[0].Detail != "Calendar set to Lymewald 5" {
		t.Errorf("audit detail = %q, want %q", logs[0].Detail, "Calendar set to Lymewald 5")
	}
}

func TestBankDeposit(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 5,
	}
	d.CreateCharacter(ch)

	// Create coin item to deposit from
	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 15, Notes: "10gp 5sp"}
	d.CreateItem(coins)

	// Split coins to bank
	form := url.Values{}
	form.Set("quantity", "10gp 5sp")
	form.Set("move_to", "bank")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Coin item should be deleted (all coins deposited)
	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (coin item should be removed)", len(items))
	}

	// Check bank deposit created
	deps, _ := d.ListBankDeposits(ch.ID)
	if len(deps) != 1 {
		t.Fatalf("bank deposits = %d, want 1", len(deps))
	}
	// 10gp=1000cp, 5sp=50cp, total=1050cp
	if deps[0].CPValue != 1050 {
		t.Errorf("CPValue = %d, want 1050", deps[0].CPValue)
	}
	if deps[0].DepositDay != 5 {
		t.Errorf("DepositDay = %d, want 5", deps[0].DepositDay)
	}
}

func TestBankDepositRejectsPP(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 1,
	}
	d.CreateCharacter(ch)

	// Create coin item with PP
	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 5, Notes: "5pp"}
	d.CreateItem(coins)

	form := url.Values{}
	form.Set("quantity", "5pp")
	form.Set("move_to", "bank")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (PP not allowed)", w.Code, http.StatusBadRequest)
	}
}

func TestMoveCoinItemToBank(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 10,
	}
	d.CreateCharacter(ch)

	// Create coin item
	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp"}
	d.CreateItem(coins)

	// Move entire coin item to bank via update endpoint
	form := url.Values{}
	form.Set("move_to", "bank")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", coins.ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Coin item should be deleted
	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (coin item deposited to bank)", len(items))
	}

	// Bank deposit should be created
	deps, _ := d.ListBankDeposits(ch.ID)
	if len(deps) != 1 {
		t.Fatalf("bank deposits = %d, want 1", len(deps))
	}
	if deps[0].CPValue != 5000 {
		t.Errorf("CPValue = %d, want 5000", deps[0].CPValue)
	}
	if deps[0].DepositDay != 10 {
		t.Errorf("DepositDay = %d, want 10", deps[0].DepositDay)
	}
}

func TestBankDepositEmptyQuantityMovesAll(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 7,
	}
	d.CreateCharacter(ch)

	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 30, Notes: "20gp 10sp"}
	d.CreateItem(coins)

	// Split to bank with empty quantity — should deposit all coins
	form := url.Values{}
	form.Set("move_to", "bank")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Coin item should be deleted
	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}

	// Bank deposit should be created with full value
	deps, _ := d.ListBankDeposits(ch.ID)
	if len(deps) != 1 {
		t.Fatalf("bank deposits = %d, want 1", len(deps))
	}
	// 20gp=2000cp, 10sp=100cp = 2100cp
	if deps[0].CPValue != 2100 {
		t.Errorf("CPValue = %d, want 2100", deps[0].CPValue)
	}
	if deps[0].DepositDay != 7 {
		t.Errorf("DepositDay = %d, want 7", deps[0].DepositDay)
	}
}

func TestBankWithdrawMature(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 35,
	}
	d.CreateCharacter(ch)

	// Create a mature deposit (deposited on day 1, current day 35 = 34 days)
	dep := &db.BankDeposit{
		CharacterID: ch.ID,
		CoinNotes:   "10gp",
		CPValue:     1000,
		DepositDay:  1,
	}
	d.CreateBankDeposit(dep)

	form := url.Values{}
	form.Set("coins", "5gp")
	req := httptest.NewRequest("POST", "/characters/1/bank/withdraw/",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Check inventory gained 5gp coin item
	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Notes != "5gp" {
		t.Errorf("item Notes = %q, want %q", items[0].Notes, "5gp")
	}

	// Deposit should be reduced: 1000 - 500 = 500
	deps, _ := d.ListBankDeposits(ch.ID)
	if len(deps) != 1 {
		t.Fatalf("deposits = %d, want 1", len(deps))
	}
	if deps[0].CPValue != 500 {
		t.Errorf("deposit CPValue = %d, want 500", deps[0].CPValue)
	}
}

func TestBankWithdrawImmature(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8, CurrentDay: 2,
	}
	d.CreateCharacter(ch)

	// Immature deposit: 1000cp on day 1, current day 2
	dep := &db.BankDeposit{
		CharacterID: ch.ID,
		CoinNotes:   "10gp",
		CPValue:     1000,
		DepositDay:  1,
	}
	d.CreateBankDeposit(dep)

	// Withdraw 9gp (900cp). With fee: gross = 900 + 900/9 = 1000. Fee = 100.
	form := url.Values{}
	form.Set("coins", "9gp")
	req := httptest.NewRequest("POST", "/characters/1/bank/withdraw/",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Check inventory gained 9gp coin item
	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Notes != "9gp" {
		t.Errorf("item Notes = %q, want %q", items[0].Notes, "9gp")
	}

	// Deposit fully consumed (gross = 1000 = deposit value)
	deps, _ := d.ListBankDeposits(ch.ID)
	if len(deps) != 0 {
		t.Errorf("deposits = %d, want 0 (fully consumed)", len(deps))
	}
}

func TestDeleteItemAuditLogUsesName(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	item := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(item)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/delete/", item.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "item_delete" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "item_delete")
	}
	if !strings.Contains(auditLog[0].Detail, "Rope") {
		t.Errorf("AuditLog.Detail = %q, want it to contain item name 'Rope'", auditLog[0].Detail)
	}
	want := "delete Rope, qty 1 from inventory"
	if auditLog[0].Detail != want {
		t.Errorf("AuditLog.Detail = %q, want %q", auditLog[0].Detail, want)
	}
}

func TestDeleteCompanionAuditLogUsesName(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID,
		Name:        "Bessie",
		Breed:       "Mule",
		HPCurrent:   9,
		HPMax:       9,
	}
	d.CreateCompanion(comp)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/companions/%d/delete/", comp.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "companion_delete" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "companion_delete")
	}
	if !strings.Contains(auditLog[0].Detail, "Bessie") {
		t.Errorf("AuditLog.Detail = %q, want it to contain companion name 'Bessie'", auditLog[0].Detail)
	}
}

func TestDeleteNoteAuditLogUsesContent(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	note := &db.Note{CharacterID: ch.ID, Content: "Remember to buy torches"}
	d.CreateNote(note)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/notes/%d/delete/", note.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "note_delete" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "note_delete")
	}
	if !strings.Contains(auditLog[0].Detail, "Remember to buy torches") {
		t.Errorf("AuditLog.Detail = %q, want it to contain note content", auditLog[0].Detail)
	}
}

func TestSplitItemAuditLogUsesHumanReadableDestination(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	form := url.Values{}
	form.Set("quantity", "3")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", torches.ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	var splitEntry *db.AuditLogEntry
	for _, entry := range auditLog {
		if entry.Action == "item_split" {
			splitEntry = &entry
			break
		}
	}
	if splitEntry == nil {
		t.Fatal("expected audit log entry with action 'item_split'")
	}
	if !strings.Contains(splitEntry.Detail, "Backpack") {
		t.Errorf("AuditLog.Detail = %q, want it to contain destination name 'Backpack'", splitEntry.Detail)
	}
	if strings.Contains(splitEntry.Detail, fmt.Sprintf("container:%d", backpack.ID)) {
		t.Errorf("AuditLog.Detail = %q, should not contain raw 'container:ID' format", splitEntry.Detail)
	}
}

func TestSplitCoinsAuditLogUsesHumanReadableDestination(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	coins := &db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 50, Notes: "50gp"}
	d.CreateItem(coins)

	form := url.Values{}
	form.Set("quantity", "25gp")
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	var splitEntry *db.AuditLogEntry
	for _, entry := range auditLog {
		if entry.Action == "item_split" {
			splitEntry = &entry
			break
		}
	}
	if splitEntry == nil {
		t.Fatal("expected audit log entry with action 'item_split'")
	}
	if !strings.Contains(splitEntry.Detail, "Bessie") {
		t.Errorf("AuditLog.Detail = %q, want it to contain destination name 'Bessie'", splitEntry.Detail)
	}
	if strings.Contains(splitEntry.Detail, fmt.Sprintf("companion:%d", comp.ID)) {
		t.Errorf("AuditLog.Detail = %q, should not contain raw 'companion:ID' format", splitEntry.Detail)
	}
}

func TestBuyCompanionFromStore(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Give enough coins to buy a Mule (30gp = 3000cp). 10pp = 5000cp.
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 10, Notes: "10pp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("Mule", 3000))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Should NOT create an inventory item named "Mule"
	items, _ := d.ListItems(ch.ID)
	for _, item := range items {
		if item.Name == "Mule" {
			t.Fatalf("buying a breed should NOT create an inventory item, got item %q", item.Name)
		}
	}

	// Should create a companion
	companions, _ := d.ListCompanions(ch.ID)
	if len(companions) != 1 {
		t.Fatalf("expected 1 companion, got %d", len(companions))
	}
	comp := companions[0]
	if comp.Breed != "Mule" {
		t.Errorf("companion breed = %q, want %q", comp.Breed, "Mule")
	}
	if comp.Name != "Mule" {
		t.Errorf("companion name = %q, want %q (default name = breed)", comp.Name, "Mule")
	}
	stats, _ := engine.BreedStats("Mule")
	if comp.HPMax != stats.HPMax {
		t.Errorf("companion HPMax = %d, want %d", comp.HPMax, stats.HPMax)
	}
	if comp.HPCurrent != stats.HPMax {
		t.Errorf("companion HPCurrent = %d, want %d", comp.HPCurrent, stats.HPMax)
	}
}

func TestBuyCompanionFromStoreDeductsCoins(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Give enough coins to buy a Mule (30gp = 3000cp). 10pp = 5000cp.
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 10, Notes: "10pp"})

	form := url.Values{}
	form.Set("item_id", storeItemID("Mule", 3000))
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Should create a companion
	companions, _ := d.ListCompanions(ch.ID)
	if len(companions) != 1 {
		t.Fatalf("expected 1 companion, got %d", len(companions))
	}

	// Coins should be deducted: 5000cp - 3000cp = 2000cp = 4pp
	items, _ := d.ListItems(ch.ID)
	for _, item := range items {
		if item.Name == "Coins" {
			if item.Notes != "4pp" {
				t.Errorf("coin notes = %q, want %q", item.Notes, "4pp")
			}
			if item.Quantity != 4 {
				t.Errorf("coin quantity = %d, want 4", item.Quantity)
			}
		}
	}

	// Should NOT create an inventory item named "Mule"
	for _, item := range items {
		if item.Name == "Mule" {
			t.Fatalf("buying a breed should NOT create an inventory item")
		}
	}

	// Audit log should have both store_buy and companion_add
	auditLog, _ := d.ListAuditLog(ch.ID)
	var hasStoreBuy, hasCompanionAdd bool
	for _, entry := range auditLog {
		if entry.Action == "store_buy" {
			hasStoreBuy = true
		}
		if entry.Action == "companion_add" {
			hasCompanionAdd = true
		}
	}
	if !hasStoreBuy {
		t.Error("expected store_buy audit log entry")
	}
	if !hasCompanionAdd {
		t.Error("expected companion_add audit log entry")
	}
}

func TestSellItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add a Longsword (store price 10gp = 1000cp, sell price = 500cp = 1pp)
	sword := &db.Item{CharacterID: ch.ID, Name: "Longsword", Quantity: 1, Location: "stowed"}
	d.CreateItem(sword)

	form := url.Values{}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/sell/", sword.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	// Sword should be gone
	for _, item := range items {
		if item.Name == "Longsword" {
			t.Fatal("longsword should have been deleted after selling")
		}
	}
	// Should have coins worth 500cp = 5gp (PP is not used in changemaking)
	var coinNotes string
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	if coinNotes != "5gp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "5gp")
	}

	// Audit log should have store_sell with wealth info
	auditLog, _ := d.ListAuditLog(ch.ID)
	var sellDetail string
	for _, entry := range auditLog {
		if entry.Action == "store_sell" {
			sellDetail = entry.Detail
		}
	}
	if sellDetail == "" {
		t.Fatal("expected store_sell audit log entry")
	}
	if !strings.Contains(sellDetail, "wealth") {
		t.Errorf("sell audit log should contain wealth info, got %q", sellDetail)
	}
}

func TestSellBundledItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Arrows: store price 5gp = 500cp for bundle of 20. Sell = 250cp = 2gp 5sp
	arrows := &db.Item{CharacterID: ch.ID, Name: "Arrows", Quantity: 20, Location: "stowed"}
	d.CreateItem(arrows)

	form := url.Values{}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/sell/", arrows.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	// Arrows should be gone
	for _, item := range items {
		if item.Name == "Arrows" {
			t.Fatal("arrows should have been deleted after selling")
		}
	}
	// Should have coins: 250cp = 2gp 5sp
	var coinNotes string
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	if coinNotes != "2gp 5sp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "2gp 5sp")
	}
}

func TestSellOilFlask(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Oil Flask: store price 1gp = 100cp, sell price = 50cp = 5sp
	oil := &db.Item{CharacterID: ch.ID, Name: "Oil Flask", Quantity: 1, Location: "stowed"}
	d.CreateItem(oil)

	form := url.Values{}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/sell/", oil.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	for _, item := range items {
		if item.Name == "Oil Flask" {
			t.Fatal("oil flask should have been deleted after selling")
		}
	}
	var coinNotes string
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	if coinNotes != "5sp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "5sp")
	}
}

func TestSellPartialQuantity(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add 3 longswords (store price 10gp = 1000cp each, sell = 500cp each)
	swords := &db.Item{CharacterID: ch.ID, Name: "Longsword", Quantity: 3, Location: "stowed"}
	d.CreateItem(swords)

	// Sell 2 via split handler with move_to=sell
	form := url.Values{"quantity": {"2"}, "move_to": {"sell"}}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", swords.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	var swordQty int
	var coinNotes string
	for _, item := range items {
		if item.Name == "Longsword" {
			swordQty = item.Quantity
		}
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	if swordQty != 1 {
		t.Errorf("remaining swords = %d, want 1", swordQty)
	}
	// 2 * 500cp = 1000cp = 10gp
	if coinNotes != "10gp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "10gp")
	}
}

func TestSellBundledPartial(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// 40 arrows (2 bundles of 20). Store price 5gp = 500cp per bundle, sell = 250cp per bundle.
	arrows := &db.Item{CharacterID: ch.ID, Name: "Arrows", Quantity: 40, Location: "stowed"}
	d.CreateItem(arrows)

	// Sell 20 (1 bundle worth)
	form := url.Values{"quantity": {"20"}, "move_to": {"sell"}}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", arrows.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	var arrowQty int
	var coinNotes string
	for _, item := range items {
		if item.Name == "Arrows" {
			arrowQty = item.Quantity
		}
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	if arrowQty != 20 {
		t.Errorf("remaining arrows = %d, want 20", arrowQty)
	}
	// 250cp = 2gp 5sp
	if coinNotes != "2gp 5sp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "2gp 5sp")
	}
}

func TestSellFullStackScalesPrice(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// 40 arrows: sell all should give 500cp (2 bundles * 250cp each), not 250cp
	arrows := &db.Item{CharacterID: ch.ID, Name: "Arrows", Quantity: 40, Location: "stowed"}
	d.CreateItem(arrows)

	form := url.Values{}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/sell/", arrows.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	for _, item := range items {
		if item.Name == "Arrows" {
			t.Fatal("arrows should be deleted after selling all")
		}
	}
	var coinNotes string
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	// 40 arrows / 20 bundle * 250cp = 500cp = 5gp
	if coinNotes != "5gp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "5gp")
	}
}

func TestDeductCoinsPreservesPP(t *testing.T) {
	srv, d := setupTest(t)

	ch := &db.Character{
		Name: "Rich", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Give character 2pp 5gp (1000 + 500 = 1500cp total)
	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        engine.CoinItemNameStr,
		Quantity:    7,
		Notes:       "2pp 5gp",
	})

	// Buy something for 100cp (1gp)
	oldWealth, newWealth, err := srv.deductCoins(ch, 100)
	if err != nil {
		t.Fatalf("deductCoins: %v", err)
	}

	items, _ := d.ListItems(ch.ID)
	var coinNotes string
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	// PP should be preserved: 2pp 5gp - 1gp = 2pp 4gp
	if coinNotes != "2pp 4gp" {
		t.Errorf("coin notes = %q, want %q", coinNotes, "2pp 4gp")
	}
	if oldWealth != "15gp" {
		t.Errorf("oldWealth = %q, want %q", oldWealth, "15gp")
	}
	if newWealth != "14gp" {
		t.Errorf("newWealth = %q, want %q", newWealth, "14gp")
	}
}

func TestSellDoesNotCreatePP(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Seller", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Longsword sell price = 500cp. Should yield 5gp, not 1pp.
	sword := &db.Item{CharacterID: ch.ID, Name: "Longsword", Quantity: 1, Location: "stowed"}
	d.CreateItem(sword)

	form := url.Values{}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/sell/", sword.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	var coinNotes string
	for _, item := range items {
		if item.Name == "Coins" {
			coinNotes = item.Notes
		}
	}
	if coinNotes != "5gp" {
		t.Errorf("coin notes = %q, want %q (PP should never appear in changemaking)", coinNotes, "5gp")
	}
}

func TestBuyAuditLogShowsWealth(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Buyer", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Give 50gp (5000cp)
	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        engine.CoinItemNameStr,
		Quantity:    50,
		Notes:       "50gp",
	})

	// Buy a Longsword (10gp = 1000cp)
	form := url.Values{"item_id": {"Longsword|1000"}}
	req := httptest.NewRequest("POST", "/characters/1/store/buy/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	auditLog, _ := d.ListAuditLog(ch.ID)
	var buyDetail string
	for _, entry := range auditLog {
		if entry.Action == "store_buy" {
			buyDetail = entry.Detail
		}
	}
	if buyDetail == "" {
		t.Fatal("expected store_buy audit log entry")
	}
	// Should contain wealth transition
	if !strings.Contains(buyDetail, "wealth 50gp -> 40gp") {
		t.Errorf("buy audit log should show wealth transition, got %q", buyDetail)
	}
}

func TestNegativeQuantityDeductsFromInventory(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Start with 10 feed
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Feed", Quantity: 10, Location: "stowed"})

	// Deduct 2 feed via "-2x Feed"
	form := url.Values{}
	form.Set("name", "-2x Feed")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/items/", ch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 8 {
		t.Errorf("Quantity = %d, want 8", items[0].Quantity)
	}
}

func TestNegativeQuantityDeletesItemAtZero(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Start with 3 feed
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Feed", Quantity: 3, Location: "stowed"})

	// Deduct all 3
	form := url.Values{}
	form.Set("name", "-3x Feed")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/items/", ch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (item should be deleted)", len(items))
	}
}

func TestNegativeQuantityInsufficientStock(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Start with 2 feed
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Feed", Quantity: 2, Location: "stowed"})

	// Try to deduct 5 — should fail
	form := url.Values{}
	form.Set("name", "-5x Feed")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/items/", ch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Item should be unchanged
	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 2 {
		t.Errorf("Quantity = %d, want 2 (unchanged)", items[0].Quantity)
	}
}

func TestNegativeQuantityWithParenthesizedName(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Start with 5 rations (preserved) — using the exact store name
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Rations (preserved)", Quantity: 5})

	// Deduct 3 via "-3x rations (preserved)" with move_to=equipped (matches real form)
	form := url.Values{}
	form.Set("name", "-3x rations (preserved)")
	form.Set("move_to", "equipped")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/items/", ch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 2 {
		t.Errorf("Quantity = %d, want 2", items[0].Quantity)
	}
}

func TestAddTownsfolkRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		CHA: 14, // modifier +1
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "Torchbearer")
	form.Set("breed", "Townsfolk")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/companions/", ch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 1 {
		t.Fatalf("got %d companions, want 1", len(comps))
	}
	got := comps[0]
	if got.Breed != "Townsfolk" {
		t.Errorf("Breed = %q, want %q", got.Breed, "Townsfolk")
	}
	if got.HPMax != 2 {
		t.Errorf("HPMax = %d, want 2", got.HPMax)
	}
	// CHA 14 → modifier +1, loyalty = 7 + 1 = 8
	if got.Loyalty != 8 {
		t.Errorf("Loyalty = %d, want 8", got.Loyalty)
	}
}

func TestUpdateRetainerLoyalty(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		CHA: 10,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID, Name: "Porter", Breed: "Townsfolk",
		HPCurrent: 2, HPMax: 2, Loyalty: 7,
	}
	d.CreateCompanion(comp)

	form := url.Values{}
	form.Set("name", "Porter")
	form.Set("hp_current", "2")
	form.Set("hp_max", "2")
	form.Set("loyalty", "9")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/companions/%d/update/", ch.ID, comp.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	updated, _ := d.GetCompanion(comp.ID)
	if updated.Loyalty != 9 {
		t.Errorf("Loyalty = %d, want 9", updated.Loyalty)
	}
}

func TestHireAdventurerRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:       "Employer",
		Class:      "Knight",
		Kindred:    "Human",
		Level:      2,
		HPCurrent:  12,
		HPMax:      12,
		CHA:        12,
		CurrentDay: 8,
	}
	d.CreateCharacter(employer)

	form := url.Values{}
	form.Set("name", "Rowan")
	form.Set("class", "Fighter")
	form.Set("kindred", "Human")
	form.Set("str", "13")
	form.Set("dex", "10")
	form.Set("con", "12")
	form.Set("int", "9")
	form.Set("wis", "11")
	form.Set("cha", "8")
	form.Set("hp_max", "6")
	form.Set("alignment", "Neutral")
	form.Set("loot_share_pct", "20")
	form.Set("xp_share_pct", "50")
	form.Set("daily_wage_cp", "12")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/", employer.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	contracts, err := d.ListActiveRetainerContracts(employer.ID)
	if err != nil {
		t.Fatalf("ListActiveRetainerContracts: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}
	contract := contracts[0]
	if contract.LootSharePct != 20 {
		t.Errorf("LootSharePct = %v, want 20", contract.LootSharePct)
	}
	if contract.XPSharePct != 50 {
		t.Errorf("XPSharePct = %v, want 50", contract.XPSharePct)
	}
	if contract.DailyWageCP != 12 {
		t.Errorf("DailyWageCP = %d, want 12", contract.DailyWageCP)
	}
	if contract.HiredOnDay != employer.CurrentDay {
		t.Errorf("HiredOnDay = %d, want %d", contract.HiredOnDay, employer.CurrentDay)
	}

	retainer, err := d.GetCharacter(contract.RetainerID)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	if retainer.Name != "Rowan" {
		t.Errorf("retainer name = %q, want Rowan", retainer.Name)
	}
	if retainer.Class != "Fighter" {
		t.Errorf("retainer class = %q, want Fighter", retainer.Class)
	}
	if retainer.Kindred != "Human" {
		t.Errorf("retainer kindred = %q, want Human", retainer.Kindred)
	}
	if retainer.Level != 1 {
		t.Errorf("retainer level = %d, want 1", retainer.Level)
	}
	if retainer.HPMax != 6 || retainer.HPCurrent != 6 {
		t.Errorf("retainer HP = %d/%d, want 6/6", retainer.HPCurrent, retainer.HPMax)
	}

	employerLog, _ := d.ListAuditLog(employer.ID)
	foundEmployer := false
	for _, entry := range employerLog {
		if entry.Action == "retainer_hire" && strings.Contains(entry.Detail, "Rowan") {
			foundEmployer = true
		}
	}
	if !foundEmployer {
		t.Fatal("expected retainer_hire audit log entry on employer")
	}

	retainerLog, _ := d.ListAuditLog(retainer.ID)
	foundRetainer := false
	for _, entry := range retainerLog {
		if entry.Action == "retainer_hired" && strings.Contains(entry.Detail, "Employer") {
			foundRetainer = true
		}
	}
	if !foundRetainer {
		t.Fatal("expected retainer_hired audit log entry on retainer")
	}
}

func TestDismissRetainerContract(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:       "Employer",
		Class:      "Knight",
		Kindred:    "Human",
		Level:      2,
		HPCurrent:  12,
		HPMax:      12,
		CHA:        12,
		CurrentDay: 8,
	}
	d.CreateCharacter(employer)

	retainer := &db.Character{
		Name:      "Rowan",
		Class:     "Fighter",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 6,
		HPMax:     6,
	}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{
		EmployerID:   employer.ID,
		RetainerID:   retainer.ID,
		LootSharePct: 15,
		XPSharePct:   50,
		DailyWageCP:  0,
		HiredOnDay:   employer.CurrentDay,
		Active:       true,
	}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/dismiss/", employer.ID, contract.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	updated, err := d.GetRetainerContract(contract.ID)
	if err != nil {
		t.Fatalf("GetRetainerContract: %v", err)
	}
	if updated.Active {
		t.Fatal("expected contract to be inactive")
	}

	if _, err := d.GetCharacter(retainer.ID); err != nil {
		t.Fatalf("retainer should still exist: %v", err)
	}

	employerLog, _ := d.ListAuditLog(employer.ID)
	foundEmployer := false
	for _, entry := range employerLog {
		if entry.Action == "retainer_dismiss" && strings.Contains(entry.Detail, "Rowan") {
			foundEmployer = true
		}
	}
	if !foundEmployer {
		t.Fatal("expected retainer_dismiss audit log entry on employer")
	}

	retainerLog, _ := d.ListAuditLog(retainer.ID)
	foundRetainer := false
	for _, entry := range retainerLog {
		if entry.Action == "retainer_dismissed" && strings.Contains(entry.Detail, "Employer") {
			foundRetainer = true
		}
	}
	if !foundRetainer {
		t.Fatal("expected retainer_dismissed audit log entry on retainer")
	}
}

func TestUpdateRetainerContract(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:       "Employer",
		Class:      "Knight",
		Kindred:    "Human",
		Level:      2,
		HPCurrent:  12,
		HPMax:      12,
		CHA:        12,
		CurrentDay: 8,
	}
	d.CreateCharacter(employer)

	retainer := &db.Character{
		Name:      "Rowan",
		Class:     "Fighter",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 6,
		HPMax:     6,
	}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{
		EmployerID:   employer.ID,
		RetainerID:   retainer.ID,
		LootSharePct: 15,
		XPSharePct:   50,
		DailyWageCP:  0,
		HiredOnDay:   employer.CurrentDay,
		Active:       true,
	}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	form := url.Values{}
	form.Set("loot_share_pct", "25")
	form.Set("xp_share_pct", "40")
	form.Set("daily_wage_cp", "12")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/update/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	updated, err := d.GetRetainerContract(contract.ID)
	if err != nil {
		t.Fatalf("GetRetainerContract: %v", err)
	}
	if updated.LootSharePct != 25 {
		t.Errorf("LootSharePct = %v, want 25", updated.LootSharePct)
	}
	if updated.XPSharePct != 40 {
		t.Errorf("XPSharePct = %v, want 40", updated.XPSharePct)
	}
	if updated.DailyWageCP != 12 {
		t.Errorf("DailyWageCP = %d, want 12", updated.DailyWageCP)
	}
}

func TestTransferItemGiveToRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:      "Employer",
		Class:     "Knight",
		Kindred:   "Human",
		Level:     1,
		CHA:       12,
		HPCurrent: 8,
		HPMax:     8,
	}
	d.CreateCharacter(employer)

	retainer := &db.Character{
		Name:      "Rowan",
		Class:     "Fighter",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 6,
		HPMax:     6,
	}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{
		EmployerID:   employer.ID,
		RetainerID:   retainer.ID,
		LootSharePct: 15,
		XPSharePct:   50,
		DailyWageCP:  0,
		HiredOnDay:   1,
		Active:       true,
	}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Rope", Quantity: 1, Location: "stowed"}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("quantity", "0")
	form.Set("direction", "give")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	employerItems, _ := d.ListItems(employer.ID)
	if len(employerItems) != 0 {
		t.Fatalf("employer items = %d, want 0", len(employerItems))
	}
	retainerItems, _ := d.ListItems(retainer.ID)
	if len(retainerItems) != 1 {
		t.Fatalf("retainer items = %d, want 1", len(retainerItems))
	}
	if retainerItems[0].Name != "Rope" {
		t.Errorf("retainer item name = %q, want Rope", retainerItems[0].Name)
	}

	employerLog, _ := d.ListAuditLog(employer.ID)
	retainerLog, _ := d.ListAuditLog(retainer.ID)
	if len(employerLog) == 0 || len(retainerLog) == 0 {
		t.Fatalf("expected audit logs for employer and retainer")
	}
	foundGive := false
	for _, entry := range employerLog {
		if entry.Action == "retainer_transfer" && strings.Contains(entry.Detail, "gave") && strings.Contains(entry.Detail, " to ") && strings.Contains(entry.Detail, retainer.Name) {
			foundGive = true
		}
	}
	if !foundGive {
		t.Errorf("expected employer transfer audit log")
	}
	foundReceive := false
	for _, entry := range retainerLog {
		if entry.Action == "retainer_transfer" && strings.Contains(entry.Detail, "received") && strings.Contains(entry.Detail, " from ") && strings.Contains(entry.Detail, employer.Name) {
			foundReceive = true
		}
	}
	if !foundReceive {
		t.Errorf("expected retainer transfer audit log")
	}
}

func TestTransferItemTakeFromRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:      "Employer",
		Class:     "Knight",
		Kindred:   "Human",
		Level:     1,
		CHA:       12,
		HPCurrent: 8,
		HPMax:     8,
	}
	d.CreateCharacter(employer)

	retainer := &db.Character{
		Name:      "Rowan",
		Class:     "Fighter",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 6,
		HPMax:     6,
	}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{
		EmployerID:   employer.ID,
		RetainerID:   retainer.ID,
		LootSharePct: 15,
		XPSharePct:   50,
		DailyWageCP:  0,
		HiredOnDay:   1,
		Active:       true,
	}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: retainer.ID, Name: "Torch", Quantity: 1, Location: "stowed"}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("quantity", "0")
	form.Set("direction", "take")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	employerItems, _ := d.ListItems(employer.ID)
	if len(employerItems) != 1 {
		t.Fatalf("employer items = %d, want 1", len(employerItems))
	}
	if employerItems[0].Name != "Torch" {
		t.Errorf("employer item name = %q, want Torch", employerItems[0].Name)
	}
	retainerItems, _ := d.ListItems(retainer.ID)
	if len(retainerItems) != 0 {
		t.Fatalf("retainer items = %d, want 0", len(retainerItems))
	}
}

func TestTransferItemPartialToRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{
		Name:      "Employer",
		Class:     "Knight",
		Kindred:   "Human",
		Level:     1,
		CHA:       12,
		HPCurrent: 8,
		HPMax:     8,
	}
	d.CreateCharacter(employer)

	retainer := &db.Character{
		Name:      "Rowan",
		Class:     "Fighter",
		Kindred:   "Human",
		Level:     1,
		HPCurrent: 6,
		HPMax:     6,
	}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{
		EmployerID:   employer.ID,
		RetainerID:   retainer.ID,
		LootSharePct: 15,
		XPSharePct:   50,
		DailyWageCP:  0,
		HiredOnDay:   1,
		Active:       true,
	}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Torches", Quantity: 5, Location: "stowed"}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("quantity", "2")
	form.Set("direction", "give")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	employerItems, _ := d.ListItems(employer.ID)
	if len(employerItems) != 1 {
		t.Fatalf("employer items = %d, want 1", len(employerItems))
	}
	if employerItems[0].Quantity != 3 {
		t.Errorf("employer torches qty = %d, want 3", employerItems[0].Quantity)
	}
	retainerItems, _ := d.ListItems(retainer.ID)
	if len(retainerItems) != 1 {
		t.Fatalf("retainer items = %d, want 1", len(retainerItems))
	}
	if retainerItems[0].Quantity != 2 {
		t.Errorf("retainer torches qty = %d, want 2", retainerItems[0].Quantity)
	}
}

func TestAddItemToRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	d.CreateCharacter(employer)
	retainer := &db.Character{Name: "Rowan", Class: "Fighter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	form := url.Values{}
	form.Set("name", "Rope")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/items/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	employerItems, _ := d.ListItems(employer.ID)
	if len(employerItems) != 0 {
		t.Fatalf("employer items = %d, want 0", len(employerItems))
	}
	retainerItems, _ := d.ListItems(retainer.ID)
	if len(retainerItems) != 1 {
		t.Fatalf("retainer items = %d, want 1", len(retainerItems))
	}
	if retainerItems[0].Name != "Rope" {
		t.Errorf("retainer item name = %q, want Rope", retainerItems[0].Name)
	}

	retainerLog, _ := d.ListAuditLog(retainer.ID)
	foundAdd := false
	for _, entry := range retainerLog {
		if entry.Action == "item_add" && strings.Contains(entry.Detail, "Rope") {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		t.Fatalf("expected item_add audit log entry for retainer")
	}
}

func TestSplitItemMovesToRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	d.CreateCharacter(employer)
	retainer := &db.Character{Name: "Rowan", Class: "Fighter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Torches", Quantity: 5, Location: "stowed"}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("quantity", "2")
	form.Set("move_to", fmt.Sprintf("retainer:%d", contract.ID))

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/items/%d/split/", employer.ID, item.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	employerItems, _ := d.ListItems(employer.ID)
	if len(employerItems) != 1 {
		t.Fatalf("employer items = %d, want 1", len(employerItems))
	}
	if employerItems[0].Quantity != 3 {
		t.Errorf("employer torches qty = %d, want 3", employerItems[0].Quantity)
	}

	retainerItems, _ := d.ListItems(retainer.ID)
	if len(retainerItems) != 1 {
		t.Fatalf("retainer items = %d, want 1", len(retainerItems))
	}
	if retainerItems[0].Quantity != 2 {
		t.Errorf("retainer torches qty = %d, want 2", retainerItems[0].Quantity)
	}

	employerLog, _ := d.ListAuditLog(employer.ID)
	retainerLog, _ := d.ListAuditLog(retainer.ID)
	foundEmployer := false
	for _, entry := range employerLog {
		if entry.Action == "retainer_transfer" && strings.Contains(entry.Detail, "gave") && strings.Contains(entry.Detail, " to ") && strings.Contains(entry.Detail, "Rowan") {
			foundEmployer = true
		}
	}
	if !foundEmployer {
		t.Fatal("expected retainer_transfer audit log entry on employer")
	}
	foundRetainer := false
	for _, entry := range retainerLog {
		if entry.Action == "retainer_transfer" && strings.Contains(entry.Detail, "received") && strings.Contains(entry.Detail, " from ") && strings.Contains(entry.Detail, "Employer") {
			foundRetainer = true
		}
	}
	if !foundRetainer {
		t.Fatal("expected retainer_transfer audit log entry on retainer")
	}
}

func TestSplitCoinsMoveToRetainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	d.CreateCharacter(employer)
	retainer := &db.Character{Name: "Rowan", Class: "Fighter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	coins := &db.Item{CharacterID: employer.ID, Name: engine.CoinItemNameStr, Quantity: 5, Notes: "5gp", Location: "stowed"}
	d.CreateItem(coins)

	form := url.Values{}
	form.Set("quantity", "2gp")
	form.Set("move_to", fmt.Sprintf("retainer:%d", contract.ID))

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/items/%d/split/", employer.ID, coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	employerItems, _ := d.ListItems(employer.ID)
	if len(employerItems) != 1 {
		t.Fatalf("employer items = %d, want 1", len(employerItems))
	}
	if employerItems[0].Notes != "3gp" {
		t.Errorf("employer coin notes = %q, want 3gp", employerItems[0].Notes)
	}

	retainerItems, _ := d.ListItems(retainer.ID)
	if len(retainerItems) != 1 {
		t.Fatalf("retainer items = %d, want 1", len(retainerItems))
	}
	if retainerItems[0].Notes != "2gp" {
		t.Errorf("retainer coin notes = %q, want 2gp", retainerItems[0].Notes)
	}
}

func TestTransferItemRejectsInvalidDirectionV2(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	d.CreateCharacter(employer)
	retainer := &db.Character{Name: "Rowan", Class: "Fighter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("quantity", "0")
	form.Set("direction", "flip")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTransferItemRejectsItemNotOwnedBySource(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	employer := &db.Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	d.CreateCharacter(employer)
	retainer := &db.Character{Name: "Rowan", Class: "Fighter", Kindred: "Human", Level: 1, HPCurrent: 6, HPMax: 6}
	d.CreateCharacter(retainer)

	contract := &db.RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := d.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	item := &db.Item{CharacterID: employer.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(item)

	form := url.Values{}
	form.Set("item_id", fmt.Sprintf("%d", item.ID))
	form.Set("quantity", "0")
	form.Set("direction", "take")

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/%d/retainers/%d/transfer/", employer.ID, contract.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestConsumeItemViaMove(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	feed := &db.Item{CharacterID: ch.ID, Name: "Feed", Quantity: 5, Location: "stowed"}
	d.CreateItem(feed)

	// Consume 2 via split handler with move_to=consume
	form := url.Values{"quantity": {"2"}, "move_to": {"consume"}}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", feed.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	var feedQty int
	for _, item := range items {
		if item.Name == "Feed" {
			feedQty = item.Quantity
		}
	}
	if feedQty != 3 {
		t.Errorf("remaining feed = %d, want 3", feedQty)
	}

	// Audit log should have item_consume
	auditLog, _ := d.ListAuditLog(ch.ID)
	var consumeDetail string
	for _, entry := range auditLog {
		if entry.Action == "item_consume" {
			consumeDetail = entry.Detail
		}
	}
	if consumeDetail == "" {
		t.Fatal("expected item_consume audit log entry")
	}
	if !strings.Contains(consumeDetail, "Feed") || !strings.Contains(consumeDetail, "2") {
		t.Errorf("consume audit log should mention item and qty, got %q", consumeDetail)
	}
}

func TestConsumeEntireItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	torch := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 1, Location: "equipped"}
	d.CreateItem(torch)

	// Consume single-qty item via update handler with move_to=consume
	form := url.Values{"move_to": {"consume"}}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", torch.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	items, _ := d.ListItems(ch.ID)
	for _, item := range items {
		if item.Name == "Torch" {
			t.Fatal("torch should have been deleted after consuming")
		}
	}

	// Audit log should have item_consume
	auditLog, _ := d.ListAuditLog(ch.ID)
	found := false
	for _, entry := range auditLog {
		if entry.Action == "item_consume" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected item_consume audit log entry")
	}
}

func TestConsumeItemRejectsCoins(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	coins := &db.Item{CharacterID: ch.ID, Name: engine.CoinItemNameStr, Quantity: 5, Notes: "5gp", Location: "stowed"}
	d.CreateItem(coins)

	// Try to consume coins via split handler — should be rejected
	form := url.Values{"quantity": {"5gp"}, "move_to": {"consume"}}
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/split/", coins.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Coins should still exist
	items, _ := d.ListItems(ch.ID)
	var coinFound bool
	for _, item := range items {
		if item.Name == engine.CoinItemNameStr {
			coinFound = true
		}
	}
	if !coinFound {
		t.Fatal("coins should not have been consumed")
	}
}

func TestBardingShowsACBonusPill(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID, Name: "Bessie", Breed: "Mule",
		HPCurrent: 9, HPMax: 9,
	}
	d.CreateCompanion(comp)

	d.CreateItem(&db.Item{
		CharacterID: ch.ID, Name: "Horse barding",
		Quantity: 1, CompanionID: &comp.ID,
	})

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	// The inventory should show "+2 AC" pill on the barding item
	bardingIdx := strings.Index(body, "Horse barding")
	if bardingIdx == -1 {
		t.Fatal("Horse barding not found in body")
	}
	// Look in a reasonable region after the item name
	region := body[bardingIdx : bardingIdx+500]
	if !strings.Contains(region, "+2 AC") {
		t.Errorf("expected '+2 AC' pill near Horse barding item, got: %s", region)
	}
}

func TestWealthCardConsolidatesAllWealthLogs(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Create audit log entries for all wealth-affecting actions
	d.AddAuditLog(ch.ID, "treasure_add", "10 gp dragon hoard (found)", 0)
	d.AddAuditLog(ch.ID, "treasure_undo", "undo dragon hoard", 0)
	d.AddAuditLog(ch.ID, "return_to_safety", "returned to safety", 0)
	d.AddAuditLog(ch.ID, "store_buy", "bought Longsword for 10gp", 0)
	d.AddAuditLog(ch.ID, "store_sell", "sold Longsword for 5gp", 0)
	d.AddAuditLog(ch.ID, "bank_deposit", "deposit 10gp to bank", 0)
	d.AddAuditLog(ch.ID, "bank_withdraw", "withdrew 5gp from bank", 0)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Wealth card should have a single "Wealth Log" containing all entries
	if !strings.Contains(body, "Wealth Log") {
		t.Error("wealth card should contain 'Wealth Log'")
	}
	for _, detail := range []string{
		"dragon hoard", "undo dragon hoard", "returned to safety",
		"bought Longsword", "sold Longsword",
		"deposit 10gp", "withdrew 5gp",
	} {
		if !strings.Contains(body, detail) {
			t.Errorf("wealth log should contain %q", detail)
		}
	}

	// Store card should NOT have its own log
	if strings.Contains(body, "Store Log") {
		t.Error("store card should not have its own 'Store Log' — entries should be in wealth log")
	}

	// Bank card should NOT have its own log
	if strings.Contains(body, "Bank Log") {
		t.Error("bank card should not have its own 'Bank Log' — entries should be in wealth log")
	}
}
