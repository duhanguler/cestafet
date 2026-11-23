package setup

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// configDirRe finds a CLAUDE_CONFIG_DIR assignment in any shell's syntax:
// sh/zsh/bash/fish (`CLAUDE_CONFIG_DIR=...`, `set -x CLAUDE_CONFIG_DIR ...`),
// PowerShell (`$env:CLAUDE_CONFIG_DIR = "..."`) and cmd (`set "CLAUDE_CONFIG_DIR=..."`,
// `setx CLAUDE_CONFIG_DIR "..."`). The value is the quoted string, or the text up
// to whitespace or ';'.
var configDirRe = regexp.MustCompile(`(?i)CLAUDE_CONFIG_DIR(?:\s*=\s*|\s+)(?:"([^"]*)"|'([^']*)'|([^"'\s;]+))`)

// extract returns the CLAUDE_CONFIG_DIR values assigned on one line, ignoring
// comment lines (#, REM, ::).
func extract(line string) []string {
	t := strings.TrimSpace(line)
	lower := strings.ToLower(t)
	if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "::") || lower == "rem" || strings.HasPrefix(lower, "rem ") {
		return nil
	}
	var out []string
	for _, m := range configDirRe.FindAllStringSubmatch(t, -1) {
		v := m[1] + m[2] + m[3]
		v = strings.TrimSuffix(v, `"`) // set "CLAUDE_CONFIG_DIR=...": the quote closes the whole assignment
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

var (
	cmdVarRe = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)
	psVarRe  = regexp.MustCompile(`(?i)\$env:([A-Za-z_][A-Za-z0-9_]*)`)
)

// expand resolves ~, $HOME, ${HOME}, %VAR% and $env:VAR in a profile path.
// It returns "" for a relative path or a variable it cannot resolve.
func expand(dir, home string) string {
	switch {
	case strings.HasPrefix(dir, "$HOME"):
		dir = home + strings.TrimPrefix(dir, "$HOME")
	case strings.HasPrefix(dir, "${HOME}"):
		dir = home + strings.TrimPrefix(dir, "${HOME}")
	case dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`):
		dir = home + dir[1:] // only a leading ~: 8.3 names such as OSMANH~1 keep theirs
	}
	unresolved := false
	resolve := func(re *regexp.Regexp) {
		dir = re.ReplaceAllStringFunc(dir, func(m string) string {
			name := re.FindStringSubmatch(m)[1]
			if strings.EqualFold(name, "HOME") || strings.EqualFold(name, "USERPROFILE") {
				return home
			}
			if v, ok := os.LookupEnv(name); ok && v != "" {
				return v
			}
			unresolved = true
			return m
		})
	}
	resolve(cmdVarRe)
	resolve(psVarRe)
	if unresolved || strings.Contains(dir, "$") || !filepath.IsAbs(dir) {
		return ""
	}
	return filepath.Clean(dir)
}

// scanCandidate reports whether a file on PATH may be a wrapper script worth
// reading: an extensionless script or a shell/cmd/PowerShell script. Windows
// PATH directories hold thousands of executables and DLLs; reading them all is
// slow and wakes the virus scanner.
func scanCandidate(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case "", ".sh", ".bash", ".zsh", ".fish", ".cmd", ".bat", ".ps1":
		return true
	}
	return false
}

// shellProfiles lists the shell startup files under home that may set
// CLAUDE_CONFIG_DIR, Unix and Windows (PowerShell 7 and Windows PowerShell, in
// Documents or a OneDrive-redirected Documents).
func shellProfiles(home string) []string {
	files := []string{".zshrc", ".zprofile", ".zshenv", ".bashrc", ".bash_profile", ".profile",
		filepath.Join(".config", "fish", "config.fish")}
	docs := []string{"Documents"}
	if matches, err := filepath.Glob(filepath.Join(home, "OneDrive*", "Documents")); err == nil {
		for _, m := range matches {
			if rel, err := filepath.Rel(home, m); err == nil {
				docs = append(docs, rel)
			}
		}
	}
	for _, d := range docs {
		for _, shell := range []string{"PowerShell", "WindowsPowerShell"} {
			for _, f := range []string{"Microsoft.PowerShell_profile.ps1", "profile.ps1"} {
				files = append(files, filepath.Join(d, shell, f))
			}
		}
	}
	return files
}
