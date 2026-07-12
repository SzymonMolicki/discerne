package seeddata

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func DefaultDataDir() string {
	if fileExists(filepath.Join("data", "languages.yaml")) {
		return "data"
	}
	if fileExists(filepath.Join("backend", "data", "languages.yaml")) {
		return filepath.Join("backend", "data")
	}
	return "data"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func Load(dataDir string) (Catalog, error) {
	return loadCatalog(filepath.Clean(dataDir))
}

func loadCatalog(dataDir string) (Catalog, error) {
	var catalog Catalog

	families, err := readListFile(filepath.Join(dataDir, "families.yaml"), "families")
	if err != nil {
		return Catalog{}, err
	}
	for _, item := range families {
		catalog.Families = append(catalog.Families, Family{
			Code: item["code"],
			Name: item["name"],
		})
	}

	groups, err := readListFile(filepath.Join(dataDir, "groups.yaml"), "groups")
	if err != nil {
		return Catalog{}, err
	}
	for _, item := range groups {
		catalog.Groups = append(catalog.Groups, Group{
			Code:   item["code"],
			Family: item["family"],
			Name:   item["name"],
		})
	}

	subgroups, err := readListFile(filepath.Join(dataDir, "subgroups.yaml"), "subgroups")
	if err != nil {
		return Catalog{}, err
	}
	for _, item := range subgroups {
		catalog.Subgroups = append(catalog.Subgroups, Subgroup{
			Code:  item["code"],
			Group: item["group"],
			Name:  item["name"],
		})
	}

	continents, err := readListFile(filepath.Join(dataDir, "continents.yaml"), "continents")
	if err != nil {
		return Catalog{}, err
	}
	for _, item := range continents {
		catalog.Continents = append(catalog.Continents, Continent{
			Code: item["code"],
			Name: item["name"],
		})
	}

	scripts, err := readListFile(filepath.Join(dataDir, "scripts.yaml"), "scripts")
	if err != nil {
		return Catalog{}, err
	}
	for _, item := range scripts {
		catalog.Scripts = append(catalog.Scripts, Script{
			Code: item["iso_15924_code"],
			Name: item["name"],
		})
	}

	languages, err := readListFile(filepath.Join(dataDir, "languages.yaml"), "languages")
	if err != nil {
		return Catalog{}, err
	}
	for _, item := range languages {
		enabled, err := parseBool(item["enabled"])
		if err != nil {
			return Catalog{}, fmt.Errorf("parse enabled for language %q: %w", item["iso_639_3"], err)
		}

		catalog.Languages = append(catalog.Languages, Language{
			ISO6393:       item["iso_639_3"],
			ISO6391:       item["iso_639_1"],
			CanonicalName: item["canonical_name"],
			Family:        item["family"],
			Group:         item["group"],
			Subgroup:      item["subgroup"],
			Continent:     item["continent"],
			Script:        item["script"],
			Enabled:       enabled,
		})
	}

	names, err := readLanguageNames(filepath.Join(dataDir, "language_names.yaml"))
	if err != nil {
		return Catalog{}, err
	}
	catalog.LanguageNames = names

	texts, err := readTextFiles(filepath.Join(dataDir, "texts"))
	if err != nil {
		return Catalog{}, err
	}
	catalog.Texts = texts

	return catalog, nil
}

func readListFile(path string, root string) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var items []map[string]string
	var current map[string]string
	rootSeen := false

	for index, rawLine := range strings.Split(string(data), "\n") {
		lineNumber := index + 1
		line := strings.TrimRight(rawLine, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !rootSeen {
			if trimmed != root+":" {
				return nil, fmt.Errorf("%s:%d: expected root %q", path, lineNumber, root+":")
			}
			rootSeen = true
			continue
		}

		if strings.HasPrefix(line, "  - ") {
			if current != nil {
				items = append(items, current)
			}
			current = make(map[string]string)
			key, value, err := parseKeyValue(strings.TrimPrefix(line, "  - "))
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			current[key] = value
			continue
		}

		if strings.HasPrefix(line, "    ") {
			if current == nil {
				return nil, fmt.Errorf("%s:%d: field before list item", path, lineNumber)
			}
			key, value, err := parseKeyValue(strings.TrimSpace(line))
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			current[key] = value
			continue
		}

		return nil, fmt.Errorf("%s:%d: unsupported line %q", path, lineNumber, line)
	}

	if current != nil {
		items = append(items, current)
	}
	if !rootSeen {
		return nil, fmt.Errorf("%s: missing root %q", path, root+":")
	}

	return items, nil
}

func readLanguageNames(path string) (map[string]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	names := make(map[string]map[string]string)
	rootSeen := false
	currentLanguage := ""

	for index, rawLine := range strings.Split(string(data), "\n") {
		lineNumber := index + 1
		line := strings.TrimRight(rawLine, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !rootSeen {
			if trimmed != "language_names:" {
				return nil, fmt.Errorf("%s:%d: expected root %q", path, lineNumber, "language_names:")
			}
			rootSeen = true
			continue
		}

		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			if !strings.HasSuffix(trimmed, ":") {
				return nil, fmt.Errorf("%s:%d: expected language code", path, lineNumber)
			}
			currentLanguage = strings.TrimSuffix(trimmed, ":")
			names[currentLanguage] = make(map[string]string)
			continue
		}

		if strings.HasPrefix(line, "    ") {
			if currentLanguage == "" {
				return nil, fmt.Errorf("%s:%d: locale before language code", path, lineNumber)
			}
			key, value, err := parseKeyValue(strings.TrimSpace(line))
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			names[currentLanguage][key] = value
			continue
		}

		return nil, fmt.Errorf("%s:%d: unsupported line %q", path, lineNumber, line)
	}

	if !rootSeen {
		return nil, fmt.Errorf("%s: missing root %q", path, "language_names:")
	}

	return names, nil
}

func readTextFiles(textDir string) (map[string]TextFile, error) {
	paths, err := filepath.Glob(filepath.Join(textDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("find text files: %w", err)
	}

	texts := make(map[string]TextFile, len(paths))
	for _, path := range paths {
		textFile, err := readTextFile(path)
		if err != nil {
			return nil, err
		}
		code := strings.TrimSuffix(filepath.Base(path), ".yaml")
		texts[code] = textFile
	}

	return texts, nil
}

func readTextFile(path string) (TextFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TextFile{}, fmt.Errorf("read %s: %w", path, err)
	}

	var textFile TextFile
	var current map[string]string
	textsSeen := false

	for index, rawLine := range strings.Split(string(data), "\n") {
		lineNumber := index + 1
		line := strings.TrimRight(rawLine, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(line, "language:") {
			_, value, err := parseKeyValue(trimmed)
			if err != nil {
				return TextFile{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			textFile.Language = value
			continue
		}

		if trimmed == "texts:" {
			textsSeen = true
			continue
		}

		if !textsSeen {
			return TextFile{}, fmt.Errorf("%s:%d: expected texts root", path, lineNumber)
		}

		if strings.HasPrefix(line, "  - ") {
			if current != nil {
				text, err := buildText(current)
				if err != nil {
					return TextFile{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
				}
				textFile.Texts = append(textFile.Texts, text)
			}
			current = make(map[string]string)
			key, value, err := parseKeyValue(strings.TrimPrefix(line, "  - "))
			if err != nil {
				return TextFile{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			current[key] = value
			continue
		}

		if strings.HasPrefix(line, "    ") {
			if current == nil {
				return TextFile{}, fmt.Errorf("%s:%d: field before text item", path, lineNumber)
			}
			key, value, err := parseKeyValue(strings.TrimSpace(line))
			if err != nil {
				return TextFile{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			current[key] = value
			continue
		}

		return TextFile{}, fmt.Errorf("%s:%d: unsupported line %q", path, lineNumber, line)
	}

	if current != nil {
		text, err := buildText(current)
		if err != nil {
			return TextFile{}, fmt.Errorf("%s: %w", path, err)
		}
		textFile.Texts = append(textFile.Texts, text)
	}

	if textFile.Language == "" {
		return TextFile{}, fmt.Errorf("%s: missing language", path)
	}
	if !textsSeen {
		return TextFile{}, fmt.Errorf("%s: missing texts root", path)
	}

	return textFile, nil
}

func buildText(fields map[string]string) (Text, error) {
	sentenceCount, err := strconv.Atoi(fields["sentence_count"])
	if err != nil {
		return Text{}, fmt.Errorf("parse sentence_count: %w", err)
	}
	approved, err := parseBool(fields["approved"])
	if err != nil {
		return Text{}, fmt.Errorf("parse approved: %w", err)
	}

	return Text{
		Content:       fields["content"],
		SentenceCount: sentenceCount,
		License:       fields["license"],
		Approved:      approved,
	}, nil
}

func parseKeyValue(line string) (string, string, error) {
	key, rawValue, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", fmt.Errorf("expected key-value pair")
	}

	key = strings.TrimSpace(key)
	value := strings.TrimSpace(rawValue)
	if key == "" {
		return "", "", fmt.Errorf("empty key")
	}

	parsedValue, err := parseScalar(value)
	if err != nil {
		return "", "", err
	}

	return key, parsedValue, nil
}

func parseScalar(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "\"") {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("parse quoted value %q: %w", value, err)
		}
		return unquoted, nil
	}
	return value, nil
}

func parseBool(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", value)
	}
}
