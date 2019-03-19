package natter

import (
	"bufio"
	"errors"
	"os"
	"regexp"
)

func LoadClientConfig(filename string) (*Config, error) {
	rawconfig, err := loadRawConfig(filename)
	if err != nil {
		return nil, err
	}

	clientUser, ok := rawconfig["ClientUser"]
	if !ok {
		return nil, errors.New("invalid config file, ClientUser setting is missing")
	}

	brokerAddr, ok := rawconfig["BrokerAddr"]
	if !ok {
		return nil, errors.New("invalid config file, Server setting is missing")
	}

	return &Config{
		ClientUser: clientUser,
		BrokerAddr: brokerAddr,
	}, nil
}

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
