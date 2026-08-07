package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct {
	IP       string
	User     string
	Password string
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("cannot open configuration file %q: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return Config{}, fmt.Errorf("invalid configuration line %d: expected KEY=VALUE", lineNumber)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "IP" && key != "USER" && key != "PASSWRD" {
			return Config{}, fmt.Errorf("unknown configuration key %q on line %d", key, lineNumber)
		}
		if _, exists := values[key]; exists {
			return Config{}, fmt.Errorf("duplicate configuration key %q on line %d", key, lineNumber)
		}
		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("cannot read configuration file %q: %w", path, err)
	}

	for _, key := range []string{"IP", "USER", "PASSWRD"} {
		if values[key] == "" {
			return Config{}, fmt.Errorf("configuration value %s must not be empty", key)
		}
	}

	if net.ParseIP(values["IP"]) == nil || strings.Contains(values["IP"], ":") {
		return Config{}, fmt.Errorf("configuration value IP must be a valid IPv4 address")
	}
	if values["USER"] != "root" {
		return Config{}, fmt.Errorf("configuration value USER must be root")
	}

	return Config{IP: values["IP"], User: values["USER"], Password: values["PASSWRD"]}, nil
}
