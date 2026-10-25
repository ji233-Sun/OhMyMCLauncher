package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Java struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Major   int    `json:"major"`
	Arch    string `json:"arch"`
}

var javaVersion = regexp.MustCompile(`(?:java|openjdk) version "([^"]+)"`)
var javaArchPattern = regexp.MustCompile(`(?m)^\s*os\.arch\s*=\s*(\S+)`)

func normalizeJavaArch(arch string) string {
	switch strings.ToLower(arch) {
	case "x86", "i386", "i486", "i586", "i686", "386":
		return "386"
	case "amd64", "x86_64", "x64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return strings.ToLower(arch)
	}
}
func validateJavaArchitecture(j Java) error {
	if j.Arch != runtime.GOARCH {
		return fmt.Errorf("Java 架构 %s 与启动器/原生库架构 %s 不匹配，请选择对应架构的 Java", j.Arch, runtime.GOARCH)
	}
	return nil
}

func InspectJava(ctx context.Context, path string) (Java, error) {
	j := Java{Path: path}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	probe := path
	if strings.EqualFold(filepath.Base(path), "javaw.exe") {
		p := filepath.Join(filepath.Dir(path), "java.exe")
		if _, err := os.Stat(p); err == nil {
			probe = p
		}
	}
	cmd := exec.CommandContext(ctx, probe, "-XshowSettings:properties", "-version")
	configureProcess(cmd)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return j, fmt.Errorf("无法运行 Java %s: %w", path, err)
	}
	m := javaVersion.FindStringSubmatch(string(b))
	if len(m) != 2 {
		return j, errors.New("无法识别 Java 版本: " + string(b))
	}
	arch := javaArchPattern.FindStringSubmatch(string(b))
	if len(arch) != 2 {
		return j, errors.New("无法识别 Java 架构，请使用支持 -XshowSettings:properties 的 Java")
	}
	j.Arch = normalizeJavaArch(arch[1])
	j.Version = m[1]
	parts := strings.Split(j.Version, ".")
	major := parts[0]
	if major == "1" && len(parts) > 1 {
		major = parts[1]
	}
	major = strings.Split(major, "-")[0]
	j.Major, err = strconv.Atoi(major)
	if err != nil {
		return j, err
	}
	j.Path, err = filepath.Abs(path)
	return j, err
}
func DetectJava(ctx context.Context) []Java {
	exe := "java"
	if runtime.GOOS == "windows" {
		exe = "java.exe"
	}
	paths := []string{}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		paths = append(paths, filepath.Join(home, "bin", exe))
	}
	if path, err := exec.LookPath(exe); err == nil {
		paths = append(paths, path)
	}
	patterns := []string{"/usr/lib/jvm/*/bin/java", "/Library/Java/JavaVirtualMachines/*/Contents/Home/bin/java", "/opt/java/*/bin/java"}
	if runtime.GOOS == "windows" {
		patterns = nil
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
			base := os.Getenv(env)
			if base == "" {
				continue
			}
			for _, vendor := range []string{"Java", "Eclipse Adoptium", "Microsoft", "Zulu", "BellSoft", "Programs\\Eclipse Adoptium"} {
				patterns = append(patterns, filepath.Join(base, vendor, "*", "bin", exe))
			}
		}
	}
	for _, p := range patterns {
		matches, _ := filepath.Glob(p)
		paths = append(paths, matches...)
	}
	seen := map[string]bool{}
	out := []Java{}
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		if j, err := InspectJava(ctx, p); err == nil {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Major < out[j].Major })
	return out
}
func ChooseJava(ctx context.Context, path string, required int) (Java, error) {
	if path != "" {
		j, err := InspectJava(ctx, path)
		if err != nil {
			return j, err
		}
		if err = validateJavaArchitecture(j); err != nil {
			return j, err
		}
		if j.Major < required {
			return j, fmt.Errorf("此版本至少需要 Java %d，所选为 Java %d", required, j.Major)
		}
		return j, nil
	}
	all := DetectJava(ctx)
	for _, j := range all {
		if j.Major == required && validateJavaArchitecture(j) == nil {
			return j, nil
		}
	}
	for _, j := range all {
		if j.Major >= required && validateJavaArchitecture(j) == nil {
			return j, nil
		}
	}
	return Java{}, fmt.Errorf("未找到 %s 架构的 Java %d 或更新版本。请安装合适的 Java，或在启动参数插件中指定路径", runtime.GOARCH, required)
}
