package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	userCancelText             = "用户取消"
	defaultChinaMainlandTarget = "223.5.5.5"
	defaultUSWestTarget        = "74.82.42.42"
	defaultTargetsCSV          = defaultChinaMainlandTarget + "," + defaultUSWestTarget
)

type panelTool struct {
	ID          string
	Name        string
	Description string
}

type panelState struct {
	LastReport *report
	LastErr    string
}

var terminalTools = []panelTool{
	{
		ID:          "routeprobe",
		Name:        "去程回程线路探测",
		Description: "测量本机到目标 IP 的去程/回程线路名称与延迟",
	},
}

func runTerminalPanel() error {
	reader := bufio.NewReader(os.Stdin)
	state := &panelState{}

	for {
		if state.LastErr == userCancelText {
			state.LastErr = ""
		}

		clearTerminal()
		renderPanelHeader(state)
		renderToolMenu()

		choice, err := promptString(reader, "请选择编号")
		if err != nil {
			if isUserCancel(err) {
				return nil
			}
			return err
		}

		switch strings.ToLower(strings.TrimSpace(choice)) {
		case "1":
			runRouteProbeTool(reader, state)
		case "0", "q", "quit", "exit":
			return nil
		default:
			fmt.Printf("\n无效选择: %s\n", strings.TrimSpace(choice))
			waitForEnter(reader)
		}
	}
}

func renderPanelHeader(state *panelState) {
	fmt.Println("VPS 工具包 - 终端面板")
	fmt.Println("========================================================")

	if state.LastErr != "" {
		fmt.Printf("上次执行: 失败 - %s\n", state.LastErr)
	} else if state.LastReport != nil {
		fmt.Printf("上次执行: 成功 - %s, 目标 %d 个\n", state.LastReport.Timestamp, len(state.LastReport.Results))
	} else {
		fmt.Println("上次执行: 暂无记录")
	}

	fmt.Println()
}

func renderToolMenu() {
	for i, tool := range terminalTools {
		fmt.Printf("%d) %s\n", i+1, tool.Name)
		fmt.Printf("   %s\n", tool.Description)
	}
	fmt.Println("0) 退出")
	fmt.Println()
}

func runRouteProbeTool(reader *bufio.Reader, state *panelState) {
	clearTerminal()
	fmt.Println("[去程回程线路探测]")
	fmt.Println("留空将使用默认值。输入 q 可中止并返回主菜单。")
	fmt.Printf("默认目标: %s (中国大陆), %s (美国西海岸)\n", defaultChinaMainlandTarget, defaultUSWestTarget)
	fmt.Println()

	targetsRaw, err := promptStringDefault(reader, "目标地址（逗号分隔，直接回车用默认）", defaultTargetsCSV)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	reverseSSHRaw, err := promptStringDefault(reader, "回程 SSH 映射（可选，格式 target=user@host）", "")
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	localIP, err := promptStringDefault(reader, "本机可回测 IP（可选，留空自动获取）", "")
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	thirdPartyReturn, err := promptBoolDefault(reader, "无 SSH 时启用第三方回程（近似）", true)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	thirdPartyLocation := ""
	if thirdPartyReturn {
		thirdPartyLocation, err = promptStringDefault(reader, "第三方探针位置（可选，留空按目标就近）", "")
		if err != nil {
			handleToolError(reader, state, err)
			return
		}
	}

	maxHops, err := promptIntDefault(reader, "最大 hops", 30, 1)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	waitSec, err := promptIntDefault(reader, "单探针等待秒数", 2, 1)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	queriesPerHop, err := promptIntDefault(reader, "每跳探针数", 3, 1)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	pingCount, err := promptIntDefault(reader, "Ping 次数（0 关闭）", 4, 0)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	noDNS, err := promptBoolDefault(reader, "禁用 DNS 反查", false)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	autoInstall, err := promptBoolDefault(reader, "自动安装缺失依赖（Linux）", true)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	includeRaw, err := promptBoolDefault(reader, "输出原始命令文本", false)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	timeoutSec, err := promptIntDefault(reader, "命令超时秒数", 120, 10)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}

	reverseMap, err := parseReverseMap(reverseSSHRaw)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	if strings.TrimSpace(localIP) == "" && (len(reverseMap) > 0 || thirdPartyReturn) {
		fmt.Println("提示: 已启用回程测量，将自动获取本机可回测 IP。")
	}

	cfg := config{
		Targets:              resolveTargets(targetsRaw),
		ReverseSSH:           reverseMap,
		LocalIP:              strings.TrimSpace(localIP),
		MaxHops:              maxHops,
		WaitSec:              waitSec,
		QueriesPerHop:        queriesPerHop,
		PingCount:            pingCount,
		NoDNS:                noDNS,
		CommandTimeoutSec:    timeoutSec,
		AutoInstallDeps:      autoInstall,
		ThirdPartyReturn:     thirdPartyReturn,
		ThirdPartyProvider:   "globalping",
		ThirdPartyLocation:   strings.TrimSpace(thirdPartyLocation),
		ThirdPartyProbeLimit: 1,
		ThirdPartyTimeoutSec: 90,
		ThirdPartyHTTPRequestTimeoutSec: 20,
	}

	fmt.Println()
	fmt.Println("开始探测，请稍候...")
	rep, runErr := generateReport(cfg, includeRaw)
	if runErr != nil {
		handleToolError(reader, state, runErr)
		return
	}

	state.LastErr = ""
	state.LastReport = &rep

	jsonBytes, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		handleToolError(reader, state, err)
		return
	}

	clearTerminal()
	printFriendlyReport(rep)

	showDetail, err := promptBoolDefault(reader, "是否查看详细 JSON", false)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	if showDetail {
		fmt.Println()
		fmt.Println("[详细 JSON]")
		fmt.Println(string(jsonBytes))
	}

	saveFile, err := promptBoolDefault(reader, "是否保存结果到文件", true)
	if err != nil {
		handleToolError(reader, state, err)
		return
	}
	if saveFile {
		defaultPath := fmt.Sprintf("routeprobe-%s.json", time.Now().Format("20060102-150405"))
		outPath, perr := promptStringDefault(reader, "输出文件路径", defaultPath)
		if perr != nil {
			handleToolError(reader, state, perr)
			return
		}
		if strings.TrimSpace(outPath) != "" {
			if werr := os.WriteFile(strings.TrimSpace(outPath), jsonBytes, 0644); werr != nil {
				handleToolError(reader, state, werr)
				return
			}
			fmt.Printf("\n结果已写入: %s\n", strings.TrimSpace(outPath))
		}
	}

	fmt.Println()
	waitForEnter(reader)
}

func printFriendlyReport(rep report) {
	targets := make([]string, 0, len(rep.Results))
	for _, item := range rep.Results {
		targets = append(targets, item.Target)
	}
	prefetchTargetLocations(targets)

	fmt.Println("[线路测量结果]")
	fmt.Printf("测试时间: %s\n", rep.Timestamp)
	fmt.Printf("服务器: %s\n", rep.Hostname)
	fmt.Println("说明: 线路名称为智能识别，可能存在少量误差。")
	fmt.Println("--------------------------------------------------------")

	for _, item := range rep.Results {
		outRoute := detectMajorRoute(item.Outbound.Hops)
		outLatency := pickLatency(item.Outbound)
		targetLabel := formatTargetCNLabel(item.Target)

		fmt.Printf("目标地区: %s\n", targetLabel)
		fmt.Printf("  去程线路: %s\n", prettyRouteName(outRoute))
		fmt.Printf("  去程延迟: %s\n", formatLatency(outLatency))

		backRoute := ""
		if item.Return != nil {
			backRoute = detectMajorRoute(item.Return.Hops)
			backLatency := pickLatency(*item.Return)
			fmt.Printf("  回程线路: %s\n", prettyRouteName(backRoute))
			fmt.Printf("  回程延迟: %s\n", formatLatency(backLatency))
		} else {
			fmt.Println("  回程线路: 未测")
			fmt.Println("  回程延迟: 未测")
		}

		if item.Outbound.Error != "" {
			fmt.Printf("  去程备注: %s\n", friendlyProbeRemark(item.Outbound.Error, false))
		} else if outRoute != "未识别" && !isChinaCarrierRoute(outRoute) {
			fmt.Println("  去程备注: 仅识别到境外骨干段，未能识别到国内运营商段。")
		}
		if item.Return != nil && item.Return.Error != "" {
			fmt.Printf("  回程备注: %s\n", friendlyProbeRemark(item.Return.Error, true))
		} else if item.Return != nil && backRoute != "" && backRoute != "未识别" && !isChinaCarrierRoute(backRoute) {
			fmt.Println("  回程备注: 仅识别到境外骨干段，未能识别到入境后的国内运营商段。")
		}

		fmt.Println("--------------------------------------------------------")
	}
}

func detectMajorRoute(hops []hopResult) string {
	if route := detectRouteByHopDatabase(hops); route != "" {
		return route
	}

	text := collectPathText(hops)
	if text == "" {
		return "未识别"
	}

	if route := detectRouteByKeywords(text); route != "" {
		return route
	}
	if route := lookupRouteByTextDatabase(text); route != "" {
		return route
	}

	return "未识别"
}

func detectRouteByHopDatabase(hops []hopResult) string {
	scores := make(map[string]int, len(hops))
	for _, hop := range hops {
		if hop.Timeout {
			continue
		}
		if route := normalizeRouteAlias(hop.LineName); route != "" {
			scores[route] += 3
		}
		if route := lookupRouteByIPDatabase(hop.IP); route != "" {
			scores[route] += 5
		}
		if route := lookupRouteByTextDatabase(strings.ToLower(hop.Raw+" "+hop.Host)); route != "" {
			scores[route] += 2
		}
	}

	bestCN := ""
	bestCNScore := 0
	bestAny := ""
	bestAnyScore := 0
	for route, score := range scores {
		if score > bestAnyScore {
			bestAnyScore = score
			bestAny = route
		}
		if isChinaCarrierRoute(route) && score > bestCNScore {
			bestCNScore = score
			bestCN = route
		}
	}

	if bestCN != "" {
		return bestCN
	}
	return bestAny
}

func prettyRouteName(route string) string {
	route = strings.TrimSpace(route)
	if route == "" || route == "未识别" {
		return "暂未识别"
	}
	if isForeignBackboneRoute(route) {
		return "境外骨干"
	}
	return route
}

func friendlyProbeRemark(err string, isReturn bool) string {
	msg := strings.TrimSpace(err)
	if msg == "" {
		return ""
	}

	lower := strings.ToLower(msg)

	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") {
		return "第三方服务鉴权失败，请检查 API 密钥。"
	}
	if strings.Contains(lower, "validation_error") || strings.Contains(lower, "parameter validation failed") {
		return "第三方回程服务请求失败（参数校验），请稍后重试。"
	}
	if strings.Contains(lower, "third-party return failed") {
		return "第三方回程服务暂时不可用，系统已自动重试。"
	}
	if strings.Contains(msg, "已自动重试多种探测方式，线路仍未识别") {
		return "目标网络可能限制了路由可见性，已自动重试仍无法识别线路。"
	}
	if strings.Contains(msg, "已自动重试多探针与多地区，线路仍未识别") {
		return "已自动切换多个探针和地区，仍无法稳定识别线路。"
	}
	if strings.Contains(lower, "timeout") && strings.Contains(lower, "third-party") {
		return "第三方回程服务超时，系统已自动重试。"
	}
	if strings.Contains(lower, "no hop data") {
		if isReturn {
			return "未获取到可用的回程跳点数据。"
		}
		return "未获取到可用的去程跳点数据。"
	}

	if idx := strings.Index(msg, "\n{"); idx > 0 {
		msg = strings.TrimSpace(msg[:idx])
	}
	runes := []rune(msg)
	if len(runes) > 80 {
		msg = string(runes[:80]) + "..."
	}
	return msg
}

func collectPathText(hops []hopResult) string {
	var builder strings.Builder
	first := true

	appendPart := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if !first {
			builder.WriteByte(' ')
		}
		first = false
		builder.WriteString(strings.ToLower(s))
	}

	for _, hop := range hops {
		if hop.Timeout {
			continue
		}
		line := strings.TrimSpace(strings.ToLower(hop.LineName))
		if line == "private network" || line == "timeout" {
			continue
		}

		appendPart(hop.Host)
		appendPart(hop.IP)
		appendPart(hop.LineName)
	}

	return builder.String()
}

func pickLatency(tr traceResult) float64 {
	if tr.DestinationRTTMs > 0 {
		return tr.DestinationRTTMs
	}
	if tr.Ping != nil && tr.Ping.Received > 0 && tr.Ping.AvgMs > 0 {
		return tr.Ping.AvgMs
	}

	for i := len(tr.Hops) - 1; i >= 0; i-- {
		h := tr.Hops[i]
		if h.Timeout {
			continue
		}
		if h.RTTMs > 0 {
			return h.RTTMs
		}
	}
	return 0
}

func formatLatency(ms float64) string {
	if ms <= 0 {
		return "未知"
	}
	return fmt.Sprintf("%.2f ms", ms)
}

func resolveTargets(raw string) []string {
	targets := splitCSV(raw)
	if len(targets) > 0 {
		return targets
	}
	return splitCSV(defaultTargetsCSV)
}

func promptStringDefault(reader *bufio.Reader, label, def string) (string, error) {
	if def != "" {
		v, err := promptString(reader, fmt.Sprintf("%s [默认: %s]", label, def))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(v) == "" {
			return def, nil
		}
		return strings.TrimSpace(v), nil
	}

	v, err := promptString(reader, label)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v), nil
}

func promptIntDefault(reader *bufio.Reader, label string, def int, min int) (int, error) {
	for {
		v, err := promptString(reader, fmt.Sprintf("%s [默认: %d]", label, def))
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(v) == "" {
			return def, nil
		}

		n, nerr := strconv.Atoi(strings.TrimSpace(v))
		if nerr != nil || n < min {
			fmt.Printf("请输入不小于 %d 的整数。\n", min)
			continue
		}
		return n, nil
	}
}

func promptBoolDefault(reader *bufio.Reader, label string, def bool) (bool, error) {
	defText := "y"
	if !def {
		defText = "n"
	}

	for {
		v, err := promptString(reader, fmt.Sprintf("%s [y/n, 默认: %s]", label, defText))
		if err != nil {
			return false, err
		}
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "" {
			return def, nil
		}
		if s == "y" || s == "yes" || s == "1" || s == "true" {
			return true, nil
		}
		if s == "n" || s == "no" || s == "0" || s == "false" {
			return false, nil
		}
		fmt.Println("请输入 y 或 n。")
	}
}

func promptString(reader *bufio.Reader, label string) (string, error) {
	fmt.Printf("%s: ", label)
	line, err := reader.ReadString('\n')
	if err != nil {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			line = trimmed
		} else if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("检测到非交互输入 (EOF)，请在交互终端运行面板，或改用命令行参数模式")
		} else {
			return "", err
		}
	}

	text := strings.TrimSpace(line)
	if strings.EqualFold(text, "q") {
		return "", errors.New(userCancelText)
	}
	return text, nil
}

func handleToolError(reader *bufio.Reader, state *panelState, err error) {
	if isUserCancel(err) {
		state.LastErr = userCancelText
		return
	}
	state.LastErr = err.Error()
	showErrorAndWait(reader, err)
}

func showErrorAndWait(reader *bufio.Reader, err error) {
	if isUserCancel(err) {
		return
	}
	fmt.Printf("\n执行失败: %v\n", err)
	waitForEnter(reader)
}

func isUserCancel(err error) bool {
	return err != nil && err.Error() == userCancelText
}

func waitForEnter(reader *bufio.Reader) {
	fmt.Print("\n按 Enter 返回...")
	_, _ = reader.ReadString('\n')
}

func clearTerminal() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}
	fmt.Print("\033[2J\033[H")
}
