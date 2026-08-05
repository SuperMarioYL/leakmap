// Package secret — classification. Regex-first; ambiguous values can be
// promoted by an optional model classifier (ClassifyWithModel). The model
// path is a stub for v0.1: when no API endpoint/key is configured, it falls
// back to the regex verdict. This keeps the build runnable with no network
// or API keys, while leaving a clean seam for the DeepSeek/Qwen/GLM
// model_targets from the plan.
package secret

import (
	"regexp"
	"strings"
)

// Classification verdicts. They map directly onto LeakEvent severities.
const (
	ClassSecret = "secret"
	ClassConfig = "config"
	ClassState  = "state"
	ClassUnknown = "unknown"
)

// rules is an ordered list of (field pattern, value pattern, verdict). The
// first match wins; this mirrors how gitleaks/trufflehog order rules.
type rule struct {
	fieldRE *regexp.Regexp
	valueRE *regexp.Regexp
	verdict string
}

func compile(fieldPat, valPat, verdict string) rule {
	return rule{
		fieldRE: regexp.MustCompile(fieldPat),
		valueRE: regexp.MustCompile(valPat),
		verdict: verdict,
	}
}

// rules is ordered most-specific to least-specific. Field name carries a lot
// of signal (e.g. DB_TOKEN, AWS_SECRET_ACCESS_KEY), so field-anchored rules
// come first.
var rules = []rule{
	// High-signal field names → secret regardless of value shape.
	compile(`(?i)(token|secret|passwd|password|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|auth|bearer|jwt)`, `.+`, ClassSecret),
	// AWS-style secrets: AKIA... access keys and 40-char secret keys.
	compile(`(?i)(aws_access_key_id|access_key)`, `^AKIA[0-9A-Z]{16}$`, ClassSecret),
	compile(`(?i)(aws_secret_access_key|secret_access_key)`, `^[A-Za-z0-9/+=]{40}$`, ClassSecret),
	// Private key blocks.
	compile(`(?i).*`, `-----BEGIN [A-Z ]*PRIVATE KEY-----`, ClassSecret),
	// Generic high-entropy secret: long, mixed alnum, no spaces.
	compile(`(?i)(secret|token|key|credential)`, `^[A-Za-z0-9_\-+/=]{24,}$`, ClassSecret),
	// GitHub PAT / fine-grained tokens.
	compile(`(?i)(github|gh)`, `^(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}$`, ClassSecret),
	// Slack tokens.
	compile(`(?i)(slack)`, `^xox[abp]-[A-Za-z0-9-]+$`, ClassSecret),
	// DB connection DSN → config (contains structure but the password inside
	// is captured by the field-name rule above; the DSN itself is config).
	compile(`(?i)(dsn|database_url|db_url)`, `^[a-z]+://.+`, ClassConfig),
	// URLs without embedded credentials → config.
	compile(`(?i)(url|endpoint|host|registry|mirror)`, `^https?://`, ClassConfig),
	// Booleans / numbers / short port-like values → config.
	compile(`(?i)(port|timeout|retries|enabled|debug|verbose|tls|ssl)`, `^(true|false|[0-9]+)$`, ClassConfig),
	// Session / cache ids → state.
	compile(`(?i)(session[_-]?id|cache[_-]?key|request[_-]?id|trace[_-]?id|correlation[_-]?id)`, `.+`, ClassState),
}

// Classify applies the ordered regex rules and returns the first matching
// verdict, or ClassUnknown. Field is the env var name / file name; value is
// the raw material (only its shape is inspected, not stored).
func Classify(value, field string) string {
	for _, r := range rules {
		if r.fieldRE.MatchString(field) && r.valueRE.MatchString(value) {
			return r.verdict
		}
	}
	// Heuristic: a value that looks like a long opaque token but the field
	// name gave no signal is still "unknown" rather than secret — the model
	// classifier is the right place to promote these in v0.1+.
	return ClassUnknown
}

// ModelConfig describes an optional classification model endpoint. When
// Endpoint is empty, ClassifyWithModel is a pure no-op that returns the regex
// verdict unchanged. This is the stub seam for the DeepSeek-V3 / Qwen2.5 /
// GLM-4 model_targets: wired but inert until an operator supplies keys.
type ModelConfig struct {
	Endpoint string
	APIKey   string
	Model    string
}

// ClassifyWithModel runs the regex verdict first and, for ClassUnknown values
// only, asks the model to promote them to secret/config/state. With no model
// configured it returns the regex verdict immediately (the v0.1 happy path).
func ClassifyWithModel(value, field string, cfg ModelConfig) string {
	v := Classify(value, field)
	if v != ClassUnknown {
		return v
	}
	if cfg.Endpoint == "" || cfg.APIKey == "" {
		return v // stub: no model configured, leave as unknown
	}
	// NOTE: the real HTTP call lives here in a future iterate. Returning the
	// regex verdict keeps v0.1 offline and deterministic.
	return v
}

// NormalizeField lowercases and collapses separators for matching tolerance.
func NormalizeField(field string) string {
	s := strings.ToLower(field)
	s = strings.NewReplacer("-", "_", ".", "_").Replace(s)
	return s
}
