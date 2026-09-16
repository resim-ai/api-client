package commands

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// EnvPrefix is the prefix for environment variables that configure the CLI,
// e.g. SIGNALFLAG_CLIENT_ID.
const EnvPrefix = "SIGNALFLAG"

// LegacyEnvPrefix is the prefix the CLI used before it was renamed. Variables
// with this prefix are still honoured, with a deprecation warning, so that
// existing CI configurations keep working.
const LegacyEnvPrefix = "RESIM"

// PromoteLegacyEnv copies every RESIM_<NAME> environment variable to
// SIGNALFLAG_<NAME> when the latter is not already set, and writes a single
// deprecation warning to w listing the variables that were migrated. It must run
// before viper reads the environment.
func PromoteLegacyEnv(w io.Writer) {
	legacyPrefix := LegacyEnvPrefix + "_"
	var migrated []string
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(key, legacyPrefix) {
			continue
		}
		newKey := EnvPrefix + "_" + strings.TrimPrefix(key, legacyPrefix)
		if _, set := os.LookupEnv(newKey); set {
			continue
		}
		if err := os.Setenv(newKey, value); err != nil {
			continue
		}
		migrated = append(migrated, key)
	}
	if len(migrated) == 0 {
		return
	}
	sort.Strings(migrated)
	fmt.Fprintf(w, "WARNING: %s_* environment variables are deprecated and will be removed in a future release; rename them to %s_*: %s\n",
		LegacyEnvPrefix, EnvPrefix, strings.Join(migrated, ", "))
}
