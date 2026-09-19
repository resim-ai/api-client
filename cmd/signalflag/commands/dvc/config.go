package dvc

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// lookupRemoteURL reads the repository's DVC config (.dvc/config, overridden
// by .dvc/config.local) and returns the URL of the named remote, which must be
// an s3:// URL.
func lookupRemoteURL(repoRoot string, remoteName string) (*url.URL, error) {
	if remoteName == "" {
		return nil, fmt.Errorf("no DVC remote name provided")
	}
	remotes := map[string]string{}
	for _, name := range []string{"config", "config.local"} {
		configPath := filepath.Join(repoRoot, ".dvc", name)
		if err := readRemoteURLs(configPath, remotes); err != nil {
			return nil, err
		}
	}
	rawURL, ok := remotes[remoteName]
	if !ok {
		return nil, fmt.Errorf("DVC remote %q not found in %s", remoteName, filepath.Join(repoRoot, ".dvc", "config"))
	}
	remoteURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL %q of DVC remote %q: %w", rawURL, remoteName, err)
	}
	if !strings.EqualFold(remoteURL.Scheme, "s3") || remoteURL.Host == "" {
		return nil, fmt.Errorf("DVC remote %q is not an S3 remote (url: %q): only s3:// remotes are supported", remoteName, rawURL)
	}
	return remoteURL, nil
}

// readRemoteURLs parses a DVC config file (INI-style)
// and adds every [remote "name"] section's url to remotes. A missing file is
// not an error, so config.local can simply be absent.
func readRemoteURLs(configPath string, remotes map[string]string) error {
	file, err := os.Open(configPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read DVC config %q: %w", configPath, err)
	}
	defer file.Close()

	currentRemote := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentRemote = parseRemoteSection(line)
			continue
		}
		if currentRemote == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(key) == "url" {
			remotes[currentRemote] = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("could not read DVC config %q: %w", configPath, err)
	}
	return nil
}

// parseRemoteSection extracts the remote name from a section header like
// ['remote "storage"'] (configobj quotes the whole section name) or
// [remote "storage"], returning "" for any other section.
func parseRemoteSection(line string) string {
	section := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
	section = strings.TrimSpace(section)
	// configobj wraps section names containing spaces in single quotes.
	if len(section) >= 2 && section[0] == '\'' && section[len(section)-1] == '\'' {
		section = section[1 : len(section)-1]
	}
	rest, found := strings.CutPrefix(section, "remote ")
	if !found {
		return ""
	}
	rest = strings.TrimSpace(rest)
	return strings.Trim(rest, `"`)
}
