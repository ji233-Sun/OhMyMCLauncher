package core

import (
	"archive/zip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"ohmymc/internal/safefile"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type LaunchOptions struct {
	PlayerName  string `json:"playerName"`
	JavaPath    string `json:"javaPath"`
	MinMemoryMB int    `json:"minMemoryMB"`
	MaxMemoryMB int    `json:"maxMemoryMB"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Fullscreen  bool   `json:"fullscreen"`
	JVMArgs     string `json:"jvmArgs"`
	GameArgs    string `json:"gameArgs"`
}

func DefaultOptions() LaunchOptions {
	return LaunchOptions{PlayerName: "Player", MinMemoryMB: 512, MaxMemoryMB: 2048, Width: 1280, Height: 720}
}

var playerPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)

func ValidatePlayerName(name string) error {
	if !playerPattern.MatchString(name) {
		return errors.New("离线名称只能使用 1–16 个字母、数字或下划线")
	}
	return nil
}

func (o LaunchOptions) Validate() error {
	if err := ValidatePlayerName(o.PlayerName); err != nil {
		return err
	}
	if o.MinMemoryMB < 128 || o.MaxMemoryMB < o.MinMemoryMB || o.MaxMemoryMB > 262144 {
		return errors.New("内存须满足 128 ≤ 最小值 ≤ 最大值 ≤ 262144 MiB")
	}
	if o.Width < 320 || o.Width > 16384 || o.Height < 240 || o.Height > 16384 {
		return errors.New("窗口分辨率超出允许范围")
	}
	if len(o.JVMArgs)+len(o.GameArgs) > 32768 || strings.ContainsRune(o.JVMArgs+o.GameArgs+o.JavaPath, 0) {
		return errors.New("附加参数过长或含 NUL")
	}
	return nil
}

type LaunchPlan struct {
	Executable       string   `json:"executable"`
	Args             []string `json:"args"`
	WorkingDirectory string   `json:"workingDirectory"`
	JavaMajor        int      `json:"javaMajor"`
	Warnings         []string `json:"warnings,omitempty"`
}

func OfflineUUID(name string) string {
	b := md5.Sum([]byte("OfflinePlayer:" + name))
	b[6] = (b[6] & 15) | 0x30
	b[8] = (b[8] & 63) | 0x80
	return hex.EncodeToString(b[:])
}
func BuildLaunch(ctx context.Context, v Version, o LaunchOptions, dataRoot string, prepare bool) (LaunchPlan, error) {
	if err := o.Validate(); err != nil {
		return LaunchPlan{}, err
	}
	m, err := Resolve(v.Directory, v.VersionID)
	if err != nil {
		return LaunchPlan{}, err
	}
	j, err := ChooseJava(ctx, o.JavaPath, m.JavaVersion.MajorVersion)
	if err != nil {
		return LaunchPlan{}, err
	}
	nativeDir := filepath.Join(dataRoot, "natives", v.ID)
	p := CurrentPlatform()
	plan, err := Plan(v, m, o, j, p, nativeDir)
	if err != nil {
		return plan, err
	}
	if prepare {
		if err = os.MkdirAll(v.GameDirectory, 0700); err != nil {
			return plan, err
		}
		if err = os.MkdirAll(nativeDir, 0700); err != nil {
			return plan, err
		}
		for _, l := range m.Libraries {
			if !Allowed(l.Rules, p, nil) {
				continue
			}
			a, err := NativeArtifact(l, p)
			if err != nil {
				return plan, err
			}
			if a != nil {
				if err = ExtractNatives(filepath.Join(v.Directory, "libraries", filepath.FromSlash(a.Path)), nativeDir, l.Extract.Exclude); err != nil {
					return plan, err
				}
			}
		}
	}
	return plan, nil
}
func Plan(v Version, m Metadata, o LaunchOptions, j Java, p Platform, nativeDir string) (LaunchPlan, error) {
	plan := LaunchPlan{Executable: j.Path, WorkingDirectory: v.GameDirectory, JavaMajor: j.Major}
	if err := o.Validate(); err != nil {
		return plan, err
	}
	cp := []string{}
	missing := []string{}
	seen := map[string]bool{}
	check := func(path string) {
		fi, err := os.Stat(path)
		if err != nil || !fi.Mode().IsRegular() {
			missing = append(missing, path)
		}
	}
	for _, l := range m.Libraries {
		if !Allowed(l.Rules, p, nil) {
			continue
		}
		a, err := LibraryArtifact(l)
		if err != nil {
			return plan, err
		}
		if a != nil {
			path := filepath.Join(v.Directory, "libraries", filepath.FromSlash(a.Path))
			check(path)
			if !seen[path] {
				cp = append(cp, path)
				seen[path] = true
			}
		}
		n, err := NativeArtifact(l, p)
		if err != nil {
			return plan, err
		}
		if n != nil {
			check(filepath.Join(v.Directory, "libraries", filepath.FromSlash(n.Path)))
		}
	}
	if err := safefile.Component(m.Jar); err != nil {
		return plan, err
	}
	jar := filepath.Join(v.Directory, "versions", m.Jar, m.Jar+".jar")
	check(jar)
	cp = append(cp, jar)
	assets := filepath.Join(v.Directory, "assets")
	gameAssets := assets
	indexID := m.AssetIndex.ID
	if indexID == "" {
		indexID = m.Assets
	}
	if indexID != "" {
		if err := safefile.Component(indexID); err != nil {
			return plan, err
		}
		idx := filepath.Join(assets, "indexes", indexID+".json")
		check(idx)
		if b, err := os.ReadFile(idx); err == nil {
			var flags struct {
				Virtual bool `json:"virtual"`
				Map     bool `json:"map_to_resources"`
			}
			_ = json.Unmarshal(b, &flags)
			if flags.Virtual {
				gameAssets = filepath.Join(assets, "virtual", indexID)
			}
			if flags.Map {
				gameAssets = filepath.Join(v.GameDirectory, "resources")
			}
		}
	}
	vars := map[string]string{"auth_player_name": o.PlayerName, "auth_uuid": OfflineUUID(o.PlayerName), "auth_access_token": "0", "auth_session": "0", "auth_xuid": "", "clientid": "", "user_type": "legacy", "user_properties": "{}", "profile_name": v.Name, "version_name": v.VersionID, "version_type": m.Type, "game_directory": v.GameDirectory, "assets_root": assets, "assets_index_name": indexID, "game_assets": gameAssets, "natives_directory": nativeDir, "launcher_name": "OhMyMC", "launcher_version": "0.1.0", "classpath": strings.Join(cp, string(os.PathListSeparator)), "classpath_separator": string(os.PathListSeparator), "library_directory": filepath.Join(v.Directory, "libraries"), "resolution_width": strconv.Itoa(o.Width), "resolution_height": strconv.Itoa(o.Height)}
	features := map[string]bool{"has_custom_resolution": true}
	jvm, err := ExpandArguments(m.Arguments.JVM, p, features, vars)
	if err != nil {
		return plan, err
	}
	if len(jvm) == 0 {
		jvm = []string{"-Djava.library.path=" + nativeDir, "-cp", vars["classpath"]}
		if p.OS == "osx" {
			jvm = append([]string{"-XstartOnFirstThread"}, jvm...)
		}
	}
	jvm = append(jvm, fmt.Sprintf("-Xms%dM", o.MinMemoryMB), fmt.Sprintf("-Xmx%dM", o.MaxMemoryMB), "-Dlog4j2.formatMsgNoLookups=true")
	if m.Logging.Client.Argument != "" {
		id := m.Logging.Client.File.ID
		if err := safefile.Component(id); err != nil {
			return plan, err
		}
		path := filepath.Join(assets, "log_configs", id)
		_, statErr := os.Stat(path)
		if errors.Is(statErr, os.ErrNotExist) && m.Logging.Client.Argument == "-Dlog4j.configurationFile=${path}" && hasModernLog4j(m.Libraries, p) {
			// Some launchers never download Mojang's optional logging XML.
			// Only allow the default configuration with known-fixed Log4j;
			// legacy versions may rely on an external security configuration.
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("未找到外部日志配置 %s；使用 Log4j 默认配置，日志格式可能不同。", path))
		} else {
			check(path)
			arg, err := substitute(m.Logging.Client.Argument, map[string]string{"path": path})
			if err != nil {
				return plan, err
			}
			jvm = append(jvm, arg)
		}
	}
	jvm = append(jvm, argumentLines(o.JVMArgs)...)
	game, err := ExpandArguments(m.Arguments.Game, p, features, vars)
	if err != nil {
		return plan, err
	}
	if len(m.Arguments.Game) == 0 {
		args, err := SplitLegacy(m.MinecraftArguments)
		if err != nil {
			return plan, err
		}
		for _, arg := range args {
			expanded, err := substitute(arg, vars)
			if err != nil {
				return plan, err
			}
			game = append(game, expanded)
		}
		game = append(game, "--width", strconv.Itoa(o.Width), "--height", strconv.Itoa(o.Height))
	}
	if o.Fullscreen {
		game = append(game, "--fullscreen")
	}
	game = append(game, argumentLines(o.GameArgs)...)
	plan.Args = append(append(jvm, m.MainClass), game...)
	if len(missing) > 0 {
		count := len(missing)
		if count > 5 {
			missing = missing[:5]
		}
		return plan, fmt.Errorf("缺少 %d 个启动文件，请通过下载插件重新安装/补全，或检查导入目录:\n%s", count, strings.Join(missing, "\n"))
	}
	return plan, nil
}

// Be conservative: only stable Log4j 2.17.1+ releases can omit an external
// configuration. Unknown versions and any active older core keep strict checks.
func hasModernLog4j(libraries []Library, p Platform) bool {
	found := false
	for _, library := range libraries {
		if !Allowed(library.Rules, p, nil) {
			continue
		}
		parts := strings.Split(library.Name, ":")
		if len(parts) < 3 || parts[0] != "org.apache.logging.log4j" || parts[1] != "log4j-core" {
			continue
		}
		version := strings.Split(parts[2], ".")
		if len(parts) != 3 || len(version) != 3 || version[0] != "2" {
			return false
		}
		minor, minorErr := strconv.Atoi(version[1])
		patch, patchErr := strconv.Atoi(version[2])
		if minorErr != nil || patchErr != nil || patch < 0 || minor < 17 || (minor == 17 && patch < 1) {
			return false
		}
		found = true
	}
	return found
}
func argumentLines(s string) []string {
	out := []string{}
	for _, v := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// SplitLegacy tokenizes the legacy metadata before substitution; it never invokes a shell.
func SplitLegacy(s string) ([]string, error) {
	out := []string{}
	var b strings.Builder
	var quote rune
	active := false
	for _, r := range s {
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			active = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			active = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if active {
				out = append(out, b.String())
				b.Reset()
				active = false
			}
			continue
		}
		b.WriteRune(r)
		active = true
	}
	if quote != 0 {
		return nil, errors.New("版本参数中有未闭合的引号")
	}
	if active {
		out = append(out, b.String())
	}
	return out, nil
}
func ExtractNatives(archive, dest string, excludes []string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	if err = os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	var total uint64
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		skip := strings.HasPrefix(f.Name, "META-INF/")
		for _, e := range excludes {
			if strings.HasPrefix(f.Name, e) {
				skip = true
			}
		}
		if skip {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("原生库不允许符号链接")
		}
		if err = safefile.Relative(f.Name); err != nil {
			return err
		}
		total += f.UncompressedSize64
		if total > 512<<20 || f.UncompressedSize64 > 128<<20 {
			return errors.New("原生库解压大小超限")
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(r, 128<<20+1))
		r.Close()
		if err != nil {
			return err
		}
		if len(b) > 128<<20 {
			return errors.New("原生库过大")
		}
		if err = safefile.AtomicRoot(root, f.Name, b); err != nil {
			return err
		}
	}
	return nil
}
