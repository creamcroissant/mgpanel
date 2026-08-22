package capability

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
)

// CommandRunner defines an interface for running commands.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// DefaultCommandRunner uses exec.CommandContext.
type DefaultCommandRunner struct{}

func (r *DefaultCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// Detector detects the core capabilities of sing-box/xray.
type Detector struct {
	singBoxPath string
	xrayPath    string
	runner      CommandRunner
}

// DetectedCapabilities holds the detected core capabilities.
type DetectedCapabilities struct {
	CoreType     string   // Primary core type (sing-box, xray)
	CoreVersion  string   // e.g., "1.10.0"
	Capabilities []string // e.g., ["reality", "multiplex", "brutal"]
	BuildTags    []string // e.g., ["with_v2ray_api", "with_quic"]

	// AllCores holds ALL detected core types, not just the primary.
	AllCores []DetectedCoreInfo
}

// DetectedCoreInfo holds info about one detected core binary.
type DetectedCoreInfo struct {
	Type         string
	Version      string
	Capabilities []string
	BuildTags    []string
}

// NewDetector creates a new capability detector.
func NewDetector(singBoxPath, xrayPath string) *Detector {
	if singBoxPath == "" {
		singBoxPath = "sing-box"
	}
	if xrayPath == "" {
		xrayPath = "xray"
	}
	return &Detector{
		singBoxPath: singBoxPath,
		xrayPath:    xrayPath,
		runner:      &DefaultCommandRunner{},
	}
}

// SetRunner sets a custom command runner for testing.
func (d *Detector) SetRunner(runner CommandRunner) {
	d.runner = runner
}

// DetectSingBox checks if Sing-box is installed and returns its capabilities.
func (d *Detector) DetectSingBox(ctx context.Context) (*DetectedCapabilities, error) {
	path := d.singBoxPath
	if path == "" {
		path = "sing-box"
	}

	// Run sing-box version
	output, err := d.runner.Run(ctx, path, "version")
	if err != nil {
		return nil, err
	}

	outputStr := string(output)

	caps := &DetectedCapabilities{
		CoreType:     "sing-box",
		Capabilities: []string{},
		BuildTags:    []string{},
	}

	// Parse version (e.g., "sing-box version 1.10.0" or "version: 1.10.0")
	versionRegex := regexp.MustCompile(`(?:sing-box\s+)?version[:\s]+(\d+\.\d+\.\d+)`)
	if matches := versionRegex.FindStringSubmatch(outputStr); len(matches) > 1 {
		caps.CoreVersion = matches[1]
	}

	// Parse build tags from "Tags:" line (e.g., "Tags: with_quic,with_v2ray_api")
	tagsRegex := regexp.MustCompile(`(?i)tags?[:\s]+(.+)`)
	if matches := tagsRegex.FindStringSubmatch(outputStr); len(matches) > 1 {
		tags := strings.Split(matches[1], ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				caps.BuildTags = append(caps.BuildTags, tag)
			}
		}
	}

	// Derive capabilities from version and build tags
	caps.Capabilities = d.deriveSingBoxCapabilities(caps.CoreVersion, caps.BuildTags)

	return caps, nil
}

// DetectXray checks if Xray is installed and returns its capabilities.
func (d *Detector) DetectXray(ctx context.Context) (*DetectedCapabilities, error) {
	path := d.xrayPath
	if path == "" {
		path = "xray"
	}

	// Run xray version
	output, err := d.runner.Run(ctx, path, "version")
	if err != nil {
		return nil, err
	}

	outputStr := string(output)

	caps := &DetectedCapabilities{
		CoreType:     "xray",
		Capabilities: []string{},
		BuildTags:    []string{},
	}

	// Parse version (e.g., "Xray 1.8.4 (Xray, Penetrates Everything.)")
	versionRegex := regexp.MustCompile(`Xray\s+(\d+\.\d+\.\d+)`)
	if matches := versionRegex.FindStringSubmatch(outputStr); len(matches) > 1 {
		caps.CoreVersion = matches[1]
	}

	// Derive capabilities from version
	caps.Capabilities = d.deriveXrayCapabilities(caps.CoreVersion)

	return caps, nil
}

// Detect attempts to detect which cores are installed.
// It tries sing-box first (preferred primary), then xray.
// Every detected core is recorded in AllCores.
func (d *Detector) Detect(ctx context.Context) *DetectedCapabilities {
	var allCores []DetectedCoreInfo

	// Try sing-box first (preferred primary)
	if caps, err := d.DetectSingBox(ctx); err == nil {
		allCores = append(allCores, DetectedCoreInfo{
			Type: caps.CoreType, Version: caps.CoreVersion,
			Capabilities: caps.Capabilities, BuildTags: caps.BuildTags,
		})
	}

	// Try xray
	if caps, err := d.DetectXray(ctx); err == nil {
		allCores = append(allCores, DetectedCoreInfo{
			Type: caps.CoreType, Version: caps.CoreVersion,
			Capabilities: caps.Capabilities, BuildTags: caps.BuildTags,
		})
	}

	if len(allCores) == 0 {
		return &DetectedCapabilities{
			CoreType: "unknown", Capabilities: []string{}, BuildTags: []string{},
			AllCores: []DetectedCoreInfo{},
		}
	}

	// Return first core as primary, all as AllCores
	primary := allCores[0]
	return &DetectedCapabilities{
		CoreType:     primary.Type,
		CoreVersion:  primary.Version,
		Capabilities: primary.Capabilities,
		BuildTags:    primary.BuildTags,
		AllCores:     allCores,
	}
}

// deriveSingBoxCapabilities derives capabilities from sing-box version and build tags.
func (d *Detector) deriveSingBoxCapabilities(version string, buildTags []string) []string {
	caps := []string{}

	// Version-based capabilities
	if d.compareVersions(version, "1.3.0") >= 0 {
		caps = append(caps, "reality", "multiplex")
	}
	if d.compareVersions(version, "1.7.0") >= 0 {
		caps = append(caps, "brutal")
	}
	if d.compareVersions(version, "1.8.0") >= 0 {
		caps = append(caps, "ech")
	}

	// Build tag based capabilities
	for _, tag := range buildTags {
		tag = strings.ToLower(tag)
		switch {
		case strings.Contains(tag, "v2ray_api"):
			caps = append(caps, "v2ray_api")
		case strings.Contains(tag, "quic"):
			caps = append(caps, "quic")
		case strings.Contains(tag, "grpc"):
			caps = append(caps, "grpc")
		case strings.Contains(tag, "utls"):
			caps = append(caps, "utls")
		case strings.Contains(tag, "gvisor"):
			caps = append(caps, "gvisor")
		case strings.Contains(tag, "wireguard"):
			caps = append(caps, "wireguard")
		case strings.Contains(tag, "ech"):
			caps = append(caps, "ech")
		}
	}

	return d.uniqueStrings(caps)
}

// deriveXrayCapabilities derives capabilities from xray version.
func (d *Detector) deriveXrayCapabilities(version string) []string {
	caps := []string{}

	// Xray has Reality support from early versions
	if d.compareVersions(version, "1.0.0") >= 0 {
		caps = append(caps, "reality", "xtls")
	}
	if d.compareVersions(version, "1.8.0") >= 0 {
		caps = append(caps, "splithttp")
	}

	return caps
}

// compareVersions compares two semver versions.
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func (d *Detector) compareVersions(v1, v2 string) int {
	parts1 := d.parseVersion(v1)
	parts2 := d.parseVersion(v2)

	for i := 0; i < 3; i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}
	return 0
}

// parseVersion parses a version string into [major, minor, patch].
func (d *Detector) parseVersion(v string) [3]int {
	var parts [3]int
	v = strings.TrimPrefix(v, "v")
	segments := strings.Split(v, ".")
	for i := 0; i < len(segments) && i < 3; i++ {
		var num int
		for _, c := range segments[i] {
			if c >= '0' && c <= '9' {
				num = num*10 + int(c-'0')
			} else {
				break
			}
		}
		parts[i] = num
	}
	return parts
}

// uniqueStrings removes duplicate strings from a slice.
func (d *Detector) SingBoxPath() string {
	return d.singBoxPath
}

func (d *Detector) XrayPath() string {
	return d.xrayPath
}

func (d *Detector) uniqueStrings(s []string) []string {
	seen := make(map[string]struct{})
	result := []string{}
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
