package natter

import (
	"bufio"
	"os"
	"regexp"
)

func loadRawConfig(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rawconfig := make(map[string]string)
	scanner := bufio.NewScanner(file)

	comment := regexp.MustCompile(`^\s*#`)
	value := regexp.MustCompile(`^\s*(\S+)\s+(.*)$`)

	for scanner.Scan() {
		line := scanner.Text()

		if !comment.MatchString(line) {
			parts := value.FindStringSubmatch(line)

			if len(parts) == 3 {
				rawconfig[parts[1]] = parts[2]
			}
		}
	}

	return rawconfig, nil
}

