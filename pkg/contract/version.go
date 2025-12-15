package contract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version (major.minor.patch).
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Metadata   string
}

var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// ParseVersion parses a semantic version string.
func ParseVersion(s string) (Version, error) {
	matches := semverRegex.FindStringSubmatch(s)
	if matches == nil {
		return Version{}, fmt.Errorf("invalid semver: %s", s)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Metadata:   matches[5],
	}, nil
}

// String returns the string representation of the version.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Metadata != "" {
		s += "+" + v.Metadata
	}
	return s
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return compareInt(v.Patch, other.Patch)
	}

	// A version without prerelease has higher precedence
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		return comparePrereleases(v.Prerelease, other.Prerelease)
	}

	return 0
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func comparePrereleases(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aNum, aIsNum := tryParseInt(aParts[i])
		bNum, bIsNum := tryParseInt(bParts[i])

		if aIsNum && bIsNum {
			if cmp := compareInt(aNum, bNum); cmp != 0 {
				return cmp
			}
		} else if aIsNum {
			// Numeric identifiers have lower precedence than alphanumeric
			return -1
		} else if bIsNum {
			return 1
		} else {
			// Compare as strings
			if cmp := strings.Compare(aParts[i], bParts[i]); cmp != 0 {
				return cmp
			}
		}
	}

	// More parts means higher precedence
	return compareInt(len(aParts), len(bParts))
}

func tryParseInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// LessThan returns true if v < other.
func (v Version) LessThan(other Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other.
func (v Version) GreaterThan(other Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if v == other (ignoring metadata).
func (v Version) Equal(other Version) bool {
	return v.Compare(other) == 0
}

// BumpMajor returns a new version with major incremented and minor/patch reset.
func (v Version) BumpMajor() Version {
	return Version{Major: v.Major + 1, Minor: 0, Patch: 0}
}

// BumpMinor returns a new version with minor incremented and patch reset.
func (v Version) BumpMinor() Version {
	return Version{Major: v.Major, Minor: v.Minor + 1, Patch: 0}
}

// BumpPatch returns a new version with patch incremented.
func (v Version) BumpPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// VersionRange represents a semver constraint for version matching.
type VersionRange struct {
	constraints []constraint
}

type constraint struct {
	op      string
	version Version
}

// ParseVersionRange parses a version range string.
// Supports: =, >, <, >=, <=, ^, ~, and ranges like ">=1.0.0 <2.0.0"
func ParseVersionRange(s string) (*VersionRange, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return &VersionRange{}, nil // matches all
	}

	vr := &VersionRange{}
	parts := strings.Fields(s)

	for _, part := range parts {
		c, err := parseConstraint(part)
		if err != nil {
			return nil, err
		}
		vr.constraints = append(vr.constraints, c)
	}

	return vr, nil
}

func parseConstraint(s string) (constraint, error) {
	s = strings.TrimSpace(s)

	// Check for operators
	ops := []string{">=", "<=", ">", "<", "=", "^", "~"}
	for _, op := range ops {
		if after, ok :=strings.CutPrefix(s, op); ok  {
			verStr := after
			v, err := ParseVersion(verStr)
			if err != nil {
				return constraint{}, err
			}
			return constraint{op: op, version: v}, nil
		}
	}

	// No operator means exact match
	v, err := ParseVersion(s)
	if err != nil {
		return constraint{}, err
	}
	return constraint{op: "=", version: v}, nil
}

// Contains returns true if the version satisfies the range.
func (vr *VersionRange) Contains(v Version) bool {
	if len(vr.constraints) == 0 {
		return true // no constraints = matches all
	}

	for _, c := range vr.constraints {
		if !c.matches(v) {
			return false
		}
	}
	return true
}

func (c constraint) matches(v Version) bool {
	switch c.op {
	case "=":
		return v.Equal(c.version)
	case ">":
		return v.GreaterThan(c.version)
	case "<":
		return v.LessThan(c.version)
	case ">=":
		return v.GreaterThan(c.version) || v.Equal(c.version)
	case "<=":
		return v.LessThan(c.version) || v.Equal(c.version)
	case "^":
		// Caret allows changes that don't modify the leftmost non-zero digit
		if c.version.Major != 0 {
			return v.Major == c.version.Major && !v.LessThan(c.version)
		}
		if c.version.Minor != 0 {
			return v.Major == c.version.Major && v.Minor == c.version.Minor && !v.LessThan(c.version)
		}
		return v.Equal(c.version)
	case "~":
		// Tilde allows patch-level changes
		return v.Major == c.version.Major && v.Minor == c.version.Minor && !v.LessThan(c.version)
	default:
		return false
	}
}

// String returns the string representation of the range.
func (vr *VersionRange) String() string {
	if len(vr.constraints) == 0 {
		return "*"
	}

	parts := make([]string, len(vr.constraints))
	for i, c := range vr.constraints {
		if c.op == "=" {
			parts[i] = c.version.String()
		} else {
			parts[i] = c.op + c.version.String()
		}
	}
	return strings.Join(parts, " ")
}
