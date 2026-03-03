package textsplitters

import "strings"

func splitByMandatorySeparators(text string, separators []string) []string {
	normalized := normalizeMandatorySeparators(separators)
	if len(normalized) == 0 {
		return []string{text}
	}

	fragments := []string{text}
	for _, sep := range normalized {
		next := make([]string, 0, len(fragments))
		for _, fragment := range fragments {
			next = append(next, splitKeepMandatorySep(fragment, sep)...)
		}
		fragments = next
	}

	result := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		if strings.TrimSpace(fragment) == "" {
			continue
		}
		result = append(result, fragment)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeMandatorySeparators(separators []string) []string {
	if len(separators) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(separators))
	result := make([]string, 0, len(separators))
	for _, sep := range separators {
		if sep == "" {
			continue
		}
		if _, ok := seen[sep]; ok {
			continue
		}
		seen[sep] = struct{}{}
		result = append(result, sep)
	}
	return result
}

func splitKeepMandatorySep(text, sep string) []string {
	if text == "" || sep == "" || !strings.Contains(text, sep) {
		return []string{text}
	}

	parts := strings.Split(text, sep)
	result := make([]string, 0, len(parts))
	for i, part := range parts {
		piece := part
		if i < len(parts)-1 {
			piece += sep
		}
		if piece == "" {
			continue
		}
		result = append(result, piece)
	}
	if len(result) == 0 {
		return []string{text}
	}
	return result
}
