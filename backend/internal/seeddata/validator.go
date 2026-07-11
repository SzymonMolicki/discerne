package seeddata

import (
	"fmt"
	"path/filepath"
)

var requiredLocales = []string{"pl-PL", "en-US", "es-ES"}

type Catalog struct {
	Families      []Family
	Groups        []Group
	Subgroups     []Subgroup
	Continents    []Continent
	Scripts       []Script
	Languages     []Language
	LanguageNames map[string]map[string]string
	Texts         map[string]TextFile
}

type Family struct {
	Code string
	Name string
}

type Group struct {
	Code   string
	Family string
	Name   string
}

type Subgroup struct {
	Code  string
	Group string
	Name  string
}

type Continent struct {
	Code string
	Name string
}

type Script struct {
	Code string
	Name string
}

type Language struct {
	ISO6393       string
	ISO6391       string
	CanonicalName string
	Family        string
	Group         string
	Subgroup      string
	Continent     string
	Script        string
	Enabled       bool
}

type TextFile struct {
	Language string
	Texts    []Text
}

type Text struct {
	Content       string
	SentenceCount int
	License       string
	Approved      bool
}

type Report struct {
	EnabledLanguages int
	ApprovedTexts    int
	Errors           []string
}

func Validate(dataDir string) (Report, error) {
	catalog, err := loadCatalog(filepath.Clean(dataDir))
	if err != nil {
		return Report{}, err
	}

	return validateCatalog(catalog), nil
}

func validateCatalog(catalog Catalog) Report {
	var report Report

	families := indexFamilies(catalog.Families, &report)
	groups := indexGroups(catalog.Groups, families, &report)
	subgroups := indexSubgroups(catalog.Subgroups, groups, &report)
	continents := indexContinents(catalog.Continents, &report)
	scripts := indexScripts(catalog.Scripts, &report)
	languages := indexLanguages(catalog.Languages, &report)

	for _, language := range catalog.Languages {
		if !language.Enabled {
			continue
		}

		report.EnabledLanguages++
		validateEnabledLanguage(language, families, groups, subgroups, continents, scripts, &report)
		validateLanguageNames(language, catalog.LanguageNames, &report)
		validateLanguageTexts(language, catalog.Texts, &report)
	}

	for code, names := range catalog.LanguageNames {
		if _, ok := languages[code]; !ok {
			report.Errors = append(report.Errors, fmt.Sprintf("language_names contains unknown language %q", code))
		}
		if len(names) == 0 {
			report.Errors = append(report.Errors, fmt.Sprintf("language_names for %q is empty", code))
		}
	}

	for fileCode, textFile := range catalog.Texts {
		if _, ok := languages[textFile.Language]; !ok {
			report.Errors = append(report.Errors, fmt.Sprintf("text file %q references unknown language %q", fileCode, textFile.Language))
		}
		if fileCode != textFile.Language {
			report.Errors = append(report.Errors, fmt.Sprintf("text file %q has language %q", fileCode, textFile.Language))
		}
	}

	return report
}

func indexFamilies(items []Family, report *Report) map[string]Family {
	index := make(map[string]Family, len(items))
	for _, item := range items {
		if item.Code == "" {
			report.Errors = append(report.Errors, "family has empty code")
			continue
		}
		if item.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("family %q has empty name", item.Code))
		}
		if _, exists := index[item.Code]; exists {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate family %q", item.Code))
		}
		index[item.Code] = item
	}
	return index
}

func indexGroups(items []Group, families map[string]Family, report *Report) map[string]Group {
	index := make(map[string]Group, len(items))
	for _, item := range items {
		if item.Code == "" {
			report.Errors = append(report.Errors, "group has empty code")
			continue
		}
		if item.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("group %q has empty name", item.Code))
		}
		if _, exists := families[item.Family]; !exists {
			report.Errors = append(report.Errors, fmt.Sprintf("group %q references unknown family %q", item.Code, item.Family))
		}
		if _, exists := index[item.Code]; exists {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate group %q", item.Code))
		}
		index[item.Code] = item
	}
	return index
}

func indexSubgroups(items []Subgroup, groups map[string]Group, report *Report) map[string]Subgroup {
	index := make(map[string]Subgroup, len(items))
	for _, item := range items {
		if item.Code == "" {
			report.Errors = append(report.Errors, "subgroup has empty code")
			continue
		}
		if item.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("subgroup %q has empty name", item.Code))
		}
		if _, exists := groups[item.Group]; !exists {
			report.Errors = append(report.Errors, fmt.Sprintf("subgroup %q references unknown group %q", item.Code, item.Group))
		}
		if _, exists := index[item.Code]; exists {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate subgroup %q", item.Code))
		}
		index[item.Code] = item
	}
	return index
}

func indexContinents(items []Continent, report *Report) map[string]Continent {
	index := make(map[string]Continent, len(items))
	for _, item := range items {
		if item.Code == "" {
			report.Errors = append(report.Errors, "continent has empty code")
			continue
		}
		if item.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("continent %q has empty name", item.Code))
		}
		if _, exists := index[item.Code]; exists {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate continent %q", item.Code))
		}
		index[item.Code] = item
	}
	return index
}

func indexScripts(items []Script, report *Report) map[string]Script {
	index := make(map[string]Script, len(items))
	for _, item := range items {
		if item.Code == "" {
			report.Errors = append(report.Errors, "script has empty code")
			continue
		}
		if item.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("script %q has empty name", item.Code))
		}
		if _, exists := index[item.Code]; exists {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate script %q", item.Code))
		}
		index[item.Code] = item
	}
	return index
}

func indexLanguages(items []Language, report *Report) map[string]Language {
	index := make(map[string]Language, len(items))
	for _, item := range items {
		if item.ISO6393 == "" {
			report.Errors = append(report.Errors, "language has empty iso_639_3")
			continue
		}
		if item.CanonicalName == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("language %q has empty canonical_name", item.ISO6393))
		}
		if _, exists := index[item.ISO6393]; exists {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate language %q", item.ISO6393))
		}
		index[item.ISO6393] = item
	}
	return index
}

func validateEnabledLanguage(
	language Language,
	families map[string]Family,
	groups map[string]Group,
	subgroups map[string]Subgroup,
	continents map[string]Continent,
	scripts map[string]Script,
	report *Report,
) {
	if _, exists := families[language.Family]; !exists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q references unknown family %q", language.ISO6393, language.Family))
	}

	group, groupExists := groups[language.Group]
	if !groupExists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q references unknown group %q", language.ISO6393, language.Group))
	} else if group.Family != language.Family {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q group %q belongs to family %q, not %q", language.ISO6393, language.Group, group.Family, language.Family))
	}

	subgroup, subgroupExists := subgroups[language.Subgroup]
	if !subgroupExists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q references unknown subgroup %q", language.ISO6393, language.Subgroup))
	} else if subgroup.Group != language.Group {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q subgroup %q belongs to group %q, not %q", language.ISO6393, language.Subgroup, subgroup.Group, language.Group))
	}

	if _, exists := continents[language.Continent]; !exists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q references unknown continent %q", language.ISO6393, language.Continent))
	}
	if _, exists := scripts[language.Script]; !exists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q references unknown script %q", language.ISO6393, language.Script))
	}
}

func validateLanguageNames(language Language, names map[string]map[string]string, report *Report) {
	localizedNames, exists := names[language.ISO6393]
	if !exists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q has no localized names", language.ISO6393))
		return
	}

	for _, locale := range requiredLocales {
		if localizedNames[locale] == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("language %q is missing localized name for %s", language.ISO6393, locale))
		}
	}
}

func validateLanguageTexts(language Language, texts map[string]TextFile, report *Report) {
	textFile, exists := texts[language.ISO6393]
	if !exists {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q has no text file", language.ISO6393))
		return
	}

	if textFile.Language != language.ISO6393 {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q text file declares language %q", language.ISO6393, textFile.Language))
	}

	approvedCount := 0
	for index, text := range textFile.Texts {
		position := index + 1
		if text.Content == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("language %q text %d has empty content", language.ISO6393, position))
		}
		if text.SentenceCount != 2 && text.SentenceCount != 3 {
			report.Errors = append(report.Errors, fmt.Sprintf("language %q text %d has sentence_count %d", language.ISO6393, position, text.SentenceCount))
		}
		if text.License == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("language %q text %d has empty license", language.ISO6393, position))
		}
		if text.Approved {
			approvedCount++
		}
	}

	if approvedCount != 5 {
		report.Errors = append(report.Errors, fmt.Sprintf("language %q has %d approved texts, want 5", language.ISO6393, approvedCount))
	}
	report.ApprovedTexts += approvedCount
}
