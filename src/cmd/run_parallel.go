package cmd

import (
	"os"
	"strings"
)

func checkCriterionInFile(filePath, criterionText string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	oldLine := "- [ ] " + criterionText
	newLine := "- [x] " + criterionText
	content := string(data)
	if !strings.Contains(content, oldLine) {
		return nil
	}
	content = strings.Replace(content, oldLine, newLine, 1)
	return os.WriteFile(filePath, []byte(content), 0o644)
}
