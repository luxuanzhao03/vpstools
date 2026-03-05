package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type config struct {
	Targets              []string
	ReverseSSH           map[string]string
	LocalIP              string
	MaxHops              int
	WaitSec              int
	QueriesPerHop        int
	PingCount            int
	NoDNS                bool
	CommandTimeoutSec    int
	AutoInstallDeps      bool
	ThirdPartyReturn     bool
	ThirdPartyProvider   string
	ThirdPartyLocation   string
	ThirdPartyToken      string
	ThirdPartyProbeLimit int
	ThirdPartyTimeoutSec int
}

type report struct {
	Timestamp string        `json:"timestamp"`
	Hostname  string        `json:"hostname"`
	LocalIP   string        `json:"local_ip,omitempty"`
	Results   []targetProbe `json:"results"`
}

type targetProbe struct {
	Target   string       `json:"target"`
	Outbound traceResult  `json:"outbound"`
	Return   *traceResult `json:"return,omitempty"`
}

type traceResult struct {
	Direction        string       `json:"direction"`
	Command          string       `json:"command,omitempty"`
	Hops             []hopResult  `json:"hops"`
	DestinationRTTMs float64      `json:"destination_rtt_ms,omitempty"`
	Ping             *pingSummary `json:"ping,omitempty"`
	Error            string       `json:"error,omitempty"`
	RawOutput        string       `json:"raw_output,omitempty"`
}

type hopResult struct {
	TTL          int       `json:"ttl"`
	Host         string    `json:"host,omitempty"`
	IP           string    `json:"ip,omitempty"`
	LineName     string    `json:"line_name"`
	RTTMs        float64   `json:"rtt_ms,omitempty"`
	RTTSamplesMs []float64 `json:"rtt_samples_ms,omitempty"`
	Timeout      bool      `json:"timeout,omitempty"`
	Raw          string    `json:"raw"`
}

type pingSummary struct {
	Sent        int       `json:"sent"`
	Received    int       `json:"received"`
	LossPercent float64   `json:"loss_percent"`
	MinMs       float64   `json:"min_ms,omitempty"`
	AvgMs       float64   `json:"avg_ms,omitempty"`
	MaxMs       float64   `json:"max_ms,omitempty"`
	SamplesMs   []float64 `json:"samples_ms,omitempty"`
}

type lineRule struct {
	Name string
	Re   *regexp.Regexp
}

type linuxPackageManager struct {
	Name string
	Bin  string
}

var (
	hopPrefixRe   = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	rttRe         = regexp.MustCompile(`(?i)(<)?\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)
	timeoutLineRe = regexp.MustCompile(`(?i)request timed out|^\*+(?:\s+\*+)*$`)
	bracketIPRe   = regexp.MustCompile(`^(.*?)\s*\[([0-9a-fA-F:.]+)\]\s*$`)
	parenIPRe     = regexp.MustCompile(`^(.*?)\s*\(([0-9a-fA-F:.]+)\)\s*$`)
	ipv4Re        = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6Re        = regexp.MustCompile(`\b[0-9a-fA-F]*:[0-9a-fA-F:]+\b`)
    pingRTTRe     = regexp.MustCompile(`(?i)(?:time|\x{65f6}\x{95f4})[=<>]?\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)
)

var lineRules = []lineRule{
	{Name: "Timeout", Re: regexp.MustCompile(`^\*$`)},
	{Name: "Private Network", Re: regexp.MustCompile(`(?i)(^10\.|^192\.168\.|^172\.(1[6-9]|2[0-9]|3[0-1])\.|^127\.|^100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.)`)},
	{Name: "China Telecom CN2", Re: regexp.MustCompile(`(?i)(cn2|chinatelecom|ctgnet)`)},
	{Name: "China Telecom", Re: regexp.MustCompile(`(?i)(telecom|163\.com|189\.cn)`)},
	{Name: "China Unicom", Re: regexp.MustCompile(`(?i)(chinaunicom|unicom|cu\.|cncgroup)`)},
	{Name: "China Mobile", Re: regexp.MustCompile(`(?i)(chinamobile|cmcc|cmi\.|mobile)`)},
	{Name: "CERNET", Re: regexp.MustCompile(`(?i)cernet`)},
	{Name: "NTT", Re: regexp.MustCompile(`(?i)(\.ntt\.|ntt\.net)`)},
	{Name: "Telia", Re: regexp.MustCompile(`(?i)telia`)},
	{Name: "Cogent", Re: regexp.MustCompile(`(?i)cogent`)},
	{Name: "PCCW", Re: regexp.MustCompile(`(?i)pccw`)},
	{Name: "Hurricane Electric", Re: regexp.MustCompile(`(?i)(he\.net|hurricane)`)},
	{Name: "Google", Re: regexp.MustCompile(`(?i)google`)},
	{Name: "AWS", Re: regexp.MustCompile(`(?i)(amazon|aws)`)},
}

func main() {
	var (
		panelFlag            = flag.Bool("panel", false, "Run terminal panel mode")
		targetsFlag          = flag.String("targets", "", "Comma separated target IP/host list")
		reverseSSHFlag       = flag.String("reverse-ssh", "", "Comma separated mapping: target=ssh_endpoint")
		localIPFlag          = flag.String("local-ip", "", "Local reachable IP used by reverse trace (optional, auto-detected when needed)")
		maxHopsFlag          = flag.Int("max-hops", 30, "Max hops for traceroute")
		waitSecFlag          = flag.Int("wait-sec", 2, "Per probe timeout seconds")
		queryPerHopFlag      = flag.Int("queries-per-hop", 3, "Probes per hop")
		pingCountFlag        = flag.Int("ping-count", 4, "ICMP echo count (0 to disable)")
		noDNSFlag            = flag.Bool("no-dns", false, "Disable DNS lookup in traceroute")
		timeoutSecFlag       = flag.Int("cmd-timeout-sec", 120, "Command timeout seconds")
		autoInstallFlag      = flag.Bool("auto-install-deps", true, "Auto-install missing dependencies on Linux (requires root/sudo)")
		thirdPartyReturnFlag = flag.Bool("thirdparty-return", false, "Use third-party probe to approximate return path when SSH reverse trace is unavailable")
		thirdPartyProvider   = flag.String("thirdparty-provider", "globalping", "Third-party return provider (currently: globalping)")
		thirdPartyLocation   = flag.String("thirdparty-location", "", "Third-party probe location hint (empty means near target)")
		thirdPartyToken      = flag.String("thirdparty-token", "", "Third-party API token (or use GLOBALPING_TOKEN env)")
		thirdPartyProbeLimit = flag.Int("thirdparty-limit", 1, "Third-party probe count")
		thirdPartyTimeoutSec = flag.Int("thirdparty-timeout-sec", 90, "Third-party measurement timeout seconds")
		includeRawFlag       = flag.Bool("include-raw", false, "Include raw command output in JSON")
		outputFlag           = flag.String("out", "", "Write JSON report to file")
	)
	flag.Parse()

	if *panelFlag {
		if err := runTerminalPanel(); err != nil {
			exitWithError(err.Error())
		}
		return
	}

	targets := splitCSV(*targetsFlag)
	if len(targets) == 0 {
		exitWithError("-targets is required")
	}

	reverseMap, err := parseReverseMap(*reverseSSHFlag)
	if err != nil {
		exitWithError(err.Error())
	}

	cfg := config{
		Targets:              targets,
		ReverseSSH:           reverseMap,
		LocalIP:              strings.TrimSpace(*localIPFlag),
		MaxHops:              *maxHopsFlag,
		WaitSec:              *waitSecFlag,
		QueriesPerHop:        *queryPerHopFlag,
		PingCount:            *pingCountFlag,
		NoDNS:                *noDNSFlag,
		CommandTimeoutSec:    *timeoutSecFlag,
		AutoInstallDeps:      *autoInstallFlag,
		ThirdPartyReturn:     *thirdPartyReturnFlag,
		ThirdPartyProvider:   strings.TrimSpace(*thirdPartyProvider),
		ThirdPartyLocation:   strings.TrimSpace(*thirdPartyLocation),
		ThirdPartyToken:      strings.TrimSpace(*thirdPartyToken),
		ThirdPartyProbeLimit: *thirdPartyProbeLimit,
		ThirdPartyTimeoutSec: *thirdPartyTimeoutSec,
	}

	rep, err := generateReport(cfg, *includeRawFlag)
	if err != nil {
		exitWithError(err.Error())
	}

	jsonBytes, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		exitWithError(err.Error())
	}

	if *outputFlag != "" {
		if writeErr := os.WriteFile(*outputFlag, jsonBytes, 0644); writeErr != nil {
			exitWithError(fmt.Sprintf("write %s failed: %v", *outputFlag, writeErr))
		}
	}

	fmt.Println(string(jsonBytes))
}
func generateReport(cfg config, includeRaw bool) (report, error) {
	cfg = normalizeConfig(cfg)
	if len(cfg.Targets) == 0 {
		return report{}, errors.New("-targets is required")
	}
	if err := ensureReturnLocalIP(&cfg); err != nil {
		return report{}, err
	}

	if err := ensureLocalDependencies(cfg); err != nil {
		return report{}, err
	}

	hostname, _ := os.Hostname()
	rep := report{
		Timestamp: time.Now().Format(time.RFC3339),
		Hostname:  hostname,
		LocalIP:   cfg.LocalIP,
		Results:   make([]targetProbe, 0, len(cfg.Targets)),
	}

	for _, target := range cfg.Targets {
		res := targetProbe{Target: target}

		outbound, rawOut := runLocalTraceWithRetry(target, cfg)
		if includeRaw {
			outbound.RawOutput = rawOut
		}

		if cfg.PingCount > 0 {
			ping, pingCmd, pingRaw, pingErr := runLocalPing(target, cfg)
			if pingCmd != "" {
				outbound.Command = strings.TrimSpace(outbound.Command + " | " + pingCmd)
			}
			if pingErr != nil {
				outbound.Error = joinErrors(outbound.Error, fmt.Sprintf("ping failed: %v", pingErr))
			} else {
				outbound.Ping = ping
			}
			if includeRaw && pingRaw != "" {
				outbound.RawOutput = strings.TrimSpace(outbound.RawOutput + "\n\n# ping\n" + pingRaw)
			}
		}
		res.Outbound = outbound

		if endpoint, ok := cfg.ReverseSSH[target]; ok {
			back, rawBack := runReverseTrace(endpoint, cfg.LocalIP, cfg)
			if includeRaw {
				back.RawOutput = rawBack
			}
			res.Return = &back
		} else if cfg.ThirdPartyReturn {
			back, rawBack := runThirdPartyReverseTraceWithRetry(target, cfg.LocalIP, cfg)
			if includeRaw {
				back.RawOutput = rawBack
			}
			res.Return = &back
		}

		rep.Results = append(rep.Results, res)
	}

	return rep, nil
}

func normalizeConfig(cfg config) config {
	if cfg.MaxHops <= 0 {
		cfg.MaxHops = 30
	}
	if cfg.WaitSec <= 0 {
		cfg.WaitSec = 2
	}
	if cfg.QueriesPerHop <= 0 {
		cfg.QueriesPerHop = 3
	}
	if cfg.PingCount < 0 {
		cfg.PingCount = 0
	}
	if cfg.CommandTimeoutSec <= 0 {
		cfg.CommandTimeoutSec = 120
	}

	if strings.TrimSpace(cfg.ThirdPartyProvider) == "" {
		cfg.ThirdPartyProvider = "globalping"
	}
	if cfg.ThirdPartyProbeLimit <= 0 {
		cfg.ThirdPartyProbeLimit = 1
	}
	if cfg.ThirdPartyProbeLimit > 5 {
		cfg.ThirdPartyProbeLimit = 5
	}
	if cfg.ThirdPartyTimeoutSec <= 0 {
		cfg.ThirdPartyTimeoutSec = 90
	}
	if strings.TrimSpace(cfg.ThirdPartyToken) == "" {
		cfg.ThirdPartyToken = strings.TrimSpace(os.Getenv("GLOBALPING_TOKEN"))
	}

	return cfg
}
func runLocalTrace(target string, cfg config) (traceResult, string) {
	tr := traceResult{Direction: "outbound"}
	cmd, args, err := buildLocalTracerouteCommand(target, cfg)
	if err != nil {
		tr.Error = err.Error()
		return tr, ""
	}

	tr.Command = buildCommandString(cmd, args)
	output, runErr := runCommand(cmd, args, cfg.CommandTimeoutSec)
	tr.Hops = parseTracerouteOutput(output)
	tr.DestinationRTTMs = inferDestinationRTT(tr.Hops)

	if runErr != nil {
		tr.Error = fmt.Sprintf("traceroute failed: %v", runErr)
	}
	if len(tr.Hops) == 0 && tr.Error == "" {
		tr.Error = "traceroute returned no hop data"
	}
	return tr, output
}

func runReverseTrace(sshEndpoint, localIP string, cfg config) (traceResult, string) {
	tr := traceResult{Direction: "return"}

	if err := ensureRemoteTraceroute(sshEndpoint, cfg); err != nil {
		tr.Error = err.Error()
		return tr, ""
	}

	remoteCmd := buildRemoteTracerouteCommand(localIP, cfg)
	tr.Command = buildCommandString("ssh", []string{sshEndpoint, remoteCmd})

	output, err := runCommand("ssh", []string{sshEndpoint, remoteCmd}, cfg.CommandTimeoutSec)
	tr.Hops = parseTracerouteOutput(output)
	tr.DestinationRTTMs = inferDestinationRTT(tr.Hops)

	if err != nil {
		tr.Error = fmt.Sprintf("reverse trace failed: %v", err)
	}
	if len(tr.Hops) == 0 && tr.Error == "" {
		tr.Error = "reverse trace returned no hop data"
	}
	return tr, output
}

func runLocalPing(target string, cfg config) (*pingSummary, string, string, error) {
	cmd, args, err := buildPingCommand(target, cfg)
	if err != nil {
		return nil, "", "", err
	}

	output, runErr := runCommand(cmd, args, cfg.CommandTimeoutSec)
	summary := parsePingOutput(output, cfg.PingCount)
	if summary == nil && runErr == nil {
		runErr = errors.New("unable to parse ping output")
	}

	return summary, buildCommandString(cmd, args), output, runErr
}

func buildLocalTracerouteCommand(target string, cfg config) (string, []string, error) {
	if runtime.GOOS == "windows" {
		args := []string{"-h", strconv.Itoa(cfg.MaxHops), "-w", strconv.Itoa(cfg.WaitSec * 1000)}
		if cfg.NoDNS {
			args = append(args, "-d")
		}
		args = append(args, target)
		return "tracert", args, nil
	}

	if _, err := exec.LookPath("traceroute"); err != nil {
		return "", nil, errors.New("traceroute command not found in PATH")
	}

	args := []string{
		"-m", strconv.Itoa(cfg.MaxHops),
		"-w", strconv.Itoa(cfg.WaitSec),
		"-q", strconv.Itoa(cfg.QueriesPerHop),
	}
	if cfg.NoDNS {
		args = append(args, "-n")
	}
	args = append(args, target)
	return "traceroute", args, nil
}

func buildRemoteTracerouteCommand(target string, cfg config) string {
	parts := []string{
		"traceroute",
		"-m", strconv.Itoa(cfg.MaxHops),
		"-w", strconv.Itoa(cfg.WaitSec),
		"-q", strconv.Itoa(cfg.QueriesPerHop),
	}
	if cfg.NoDNS {
		parts = append(parts, "-n")
	}
	parts = append(parts, shellEscape(target))
	return strings.Join(parts, " ")
}

func buildPingCommand(target string, cfg config) (string, []string, error) {
	if cfg.PingCount <= 0 {
		return "", nil, errors.New("ping disabled")
	}

	if runtime.GOOS == "windows" {
		return "ping", []string{"-n", strconv.Itoa(cfg.PingCount), "-w", strconv.Itoa(cfg.WaitSec * 1000), target}, nil
	}

	if _, err := exec.LookPath("ping"); err != nil {
		return "", nil, errors.New("ping command not found in PATH")
	}
	return "ping", []string{"-c", strconv.Itoa(cfg.PingCount), "-W", strconv.Itoa(cfg.WaitSec), target}, nil
}

func ensureLocalDependencies(cfg config) error {
	required := requiredLocalCommands(cfg)
	missing := missingCommands(required)
	if len(missing) == 0 {
		return nil
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("missing local dependencies: %s (auto install currently supports Linux only)", strings.Join(missing, ", "))
	}
	if !cfg.AutoInstallDeps {
		return fmt.Errorf("missing local dependencies: %s", strings.Join(missing, ", "))
	}

	fmt.Fprintf(os.Stderr, "[dep] missing local dependencies: %s\n", strings.Join(missing, ", "))
	if err := installMissingLinuxCommands(missing, cfg.CommandTimeoutSec); err != nil {
		return err
	}

	missing = missingCommands(required)
	if len(missing) > 0 {
		return fmt.Errorf("dependencies still missing after auto install: %s", strings.Join(missing, ", "))
	}
	return nil
}

func ensureRemoteTraceroute(sshEndpoint string, cfg config) error {
	checkCmd := "command -v traceroute >/dev/null 2>&1"
	if _, err := runCommand("ssh", []string{sshEndpoint, checkCmd}, cfg.CommandTimeoutSec); err == nil {
		return nil
	}

	if !cfg.AutoInstallDeps {
		return fmt.Errorf("remote dependency missing on %s: traceroute", sshEndpoint)
	}

	fmt.Fprintf(os.Stderr, "[dep] traceroute missing on remote %s, attempting auto install\n", sshEndpoint)
	installCmd := buildRemoteEnsureTracerouteCommand()
	output, err := runCommand("ssh", []string{sshEndpoint, installCmd}, dependencyInstallTimeout(cfg.CommandTimeoutSec))
	if err != nil {
		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			return fmt.Errorf("failed auto-install on remote %s: %v: %s", sshEndpoint, err, trimmed)
		}
		return fmt.Errorf("failed auto-install on remote %s: %v", sshEndpoint, err)
	}

	if _, err := runCommand("ssh", []string{sshEndpoint, checkCmd}, cfg.CommandTimeoutSec); err != nil {
		return fmt.Errorf("traceroute still missing on remote %s after auto install", sshEndpoint)
	}
	return nil
}

func buildRemoteEnsureTracerouteCommand() string {
	parts := []string{
		"set -e",
		"if command -v traceroute >/dev/null 2>&1; then exit 0; fi",
		"SUDO=\"\"",
		"if [ \"$(id -u)\" -ne 0 ]; then if command -v sudo >/dev/null 2>&1; then SUDO=sudo; else echo \"need root or sudo\" >&2; exit 1; fi; fi",
		"if command -v apt-get >/dev/null 2>&1; then $SUDO apt-get update && $SUDO apt-get install -y --no-install-recommends traceroute",
		"elif command -v dnf >/dev/null 2>&1; then $SUDO dnf install -y traceroute",
		"elif command -v yum >/dev/null 2>&1; then $SUDO yum install -y traceroute",
		"elif command -v zypper >/dev/null 2>&1; then $SUDO zypper install -y traceroute",
		"elif command -v pacman >/dev/null 2>&1; then $SUDO pacman -Sy --noconfirm traceroute",
		"elif command -v apk >/dev/null 2>&1; then $SUDO apk add --no-cache traceroute",
		"else echo \"unsupported package manager\" >&2; exit 1",
		"fi",
		"command -v traceroute >/dev/null 2>&1",
	}
	return strings.Join(parts, "; ")
}

func requiredLocalCommands(cfg config) []string {
	set := make(map[string]struct{})
	if runtime.GOOS == "windows" {
		set["tracert"] = struct{}{}
	} else {
		set["traceroute"] = struct{}{}
	}
	if cfg.PingCount > 0 {
		set["ping"] = struct{}{}
	}
	if len(cfg.ReverseSSH) > 0 {
		set["ssh"] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for cmd := range set {
		out = append(out, cmd)
	}
	sort.Strings(out)
	return out
}

func missingCommands(commands []string) []string {
	missing := make([]string, 0)
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, cmd)
		}
	}
	return missing
}

func installMissingLinuxCommands(commands []string, timeoutSec int) error {
	pm, err := detectLinuxPackageManager()
	if err != nil {
		return err
	}

	useSudo, err := linuxShouldUseSudo()
	if err != nil {
		return err
	}
	if useSudo {
		if _, err := exec.LookPath("sudo"); err != nil {
			return errors.New("need root privileges or sudo to auto-install dependencies")
		}
	}

	installTimeout := dependencyInstallTimeout(timeoutSec)
	if pm.Name == "apt-get" {
		fmt.Fprintln(os.Stderr, "[dep] running apt-get update")
		if output, err := runPrivilegedCommand(pm.Bin, []string{"update"}, useSudo, installTimeout); err != nil {
			return fmt.Errorf("apt-get update failed: %v: %s", err, strings.TrimSpace(output))
		}
	}

	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err == nil {
			continue
		}

		candidates := commandPackageCandidates(pm.Name, cmd)
		if len(candidates) == 0 {
			return fmt.Errorf("no package mapping for command %q on %s", cmd, pm.Name)
		}

		installed := false
		var lastErr error
		for _, pkg := range candidates {
			fmt.Fprintf(os.Stderr, "[dep] installing %s for %s via %s\n", pkg, cmd, pm.Name)
			output, err := installPackageWithManager(*pm, pkg, useSudo, installTimeout)
			if err != nil {
				lastErr = fmt.Errorf("%v: %s", err, strings.TrimSpace(output))
				continue
			}
			if _, err := exec.LookPath(cmd); err == nil {
				installed = true
				break
			}
		}

		if !installed {
			if _, err := exec.LookPath(cmd); err == nil {
				installed = true
			}
		}

		if !installed {
			if lastErr != nil {
				return fmt.Errorf("failed to install dependency %q: %v", cmd, lastErr)
			}
			return fmt.Errorf("failed to install dependency %q", cmd)
		}
	}

	return nil
}

func detectLinuxPackageManager() (*linuxPackageManager, error) {
	candidates := []linuxPackageManager{
		{Name: "apt-get", Bin: "apt-get"},
		{Name: "dnf", Bin: "dnf"},
		{Name: "yum", Bin: "yum"},
		{Name: "zypper", Bin: "zypper"},
		{Name: "pacman", Bin: "pacman"},
		{Name: "apk", Bin: "apk"},
	}

	for _, pm := range candidates {
		if _, err := exec.LookPath(pm.Bin); err == nil {
			return &pm, nil
		}
	}
	return nil, errors.New("unable to auto-install dependencies: unsupported Linux package manager")
}

func linuxShouldUseSudo() (bool, error) {
	out, err := runCommand("id", []string{"-u"}, 5)
	if err != nil {
		return false, fmt.Errorf("unable to determine current user uid: %v", err)
	}
	return strings.TrimSpace(out) != "0", nil
}

func dependencyInstallTimeout(timeoutSec int) int {
	if timeoutSec < 600 {
		return 600
	}
	return timeoutSec
}

func commandPackageCandidates(pmName, command string) []string {
	mapping := map[string]map[string][]string{
		"apt-get": {
			"traceroute": {"traceroute"},
			"ping":       {"iputils-ping", "iputils"},
			"ssh":        {"openssh-client", "openssh"},
		},
		"dnf": {
			"traceroute": {"traceroute"},
			"ping":       {"iputils"},
			"ssh":        {"openssh-clients", "openssh"},
		},
		"yum": {
			"traceroute": {"traceroute"},
			"ping":       {"iputils"},
			"ssh":        {"openssh-clients", "openssh"},
		},
		"zypper": {
			"traceroute": {"traceroute"},
			"ping":       {"iputils"},
			"ssh":        {"openssh-clients", "openssh"},
		},
		"pacman": {
			"traceroute": {"traceroute"},
			"ping":       {"iputils"},
			"ssh":        {"openssh"},
		},
		"apk": {
			"traceroute": {"traceroute"},
			"ping":       {"iputils"},
			"ssh":        {"openssh-client", "openssh"},
		},
	}

	pmMap, ok := mapping[pmName]
	if !ok {
		return nil
	}
	return pmMap[command]
}

func installPackageWithManager(pm linuxPackageManager, pkg string, useSudo bool, timeoutSec int) (string, error) {
	var args []string

	switch pm.Name {
	case "apt-get":
		args = []string{"install", "-y", "--no-install-recommends", pkg}
	case "dnf":
		args = []string{"install", "-y", pkg}
	case "yum":
		args = []string{"install", "-y", pkg}
	case "zypper":
		args = []string{"install", "-y", pkg}
	case "pacman":
		args = []string{"-Sy", "--noconfirm", pkg}
	case "apk":
		args = []string{"add", "--no-cache", pkg}
	default:
		return "", fmt.Errorf("unsupported package manager: %s", pm.Name)
	}

	return runPrivilegedCommand(pm.Bin, args, useSudo, timeoutSec)
}

func runPrivilegedCommand(command string, args []string, useSudo bool, timeoutSec int) (string, error) {
	if useSudo {
		sudoArgs := make([]string, 0, len(args)+1)
		sudoArgs = append(sudoArgs, command)
		sudoArgs = append(sudoArgs, args...)
		return runCommand("sudo", sudoArgs, timeoutSec)
	}
	return runCommand(command, args, timeoutSec)
}

func runCommand(name string, args []string, timeoutSec int) (string, error) {
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timeout (%ds)", timeoutSec)
	}
	return string(output), err
}

func parseTracerouteOutput(raw string) []hopResult {
	lines := strings.Split(raw, "\n")
	hops := make([]hopResult, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "traceroute") ||
			strings.HasPrefix(lower, "tracing route") ||
			strings.HasPrefix(lower, "over a maximum") ||
			strings.HasPrefix(lower, "trace complete") {
			continue
		}

		m := hopPrefixRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}

		ttl, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}

		hop := parseHop(ttl, m[2])
		hops = append(hops, hop)
	}

	sort.SliceStable(hops, func(i, j int) bool { return hops[i].TTL < hops[j].TTL })
	return hops
}

func parseHop(ttl int, content string) hopResult {
	cleaned := strings.TrimSpace(content)
	hop := hopResult{
		TTL:  ttl,
		Raw:  cleaned,
		Host: "",
		IP:   "",
	}

	if timeoutLineRe.MatchString(strings.TrimSpace(cleaned)) {
		hop.Timeout = true
		hop.Host = "*"
		hop.LineName = "Timeout"
		return hop
	}

	samples := extractRTTSamples(cleaned)
	hop.RTTSamplesMs = samples
	if len(samples) > 0 {
		hop.RTTMs = round2(avg(samples))
	}

	identity := stripRTTContent(cleaned)
	host, ip := extractHostIP(identity)

	hop.Host = host
	hop.IP = ip

	if hop.Host == "" && strings.Contains(identity, "*") {
		hop.Host = "*"
		hop.Timeout = true
	}
	if hop.Host == "" && hop.IP != "" {
		hop.Host = hop.IP
	}
	if hop.Host == "" {
		hop.Host = strings.TrimSpace(identity)
	}

	hop.LineName = classifyLineName(hop.Host, hop.IP)
	return hop
}

func extractRTTSamples(s string) []float64 {
	matches := rttRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}

	samples := make([]float64, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		if strings.TrimSpace(m[1]) == "<" {
			v = math.Max(0.1, v/2)
		}
		samples = append(samples, round2(v))
	}
	return samples
}

func stripRTTContent(s string) string {
	noRTT := rttRe.ReplaceAllString(s, " ")
	replacers := []string{
		"Request timed out.", " ",
		"request timed out", " ",
		"ms", " ",
	}
	r := strings.NewReplacer(replacers...)
	noRTT = r.Replace(noRTT)
	noRTT = strings.Join(strings.Fields(noRTT), " ")
	return strings.TrimSpace(noRTT)
}

func extractHostIP(identity string) (string, string) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return "", ""
	}

	if m := bracketIPRe.FindStringSubmatch(identity); len(m) == 3 && validIP(m[2]) {
		return cleanHost(m[1], m[2]), m[2]
	}
	if m := parenIPRe.FindStringSubmatch(identity); len(m) == 3 && validIP(m[2]) {
		return cleanHost(m[1], m[2]), m[2]
	}

	if ip := firstIP(identity); ip != "" {
		host := strings.TrimSpace(strings.Replace(identity, ip, "", 1))
		host = cleanHost(host, ip)
		if host == "" {
			host = ip
		}
		return host, ip
	}

	return cleanHost(identity, ""), ""
}

func cleanHost(host, ip string) string {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "-[]()")
	host = strings.Join(strings.Fields(host), " ")
	if host == "" || host == ip {
		return host
	}

	if strings.EqualFold(host, "ms") || strings.EqualFold(host, "timeout") {
		return ""
	}
	return host
}

func firstIP(s string) string {
	for _, m := range ipv4Re.FindAllString(s, -1) {
		if validIP(m) {
			return m
		}
	}
	for _, m := range ipv6Re.FindAllString(s, -1) {
		if validIP(m) {
			return m
		}
	}
	return ""
}

func validIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}

func classifyLineName(host, ip string) string {
    host = strings.TrimSpace(host)
    ip = strings.TrimSpace(ip)

    if host == "*" {
        return "Timeout"
    }

    if ip != "" {
        parsed := net.ParseIP(ip)
        if parsed != nil {
            if parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() {
                return "Private Network"
            }
        }
    }

    text := strings.ToLower(strings.TrimSpace(host + " " + ip))
    for _, rule := range lineRules {
        if rule.Re.MatchString(text) {
            return rule.Name
        }
    }

    if route := lookupRouteByTextDatabase(text); route != "" {
        return route
    }
    if route := lookupRouteByIPDatabase(ip); route != "" {
        return route
    }

    if ip != "" {
        return "AS/Carrier Unknown"
    }
    if host != "" {
        return "Name Unknown"
    }
    return "Unknown"
}

func inferDestinationRTT(hops []hopResult) float64 {
	for i := len(hops) - 1; i >= 0; i-- {
		if hops[i].Timeout {
			continue
		}
		if hops[i].RTTMs > 0 {
			return hops[i].RTTMs
		}
		if len(hops[i].RTTSamplesMs) > 0 {
			return round2(avg(hops[i].RTTSamplesMs))
		}
	}
	return 0
}

func parsePingOutput(raw string, sent int) *pingSummary {
	samples := make([]float64, 0, sent)
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		lower := strings.ToLower(line)
        if !(strings.Contains(lower, "time") || strings.Contains(line, "\u65f6\u95f4")) {
			continue
		}

		// Only count actual echo reply lines; ignore summary lines like
		// "4 packets transmitted... time 3005ms".
		isReplyLine := strings.Contains(lower, "bytes from") ||
			strings.Contains(lower, "reply from") ||
			strings.Contains(lower, "icmp_seq") ||
			strings.Contains(lower, "ttl=") ||
            strings.Contains(line, "\u6765\u81ea")
		if !isReplyLine {
			continue
		}

		m := pingRTTRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}

		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		samples = append(samples, round2(v))
	}

	if len(samples) == 0 {
		return nil
	}

	minVal := samples[0]
	maxVal := samples[0]
	sum := 0.0
	for _, v := range samples {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sum += v
	}

	recv := len(samples)
	loss := 0.0
	if sent > 0 {
		loss = round2((1 - float64(recv)/float64(sent)) * 100)
	}

	return &pingSummary{
		Sent:        sent,
		Received:    recv,
		LossPercent: loss,
		MinMs:       round2(minVal),
		AvgMs:       round2(sum / float64(recv)),
		MaxMs:       round2(maxVal),
		SamplesMs:   samples,
	}
}

func parseReverseMap(raw string) (map[string]string, error) {
	result := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result, nil
	}

	pairs := splitCSV(raw)
	for _, p := range pairs {
		if !strings.Contains(p, "=") {
			return nil, fmt.Errorf("invalid -reverse-ssh item %q, expected target=ssh_endpoint", p)
		}
		parts := strings.SplitN(p, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" || val == "" {
			return nil, fmt.Errorf("invalid -reverse-ssh item %q", p)
		}
		result[key] = val
	}

	return result, nil
}

func splitCSV(raw string) []string {
	items := strings.Split(raw, ",")
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{})

	for _, it := range items {
		v := strings.TrimSpace(it)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func buildCommandString(name string, args []string) string {
	escaped := make([]string, 0, len(args)+1)
	escaped = append(escaped, name)
	for _, a := range args {
		escaped = append(escaped, shellEscape(a))
	}
	return strings.Join(escaped, " ")
}

func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'`$&|;<>()[]{}") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func joinErrors(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}

func exitWithError(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}













