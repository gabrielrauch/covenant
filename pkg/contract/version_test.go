package contract

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantPre   string
		wantMeta  string
		wantErr   bool
	}{
		{
			name:      "simple version",
			input:     "1.2.3",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "zero version",
			input:     "0.0.0",
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
		{
			name:      "large numbers",
			input:     "100.200.300",
			wantMajor: 100,
			wantMinor: 200,
			wantPatch: 300,
		},
		{
			name:      "v prefix",
			input:     "v1.2.3",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "v prefix zero",
			input:     "v0.1.0",
			wantMajor: 0,
			wantMinor: 1,
			wantPatch: 0,
		},
		{
			name:      "prerelease alpha",
			input:     "1.0.0-alpha",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "alpha",
		},
		{
			name:      "prerelease alpha.1",
			input:     "1.0.0-alpha.1",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "alpha.1",
		},
		{
			name:      "prerelease beta.2",
			input:     "2.1.0-beta.2",
			wantMajor: 2,
			wantMinor: 1,
			wantPatch: 0,
			wantPre:   "beta.2",
		},
		{
			name:      "prerelease rc.1",
			input:     "1.0.0-rc.1",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "rc.1",
		},
		{
			name:      "prerelease with hyphen",
			input:     "1.0.0-x-y-z",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "x-y-z",
		},
		{
			name:      "build metadata",
			input:     "1.0.0+build.123",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
			wantMeta:  "build.123",
		},
		{
			name:      "prerelease and metadata",
			input:     "1.0.0-alpha+001",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
			wantPre:   "alpha",
			wantMeta:  "001",
		},
		{
			name:      "v prefix with prerelease and metadata",
			input:     "v2.3.4-beta.1+sha.45678",
			wantMajor: 2,
			wantMinor: 3,
			wantPatch: 4,
			wantPre:   "beta.1",
			wantMeta:  "sha.45678",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "garbage",
			input:   "not-a-version",
			wantErr: true,
		},
		{
			name:    "missing patch",
			input:   "1.2",
			wantErr: true,
		},
		{
			name:    "missing minor and patch",
			input:   "1",
			wantErr: true,
		},
		{
			name:    "trailing dot",
			input:   "1.2.3.",
			wantErr: true,
		},
		{
			name:    "leading space",
			input:   " 1.2.3",
			wantErr: true,
		},
		{
			name:    "trailing space",
			input:   "1.2.3 ",
			wantErr: true,
		},
		{
			name:    "negative major",
			input:   "-1.2.3",
			wantErr: true,
		},
		{
			name:    "letters in version",
			input:   "a.b.c",
			wantErr: true,
		},
		{
			name:    "four components",
			input:   "1.2.3.4",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if v.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", v.Major, tt.wantMajor)
			}
			if v.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", v.Minor, tt.wantMinor)
			}
			if v.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", v.Patch, tt.wantPatch)
			}
			if v.Prerelease != tt.wantPre {
				t.Errorf("Prerelease = %q, want %q", v.Prerelease, tt.wantPre)
			}
			if v.Metadata != tt.wantMeta {
				t.Errorf("Metadata = %q, want %q", v.Metadata, tt.wantMeta)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		want    string
	}{
		{
			name:    "simple",
			version: Version{Major: 1, Minor: 2, Patch: 3},
			want:    "1.2.3",
		},
		{
			name:    "zero",
			version: Version{Major: 0, Minor: 0, Patch: 0},
			want:    "0.0.0",
		},
		{
			name:    "with prerelease",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want:    "1.0.0-alpha",
		},
		{
			name:    "with metadata",
			version: Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.1"},
			want:    "1.0.0+build.1",
		},
		{
			name:    "with prerelease and metadata",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta.2", Metadata: "sha.abc"},
			want:    "1.0.0-beta.2+sha.abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.version.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name string
		a    Version
		b    Version
		want int
	}{
		{
			name: "equal versions",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 3},
			want: 0,
		},
		{
			name: "major greater",
			a:    Version{Major: 2, Minor: 0, Patch: 0},
			b:    Version{Major: 1, Minor: 0, Patch: 0},
			want: 1,
		},
		{
			name: "major less",
			a:    Version{Major: 1, Minor: 0, Patch: 0},
			b:    Version{Major: 2, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "minor greater",
			a:    Version{Major: 1, Minor: 3, Patch: 0},
			b:    Version{Major: 1, Minor: 2, Patch: 0},
			want: 1,
		},
		{
			name: "minor less",
			a:    Version{Major: 1, Minor: 2, Patch: 0},
			b:    Version{Major: 1, Minor: 3, Patch: 0},
			want: -1,
		},
		{
			name: "patch greater",
			a:    Version{Major: 1, Minor: 2, Patch: 4},
			b:    Version{Major: 1, Minor: 2, Patch: 3},
			want: 1,
		},
		{
			name: "patch less",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 4},
			want: -1,
		},
		{
			name: "major takes precedence over minor",
			a:    Version{Major: 2, Minor: 0, Patch: 0},
			b:    Version{Major: 1, Minor: 99, Patch: 99},
			want: 1,
		},
		{
			name: "minor takes precedence over patch",
			a:    Version{Major: 1, Minor: 2, Patch: 0},
			b:    Version{Major: 1, Minor: 1, Patch: 99},
			want: 1,
		},
		{
			name: "no prerelease > with prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: 1,
		},
		{
			name: "with prerelease < no prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "alpha < beta prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta"},
			want: -1,
		},
		{
			name: "beta > alpha prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: 1,
		},
		{
			name: "numeric prerelease comparison",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "1"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "2"},
			want: -1,
		},
		{
			name: "numeric prerelease alpha.1 < alpha.2",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.2"},
			want: -1,
		},
		{
			name: "numeric identifier < string identifier",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "1"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: -1,
		},
		{
			name: "string identifier > numeric identifier",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "1"},
			want: 1,
		},
		{
			name: "more prerelease parts has higher precedence",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: 1,
		},
		{
			name: "fewer prerelease parts has lower precedence",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			want: -1,
		},
		{
			name: "same prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			want: 0,
		},
		{
			name: "metadata ignored in comparison",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.1"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.2"},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Compare(tt.b)
			if got != tt.want {
				t.Errorf("(%s).Compare(%s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionLessThan(t *testing.T) {
	tests := []struct {
		name string
		a    Version
		b    Version
		want bool
	}{
		{
			name: "less by major",
			a:    Version{Major: 1, Minor: 0, Patch: 0},
			b:    Version{Major: 2, Minor: 0, Patch: 0},
			want: true,
		},
		{
			name: "less by minor",
			a:    Version{Major: 1, Minor: 1, Patch: 0},
			b:    Version{Major: 1, Minor: 2, Patch: 0},
			want: true,
		},
		{
			name: "less by patch",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 4},
			want: true,
		},
		{
			name: "equal is not less",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 3},
			want: false,
		},
		{
			name: "greater is not less",
			a:    Version{Major: 2, Minor: 0, Patch: 0},
			b:    Version{Major: 1, Minor: 0, Patch: 0},
			want: false,
		},
		{
			name: "prerelease less than release",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.LessThan(tt.b)
			if got != tt.want {
				t.Errorf("(%s).LessThan(%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionGreaterThan(t *testing.T) {
	tests := []struct {
		name string
		a    Version
		b    Version
		want bool
	}{
		{
			name: "greater by major",
			a:    Version{Major: 2, Minor: 0, Patch: 0},
			b:    Version{Major: 1, Minor: 0, Patch: 0},
			want: true,
		},
		{
			name: "greater by minor",
			a:    Version{Major: 1, Minor: 3, Patch: 0},
			b:    Version{Major: 1, Minor: 2, Patch: 0},
			want: true,
		},
		{
			name: "greater by patch",
			a:    Version{Major: 1, Minor: 2, Patch: 5},
			b:    Version{Major: 1, Minor: 2, Patch: 4},
			want: true,
		},
		{
			name: "equal is not greater",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 3},
			want: false,
		},
		{
			name: "less is not greater",
			a:    Version{Major: 1, Minor: 0, Patch: 0},
			b:    Version{Major: 2, Minor: 0, Patch: 0},
			want: false,
		},
		{
			name: "release greater than prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.GreaterThan(tt.b)
			if got != tt.want {
				t.Errorf("(%s).GreaterThan(%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionEqual(t *testing.T) {
	tests := []struct {
		name string
		a    Version
		b    Version
		want bool
	}{
		{
			name: "same versions",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 3},
			want: true,
		},
		{
			name: "different major",
			a:    Version{Major: 1, Minor: 0, Patch: 0},
			b:    Version{Major: 2, Minor: 0, Patch: 0},
			want: false,
		},
		{
			name: "different minor",
			a:    Version{Major: 1, Minor: 1, Patch: 0},
			b:    Version{Major: 1, Minor: 2, Patch: 0},
			want: false,
		},
		{
			name: "different patch",
			a:    Version{Major: 1, Minor: 2, Patch: 3},
			b:    Version{Major: 1, Minor: 2, Patch: 4},
			want: false,
		},
		{
			name: "metadata ignored",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.100"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.200"},
			want: true,
		},
		{
			name: "same prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: true,
		},
		{
			name: "different prerelease",
			a:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			b:    Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Equal(tt.b)
			if got != tt.want {
				t.Errorf("(%s).Equal(%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionBumpMajor(t *testing.T) {
	tests := []struct {
		name  string
		input Version
		want  Version
	}{
		{
			name:  "simple bump",
			input: Version{Major: 1, Minor: 2, Patch: 3},
			want:  Version{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:  "from zero",
			input: Version{Major: 0, Minor: 0, Patch: 0},
			want:  Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:  "resets minor and patch",
			input: Version{Major: 1, Minor: 5, Patch: 9},
			want:  Version{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:  "strips prerelease",
			input: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want:  Version{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:  "strips metadata",
			input: Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.1"},
			want:  Version{Major: 2, Minor: 0, Patch: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.BumpMajor()
			if got != tt.want {
				t.Errorf("(%s).BumpMajor() = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersionBumpMinor(t *testing.T) {
	tests := []struct {
		name  string
		input Version
		want  Version
	}{
		{
			name:  "simple bump",
			input: Version{Major: 1, Minor: 2, Patch: 3},
			want:  Version{Major: 1, Minor: 3, Patch: 0},
		},
		{
			name:  "from zero",
			input: Version{Major: 0, Minor: 0, Patch: 0},
			want:  Version{Major: 0, Minor: 1, Patch: 0},
		},
		{
			name:  "resets patch",
			input: Version{Major: 1, Minor: 2, Patch: 9},
			want:  Version{Major: 1, Minor: 3, Patch: 0},
		},
		{
			name:  "preserves major",
			input: Version{Major: 5, Minor: 3, Patch: 7},
			want:  Version{Major: 5, Minor: 4, Patch: 0},
		},
		{
			name:  "strips prerelease",
			input: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta"},
			want:  Version{Major: 1, Minor: 1, Patch: 0},
		},
		{
			name:  "strips metadata",
			input: Version{Major: 1, Minor: 0, Patch: 0, Metadata: "sha.abc"},
			want:  Version{Major: 1, Minor: 1, Patch: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.BumpMinor()
			if got != tt.want {
				t.Errorf("(%s).BumpMinor() = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersionBumpPatch(t *testing.T) {
	tests := []struct {
		name  string
		input Version
		want  Version
	}{
		{
			name:  "simple bump",
			input: Version{Major: 1, Minor: 2, Patch: 3},
			want:  Version{Major: 1, Minor: 2, Patch: 4},
		},
		{
			name:  "from zero",
			input: Version{Major: 0, Minor: 0, Patch: 0},
			want:  Version{Major: 0, Minor: 0, Patch: 1},
		},
		{
			name:  "preserves major and minor",
			input: Version{Major: 5, Minor: 3, Patch: 7},
			want:  Version{Major: 5, Minor: 3, Patch: 8},
		},
		{
			name:  "strips prerelease",
			input: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "rc.1"},
			want:  Version{Major: 1, Minor: 0, Patch: 1},
		},
		{
			name:  "strips metadata",
			input: Version{Major: 1, Minor: 0, Patch: 0, Metadata: "build.99"},
			want:  Version{Major: 1, Minor: 0, Patch: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.BumpPatch()
			if got != tt.want {
				t.Errorf("(%s).BumpPatch() = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVersionRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty string matches all", input: "", wantErr: false},
		{name: "wildcard matches all", input: "*", wantErr: false},
		{name: "exact version", input: "1.2.3", wantErr: false},
		{name: "equal operator", input: "=1.2.3", wantErr: false},
		{name: "greater than", input: ">1.0.0", wantErr: false},
		{name: "less than", input: "<2.0.0", wantErr: false},
		{name: "greater or equal", input: ">=1.0.0", wantErr: false},
		{name: "less or equal", input: "<=2.0.0", wantErr: false},
		{name: "caret", input: "^1.2.3", wantErr: false},
		{name: "tilde", input: "~1.2.3", wantErr: false},
		{name: "compound range", input: ">=1.0.0 <2.0.0", wantErr: false},
		{name: "compound three constraints", input: ">=1.0.0 <2.0.0 >=1.5.0", wantErr: false},
		{name: "invalid version", input: ">not-a-version", wantErr: true},
		{name: "invalid constraint", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersionRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersionRange(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestVersionRangeContains(t *testing.T) {
	tests := []struct {
		name     string
		rangeStr string
		version  Version
		want     bool
	}{
		// Wildcard / empty range
		{
			name:     "empty matches any",
			rangeStr: "",
			version:  Version{Major: 99, Minor: 99, Patch: 99},
			want:     true,
		},
		{
			name:     "wildcard matches any",
			rangeStr: "*",
			version:  Version{Major: 0, Minor: 0, Patch: 0},
			want:     true,
		},
		// Exact match (= operator)
		{
			name:     "exact match equal",
			rangeStr: "=1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 3},
			want:     true,
		},
		{
			name:     "exact match not equal",
			rangeStr: "=1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 4},
			want:     false,
		},
		{
			name:     "bare version is exact match",
			rangeStr: "1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 3},
			want:     true,
		},
		{
			name:     "bare version not matching",
			rangeStr: "1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 4},
			want:     false,
		},
		// Greater than (>)
		{
			name:     "greater than satisfied",
			rangeStr: ">1.0.0",
			version:  Version{Major: 1, Minor: 0, Patch: 1},
			want:     true,
		},
		{
			name:     "greater than not satisfied equal",
			rangeStr: ">1.0.0",
			version:  Version{Major: 1, Minor: 0, Patch: 0},
			want:     false,
		},
		{
			name:     "greater than not satisfied less",
			rangeStr: ">1.0.0",
			version:  Version{Major: 0, Minor: 9, Patch: 9},
			want:     false,
		},
		// Less than (<)
		{
			name:     "less than satisfied",
			rangeStr: "<2.0.0",
			version:  Version{Major: 1, Minor: 9, Patch: 9},
			want:     true,
		},
		{
			name:     "less than not satisfied equal",
			rangeStr: "<2.0.0",
			version:  Version{Major: 2, Minor: 0, Patch: 0},
			want:     false,
		},
		{
			name:     "less than not satisfied greater",
			rangeStr: "<2.0.0",
			version:  Version{Major: 2, Minor: 0, Patch: 1},
			want:     false,
		},
		// Greater or equal (>=)
		{
			name:     "gte satisfied equal",
			rangeStr: ">=1.0.0",
			version:  Version{Major: 1, Minor: 0, Patch: 0},
			want:     true,
		},
		{
			name:     "gte satisfied greater",
			rangeStr: ">=1.0.0",
			version:  Version{Major: 2, Minor: 0, Patch: 0},
			want:     true,
		},
		{
			name:     "gte not satisfied",
			rangeStr: ">=1.0.0",
			version:  Version{Major: 0, Minor: 9, Patch: 9},
			want:     false,
		},
		// Less or equal (<=)
		{
			name:     "lte satisfied equal",
			rangeStr: "<=2.0.0",
			version:  Version{Major: 2, Minor: 0, Patch: 0},
			want:     true,
		},
		{
			name:     "lte satisfied less",
			rangeStr: "<=2.0.0",
			version:  Version{Major: 1, Minor: 0, Patch: 0},
			want:     true,
		},
		{
			name:     "lte not satisfied",
			rangeStr: "<=2.0.0",
			version:  Version{Major: 2, Minor: 0, Patch: 1},
			want:     false,
		},
		// Caret (^) - allows changes that don't modify leftmost non-zero digit
		{
			name:     "caret same major exact",
			rangeStr: "^1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 3},
			want:     true,
		},
		{
			name:     "caret same major higher minor",
			rangeStr: "^1.2.3",
			version:  Version{Major: 1, Minor: 3, Patch: 0},
			want:     true,
		},
		{
			name:     "caret same major higher patch",
			rangeStr: "^1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 9},
			want:     true,
		},
		{
			name:     "caret different major",
			rangeStr: "^1.2.3",
			version:  Version{Major: 2, Minor: 0, Patch: 0},
			want:     false,
		},
		{
			name:     "caret lower version",
			rangeStr: "^1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 2},
			want:     false,
		},
		{
			name:     "caret 0.x locks minor",
			rangeStr: "^0.2.3",
			version:  Version{Major: 0, Minor: 2, Patch: 5},
			want:     true,
		},
		{
			name:     "caret 0.x different minor",
			rangeStr: "^0.2.3",
			version:  Version{Major: 0, Minor: 3, Patch: 0},
			want:     false,
		},
		{
			name:     "caret 0.0.x exact match only",
			rangeStr: "^0.0.3",
			version:  Version{Major: 0, Minor: 0, Patch: 3},
			want:     true,
		},
		{
			name:     "caret 0.0.x different patch",
			rangeStr: "^0.0.3",
			version:  Version{Major: 0, Minor: 0, Patch: 4},
			want:     false,
		},
		// Tilde (~) - allows patch-level changes
		{
			name:     "tilde exact",
			rangeStr: "~1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 3},
			want:     true,
		},
		{
			name:     "tilde higher patch",
			rangeStr: "~1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 9},
			want:     true,
		},
		{
			name:     "tilde different minor",
			rangeStr: "~1.2.3",
			version:  Version{Major: 1, Minor: 3, Patch: 0},
			want:     false,
		},
		{
			name:     "tilde different major",
			rangeStr: "~1.2.3",
			version:  Version{Major: 2, Minor: 2, Patch: 3},
			want:     false,
		},
		{
			name:     "tilde lower patch",
			rangeStr: "~1.2.3",
			version:  Version{Major: 1, Minor: 2, Patch: 2},
			want:     false,
		},
		// Compound ranges
		{
			name:     "compound range satisfied",
			rangeStr: ">=1.0.0 <2.0.0",
			version:  Version{Major: 1, Minor: 5, Patch: 0},
			want:     true,
		},
		{
			name:     "compound range lower bound",
			rangeStr: ">=1.0.0 <2.0.0",
			version:  Version{Major: 1, Minor: 0, Patch: 0},
			want:     true,
		},
		{
			name:     "compound range upper bound excluded",
			rangeStr: ">=1.0.0 <2.0.0",
			version:  Version{Major: 2, Minor: 0, Patch: 0},
			want:     false,
		},
		{
			name:     "compound range below",
			rangeStr: ">=1.0.0 <2.0.0",
			version:  Version{Major: 0, Minor: 9, Patch: 9},
			want:     false,
		},
		{
			name:     "compound range above",
			rangeStr: ">=1.0.0 <2.0.0",
			version:  Version{Major: 3, Minor: 0, Patch: 0},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := ParseVersionRange(tt.rangeStr)
			if err != nil {
				t.Fatalf("ParseVersionRange(%q) unexpected error: %v", tt.rangeStr, err)
			}
			got := vr.Contains(tt.version)
			if got != tt.want {
				t.Errorf("range %q Contains(%s) = %v, want %v", tt.rangeStr, tt.version, got, tt.want)
			}
		})
	}
}

func TestVersionRangeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "wildcard empty",
			input: "",
			want:  "*",
		},
		{
			name:  "wildcard star",
			input: "*",
			want:  "*",
		},
		{
			name:  "bare version becomes exact",
			input: "1.2.3",
			want:  "1.2.3",
		},
		{
			name:  "equal operator omitted",
			input: "=1.2.3",
			want:  "1.2.3",
		},
		{
			name:  "greater than preserved",
			input: ">1.0.0",
			want:  ">1.0.0",
		},
		{
			name:  "less than preserved",
			input: "<2.0.0",
			want:  "<2.0.0",
		},
		{
			name:  "gte preserved",
			input: ">=1.0.0",
			want:  ">=1.0.0",
		},
		{
			name:  "lte preserved",
			input: "<=2.0.0",
			want:  "<=2.0.0",
		},
		{
			name:  "caret preserved",
			input: "^1.2.3",
			want:  "^1.2.3",
		},
		{
			name:  "tilde preserved",
			input: "~1.2.3",
			want:  "~1.2.3",
		},
		{
			name:  "compound range",
			input: ">=1.0.0 <2.0.0",
			want:  ">=1.0.0 <2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := ParseVersionRange(tt.input)
			if err != nil {
				t.Fatalf("ParseVersionRange(%q) unexpected error: %v", tt.input, err)
			}
			got := vr.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseVersionRoundTrip(t *testing.T) {
	inputs := []string{
		"1.2.3",
		"0.0.0",
		"10.20.30",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0+build.123",
		"1.0.0-beta.2+sha.abcdef",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			v, err := ParseVersion(input)
			if err != nil {
				t.Fatalf("ParseVersion(%q) unexpected error: %v", input, err)
			}
			got := v.String()
			if got != input {
				t.Errorf("round-trip: ParseVersion(%q).String() = %q", input, got)
			}
		})
	}
}

func TestVersionBumpDoesNotMutate(t *testing.T) {
	original := Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha", Metadata: "build.1"}

	_ = original.BumpMajor()
	_ = original.BumpMinor()
	_ = original.BumpPatch()

	if original.Major != 1 || original.Minor != 2 || original.Patch != 3 {
		t.Error("bump methods should not mutate the original version")
	}
	if original.Prerelease != "alpha" {
		t.Error("bump methods should not mutate the original prerelease")
	}
	if original.Metadata != "build.1" {
		t.Error("bump methods should not mutate the original metadata")
	}
}
