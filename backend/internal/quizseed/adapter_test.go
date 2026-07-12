package quizseed

import (
	"testing"

	"discerne/backend/internal/seeddata"
)

func TestGenerationLanguagesConvertsSeedCatalog(t *testing.T) {
	catalog := seeddata.Catalog{
		Languages: []seeddata.Language{
			{
				ISO6393:   "spa",
				Family:    "indo-european",
				Group:     "italic",
				Subgroup:  "romance",
				Continent: "EU",
				Script:    "Latn",
				Enabled:   true,
			},
		},
		LanguageNames: map[string]map[string]string{
			"spa": {
				"pl-PL": "hiszpański",
				"en-US": "Spanish",
				"es-ES": "español",
			},
		},
		Texts: map[string]seeddata.TextFile{
			"spa": {
				Language: "spa",
				Texts: []seeddata.Text{
					{Content: "Primer texto.", Approved: true},
					{Content: "Segundo texto.", Approved: false},
				},
			},
		},
	}

	languages, err := GenerationLanguages(catalog)
	if err != nil {
		t.Fatalf("GenerationLanguages() error = %v", err)
	}

	if len(languages) != 1 {
		t.Fatalf("len(languages) = %d, want %d", len(languages), 1)
	}
	if languages[0].ID != "spa" {
		t.Fatalf("ID = %q, want %q", languages[0].ID, "spa")
	}
	if !languages[0].HasRequiredNames {
		t.Fatal("HasRequiredNames = false, want true")
	}
	if len(languages[0].Texts) != 2 {
		t.Fatalf("len(Texts) = %d, want %d", len(languages[0].Texts), 2)
	}
	if languages[0].Texts[0].ID != "spa-text-1" {
		t.Fatalf("Text ID = %q, want %q", languages[0].Texts[0].ID, "spa-text-1")
	}
}

func TestGenerationLanguagesRejectsMismatchedTextFile(t *testing.T) {
	catalog := seeddata.Catalog{
		Languages: []seeddata.Language{{ISO6393: "spa"}},
		Texts: map[string]seeddata.TextFile{
			"spa": {Language: "fra"},
		},
	}

	_, err := GenerationLanguages(catalog)
	if err == nil {
		t.Fatal("GenerationLanguages() error = nil, want error")
	}
}
