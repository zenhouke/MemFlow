package utils

func UniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range strs {
		if !seen[s] && s != "" {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
