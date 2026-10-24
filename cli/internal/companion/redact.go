package companion

import "regexp"

// Patterns for common credential shapes. Best-effort: redaction reduces
// accidental exposure but cannot guarantee every secret format is caught.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|\z)`),
	regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bsk-(?:proj-|ant-)?[A-Za-z0-9_\-]{20,}\b`),
	regexp.MustCompile(`\bsk_(?:live|test)_[A-Za-z0-9]{16,}\b`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bsb_(?:secret|publishable)_[A-Za-z0-9_\-]{12,}\b`),
	regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=\-]+`),
	regexp.MustCompile(`(?i)://[^\s:/@]+:[^\s@/]+@`),
}

// key=value / key: value where the key name suggests a secret.
var assignPattern = regexp.MustCompile(`(?i)(["']?\b[A-Z0-9_.-]*(?:password|passwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|credential)[A-Z0-9_.-]*["']?)(\s*[=:]\s*)(\[REDACTED\]|"[^"\r\n]*(?:"|\r?\n|\z)|'[^'\r\n]*(?:'|\r?\n|\z)|[^\s,;\}\]]+)`)
var headerPattern = regexp.MustCompile(`(?im)(\b(?:authorization|proxy-authorization|cookie|set-cookie)["']?\s*:\s*)[^\r\n]+`)

const redacted = "[REDACTED]"

// RedactSecrets replaces secret-like values and returns the count replaced.
func RedactSecrets(in []byte) ([]byte, int) {
	n := 0
	out := headerPattern.ReplaceAllFunc(in, func(m []byte) []byte {
		s := headerPattern.FindSubmatch(m)
		if string(m[len(s[1]):]) == redacted {
			return m
		}
		n++
		return append(append([]byte{}, s[1]...), redacted...)
	})
	out = assignPattern.ReplaceAllFunc(out, func(m []byte) []byte {
		s := assignPattern.FindSubmatch(m)
		if string(s[3]) == redacted {
			return m
		}
		n++
		return append(append(append([]byte{}, s[1]...), s[2]...), redacted...)
	})
	for _, re := range secretPatterns {
		out = re.ReplaceAllFunc(out, func(m []byte) []byte {
			n++
			if m[0] == ':' {
				return []byte("://" + redacted + "@")
			}
			return []byte(redacted)
		})
	}
	return out, n
}
