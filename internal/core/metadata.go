package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"ohmymc/internal/safefile"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type Artifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}
type Rule struct {
	Action string `json:"action"`
	OS     struct {
		Name    string `json:"name"`
		Arch    string `json:"arch"`
		Version string `json:"version"`
	} `json:"os"`
	Features map[string]bool `json:"features"`
}
type Library struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Rules     []Rule `json:"rules"`
	Downloads struct {
		Artifact    *Artifact           `json:"artifact"`
		Classifiers map[string]Artifact `json:"classifiers"`
	} `json:"downloads"`
	Natives map[string]string `json:"natives"`
	Extract struct {
		Exclude []string `json:"exclude"`
	} `json:"extract"`
}
type Metadata struct {
	ID                 string `json:"id"`
	InheritsFrom       string `json:"inheritsFrom"`
	Jar                string `json:"jar"`
	Type               string `json:"type"`
	MainClass          string `json:"mainClass"`
	MinecraftArguments string `json:"minecraftArguments"`
	Assets             string `json:"assets"`
	JavaVersion        struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
	Arguments struct {
		Game []json.RawMessage `json:"game"`
		JVM  []json.RawMessage `json:"jvm"`
	} `json:"arguments"`
	Libraries  []Library           `json:"libraries"`
	Downloads  map[string]Artifact `json:"downloads"`
	AssetIndex struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
	} `json:"assetIndex"`
	Logging struct {
		Client struct {
			Argument string `json:"argument"`
			File     struct {
				ID   string `json:"id"`
				URL  string `json:"url"`
				SHA1 string `json:"sha1"`
				Size int64  `json:"size"`
			} `json:"file"`
		} `json:"client"`
	} `json:"logging"`
}
type Platform struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Version string `json:"version"`
	Bits    string `json:"bits"`
}

func CurrentPlatform() Platform {
	osname := runtime.GOOS
	if osname == "darwin" {
		osname = "osx"
	}
	bits := "64"
	if runtime.GOARCH == "386" {
		bits = "32"
	}
	return Platform{OS: osname, Arch: runtime.GOARCH, Version: platformVersion(), Bits: bits}
}
func Resolve(root, id string) (Metadata, error) { return resolve(root, id, map[string]bool{}) }
func resolve(root, id string, seen map[string]bool) (Metadata, error) {
	var m Metadata
	if err := safefile.Component(id); err != nil {
		return m, err
	}
	if seen[id] || len(seen) > 32 {
		return m, errors.New("版本继承循环或过深")
	}
	seen[id] = true
	f, err := os.Open(filepath.Join(root, "versions", id, id+".json"))
	if err != nil {
		return m, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16<<20))
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.ID == "" {
		m.ID = id
	}
	if m.ID != id {
		return m, errors.New("版本 JSON 的 id 必须与文件夹名称一致")
	}
	// A legacy child supplies a complete game argument string; a modern child
	// supplies increments. Normalize before merging so neither form erases the
	// parent's implicit classpath or legacy identity arguments.
	legacyGame := m.MinecraftArguments != "" && len(m.Arguments.Game) == 0
	if legacyGame {
		args, err := SplitLegacy(m.MinecraftArguments)
		if err != nil {
			return m, err
		}
		m.Arguments.Game = rawArguments(append(args, "--width", "${resolution_width}", "--height", "${resolution_height}")...)
	}
	if m.InheritsFrom != "" {
		p, err := resolve(root, m.InheritsFrom, seen)
		if err != nil {
			return m, err
		}
		if m.MainClass == "" {
			m.MainClass = p.MainClass
		}
		if m.Type == "" {
			m.Type = p.Type
		}
		if m.MinecraftArguments == "" {
			m.MinecraftArguments = p.MinecraftArguments
		}
		if m.Assets == "" {
			m.Assets = p.Assets
		}
		if m.AssetIndex.ID == "" {
			m.AssetIndex = p.AssetIndex
		}
		if m.JavaVersion.MajorVersion == 0 {
			m.JavaVersion = p.JavaVersion
		}
		if m.Logging.Client.Argument == "" {
			m.Logging = p.Logging
		}
		if m.Jar == "" && m.Downloads["client"].URL == "" {
			m.Jar = p.Jar
		}
		if !legacyGame {
			m.Arguments.Game = append(p.Arguments.Game, m.Arguments.Game...)
		}
		m.Arguments.JVM = append(p.Arguments.JVM, m.Arguments.JVM...)
		overrides := map[string]bool{}
		for _, l := range m.Libraries {
			overrides[libraryKey(l.Name)] = true
		}
		merged := []Library{}
		for _, l := range p.Libraries {
			if !overrides[libraryKey(l.Name)] {
				merged = append(merged, l)
			}
		}
		m.Libraries = append(merged, m.Libraries...)
	}
	if m.Jar == "" {
		m.Jar = id
	}
	if err := safefile.Component(m.Jar); err != nil {
		return m, err
	}
	if m.JavaVersion.MajorVersion == 0 {
		m.JavaVersion.MajorVersion = 8
	}
	if m.MainClass == "" {
		return m, errors.New("版本缺少 mainClass（不支持早期 Applet 版本）")
	}
	if len(m.Arguments.JVM) == 0 {
		m.Arguments.JVM = append([]json.RawMessage{json.RawMessage(`{"rules":[{"action":"allow","os":{"name":"osx"}}],"value":"-XstartOnFirstThread"}`)}, rawArguments("-Djava.library.path=${natives_directory}", "-cp", "${classpath}")...)
	}
	return m, nil
}
func rawArguments(args ...string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(args))
	for _, arg := range args {
		b, _ := json.Marshal(arg)
		out = append(out, b)
	}
	return out
}
func libraryKey(name string) string {
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return name
	}
	key := strings.Join(parts[:2], ":")
	if len(parts) > 3 {
		key += ":" + strings.Join(parts[3:], ":")
	}
	return key
}
func Allowed(rules []Rule, p Platform, features map[string]bool) bool {
	allowed := len(rules) == 0
	for _, r := range rules {
		if r.OS.Name != "" && r.OS.Name != p.OS {
			continue
		}
		if r.OS.Arch != "" {
			arch := p.Arch
			if arch == "386" {
				arch = "x86"
			}
			if r.OS.Arch != arch && !(r.OS.Arch == "x86_64" && arch == "amd64") && !(r.OS.Arch == "aarch64" && arch == "arm64") {
				continue
			}
		}
		if r.OS.Version != "" {
			ok, err := regexp.MatchString(r.OS.Version, p.Version)
			if err != nil || !ok {
				continue
			}
		}
		match := true
		for k, want := range r.Features {
			if features[k] != want {
				match = false
			}
		}
		if match {
			allowed = r.Action == "allow"
		}
	}
	return allowed
}
func MavenPath(name string) (string, error) {
	ext := "jar"
	if a, b, ok := strings.Cut(name, "@"); ok {
		name = a
		ext = b
	}
	parts := strings.Split(name, ":")
	if len(parts) < 3 || len(parts) > 4 {
		return "", fmt.Errorf("无效的 Maven 坐标: %s", name)
	}
	file := parts[1] + "-" + parts[2]
	if len(parts) == 4 {
		file += "-" + parts[3]
	}
	p := strings.ReplaceAll(parts[0], ".", "/") + "/" + parts[1] + "/" + parts[2] + "/" + file + "." + ext
	return p, safefile.Relative(p)
}
func LibraryArtifact(l Library) (*Artifact, error) {
	if l.Downloads.Artifact != nil {
		a := *l.Downloads.Artifact
		if a.Path == "" {
			var err error
			a.Path, err = MavenPath(l.Name)
			if err != nil {
				return nil, err
			}
		}
		return &a, safefile.Relative(a.Path)
	}
	if len(l.Downloads.Classifiers) > 0 || len(l.Natives) > 0 {
		return nil, nil
	}
	p, err := MavenPath(l.Name)
	if err != nil {
		return nil, err
	}
	return &Artifact{Path: p}, nil
}
func NativeArtifact(l Library, p Platform) (*Artifact, error) {
	name := strings.ReplaceAll(l.Natives[p.OS], "${arch}", p.Bits)
	if name == "" {
		return nil, nil
	}
	a, ok := l.Downloads.Classifiers[name]
	if !ok && (len(l.Downloads.Classifiers) > 0 || l.Downloads.Artifact != nil) {
		return nil, fmt.Errorf("缺少本机原生依赖 %s (%s)", l.Name, name)
	}
	if a.Path == "" {
		coordinate, ext, hasExt := strings.Cut(l.Name, "@")
		parts := strings.Split(coordinate, ":")
		if len(parts) < 3 || len(parts) > 4 {
			return nil, fmt.Errorf("无效的 Maven 坐标: %s", l.Name)
		}
		coordinate = strings.Join(parts[:3], ":") + ":" + name
		if hasExt {
			coordinate += "@" + ext
		}
		var err error
		a.Path, err = MavenPath(coordinate)
		if err != nil {
			return nil, err
		}
	}
	return &a, safefile.Relative(a.Path)
}
func ExpandArguments(raw []json.RawMessage, p Platform, features map[string]bool, vars map[string]string) ([]string, error) {
	out := []string{}
	for _, b := range raw {
		var str string
		if json.Unmarshal(b, &str) == nil {
			v, err := substitute(str, vars)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			continue
		}
		var obj struct {
			Rules []Rule          `json:"rules"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			return nil, err
		}
		if !Allowed(obj.Rules, p, features) {
			continue
		}
		var values []string
		if json.Unmarshal(obj.Value, &str) == nil {
			values = []string{str}
		} else if err := json.Unmarshal(obj.Value, &values); err != nil {
			return nil, err
		}
		for _, v := range values {
			s, err := substitute(v, vars)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
	}
	return out, nil
}

var variablePattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func substitute(s string, vars map[string]string) (string, error) {
	var err error
	out := variablePattern.ReplaceAllStringFunc(s, func(key string) string {
		if v, ok := vars[key[2:len(key)-1]]; ok {
			return v
		}
		err = fmt.Errorf("未支持的启动参数占位符 %s", key)
		return key
	})
	return out, err
}
