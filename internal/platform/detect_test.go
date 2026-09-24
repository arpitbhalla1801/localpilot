package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindGitRoot(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	nested := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := findGitRoot(nested); got != repo {
		t.Errorf("findGitRoot(nested) = %q, want %q", got, repo)
	}
	if got := findGitRoot(repo); got != repo {
		t.Errorf("findGitRoot(repo) = %q, want %q", got, repo)
	}

	outside := filepath.Join(tmp, "no-repo")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findGitRoot(outside); got != "" {
		t.Errorf("findGitRoot(outside) = %q, want empty", got)
	}
}

func TestDetectFramework(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"package.json", "Node.js"},
		{"go.mod", "Go"},
		{"Cargo.toml", "Rust"},
		{"pyproject.toml", "Python"},
		{"pom.xml", "Java/Maven"},
	}

	for _, tt := range tests {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, tt.file), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := detectFramework(tmp); got != tt.want {
			t.Errorf("detectFramework with %s = %q, want %q", tt.file, got, tt.want)
		}
	}

	empty := t.TempDir()
	if got := detectFramework(empty); got != "" {
		t.Errorf("detectFramework(empty) = %q, want empty", got)
	}
}

func TestDetectProject_NoGit(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "myproject")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	proj := DetectProject(sub)
	if proj == nil {
		t.Fatal("expected non-nil project")
	}
	if proj.Name != "myproject" {
		t.Errorf("Name = %q, want myproject", proj.Name)
	}
	if proj.Path != sub {
		t.Errorf("Path = %q, want %q", proj.Path, sub)
	}
	if proj.Branch != "" || proj.Framework != "" {
		t.Errorf("expected empty repo/branch/framework outside git, got %+v", proj)
	}
}

func TestDetectProject_EmptyCwd(t *testing.T) {
	if got := DetectProject(""); got != nil {
		t.Errorf("DetectProject(\"\") = %+v, want nil", got)
	}
}

func TestDetectProject_WithGitAndFramework(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	repo := filepath.Join(tmp, "webapp")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "--allow-empty", "-q", "-m", "init")

	proj := DetectProject(repo)
	if proj == nil {
		t.Fatal("expected non-nil project")
	}
	if proj.Name != "webapp" {
		t.Errorf("Name = %q, want webapp", proj.Name)
	}
	if proj.Branch != "main" {
		t.Errorf("Branch = %q, want main", proj.Branch)
	}
	if proj.Framework != "Node.js" {
		t.Errorf("Framework = %q, want Node.js", proj.Framework)
	}
}

func TestGitBranch_NotARepo(t *testing.T) {
	tmp := t.TempDir()
	if got := gitBranch(tmp); got != "" {
		t.Errorf("gitBranch(non-repo) = %q, want empty", got)
	}
}

func TestSocketTypeName(t *testing.T) {
	tests := []struct {
		in   uint32
		want string
	}{
		{1, "tcp"},
		{2, "udp"},
		{99, "socket-99"},
	}
	for _, tt := range tests {
		if got := socketTypeName(tt.in); got != tt.want {
			t.Errorf("socketTypeName(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseEnviron(t *testing.T) {
	env := []string{"HOME=/home/user", "EMPTY=", "NOEQUALS", "A=B=C"}
	got := parseEnviron(env)

	if got["HOME"] != "/home/user" {
		t.Errorf("HOME = %q", got["HOME"])
	}
	if got["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, want empty string", got["EMPTY"])
	}
	if _, ok := got["NOEQUALS"]; ok {
		t.Errorf("NOEQUALS should be dropped (no '=')")
	}
	if got["A"] != "B=C" {
		t.Errorf("A = %q, want B=C (split on first '=' only)", got["A"])
	}
}
