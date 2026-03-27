package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

type User struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	CreatedAt   string `json:"created_at"`
}

type Repo struct {
	Name            string `json:"name"`
	Language        string `json:"language"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	PushedAt        string `json:"pushed_at"`
	Size            int    `json:"size"`
}

type Event struct {
	Type string `json:"type"`
}

type LangStat struct {
	Name    string
	Count   int
	Color   string
	Percent float64
	X       float64
	Width   float64
}

type TemplateData struct {
	DisplayName   string
	Username      string
	PublicRepos   int
	ActiveChunks  int
	TotalStars    int
	TotalForks    int
	Followers     int
	Following     int
	DaysPlayed    int
	XPLevel       int
	TotalXP       int
	TotalSize     int
	MemPercent    int
	MemUsed       int
	MemTotal      int
	TopRepos      []Repo
	Languages     []LangStat
	BarWidth      float64
	Failed        bool
}

var langColors = map[string]string{
	"Go":         "#00add8",
	"JavaScript": "#f7df1e",
	"TypeScript": "#3178c6",
	"Java":       "#ed8b00",
	"C++":        "#00599c",
	"Python":     "#3572a5",
	"Rust":       "#dea584",
	"Ruby":       "#cc342d",
	"C":          "#555555",
	"Shell":      "#89e051",
	"HTML":       "#e34c26",
	"CSS":        "#563d7c",
	"Kotlin":     "#a97bff",
	"Swift":      "#f05138",
	"PHP":        "#4f5d95",
	"C#":         "#178600",
	"Dart":       "#00b4ab",
}

func apiGet(path string, target interface{}) error {
	username := os.Getenv("GITHUB_USER")
	if username == "" {
		username = "sanki92"
	}
	url := "https://api.github.com" + strings.Replace(path, "{username}", username, 1)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "vitals-generator")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}

func fetchData() TemplateData {
	var user User
	var repos []Repo
	var events []Event

	if err := apiGet("/users/{username}", &user); err != nil {
		return TemplateData{Failed: true}
	}
	if err := apiGet("/users/{username}/repos?sort=pushed&per_page=10&type=owner", &repos); err != nil {
		return TemplateData{Failed: true}
	}
	_ = apiGet("/users/{username}/events/public?per_page=100", &events)

	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}

	now := time.Now()
	created, _ := time.Parse(time.RFC3339, user.CreatedAt)
	daysPlayed := int(now.Sub(created).Hours() / 24)
	xpLevel := int(now.Sub(created).Hours()/24/365.25) * 10

	var totalStars, totalForks, totalSize, activeChunks int
	langCount := map[string]int{}

	for _, r := range repos {
		totalStars += r.StargazersCount
		totalForks += r.ForksCount
		totalSize += r.Size
		if r.Language != "" {
			langCount[r.Language]++
		}
		pushed, err := time.Parse(time.RFC3339, r.PushedAt)
		if err == nil && now.Sub(pushed).Hours() < 30*24 {
			activeChunks++
		}
	}

	topRepos := repos
	if len(topRepos) > 5 {
		topRepos = topRepos[:5]
	}

	var langs []LangStat
	totalLangs := 0
	for _, c := range langCount {
		totalLangs += c
	}
	for name, count := range langCount {
		color := langColors[name]
		if color == "" {
			color = "#808080"
		}
		langs = append(langs, LangStat{
			Name:    name,
			Count:   count,
			Color:   color,
			Percent: math.Round(float64(count) / float64(totalLangs) * 100),
		})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Count > langs[j].Count
	})
	if len(langs) > 6 {
		langs = langs[:6]
	}

	barWidth := 400.0
	x := 0.0
	for i := range langs {
		w := langs[i].Percent / 100 * barWidth
		langs[i].X = x
		langs[i].Width = w
		x += w
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memTotal := int(m.Sys / 1024)
	memUsed := int(m.Alloc / 1024)
	memPercent := 0
	if memTotal > 0 {
		memPercent = memUsed * 100 / memTotal
	}

	return TemplateData{
		DisplayName:  displayName,
		Username:     user.Login,
		PublicRepos:  user.PublicRepos,
		ActiveChunks: activeChunks,
		TotalStars:   totalStars,
		TotalForks:   totalForks,
		Followers:    user.Followers,
		Following:    user.Following,
		DaysPlayed:   daysPlayed,
		XPLevel:      xpLevel,
		TotalXP:      len(events),
		TotalSize:    totalSize,
		MemPercent:   memPercent,
		MemUsed:      memUsed,
		MemTotal:     memTotal,
		TopRepos:     topRepos,
		Languages:    langs,
		BarWidth:     barWidth,
	}
}

func textWidth(s string) int {
	return len(s)*8 + 4
}

var svgTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 860 480" width="860" height="480">
<defs>
<style>
@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
.blink { animation: blink 2s ease-in-out infinite; }
text { font-family: 'Courier New', monospace; font-size: 12px; }
</style>
</defs>
<rect width="860" height="480" fill="#1a1a1a"/>
{{if .Failed}}
{{template "line" dict "X" 300 "Y" 220 "Text" "Failed to load world" "Align" "left"}}
{{template "line" dict "X" 300 "Y" 240 "Text" "Check connection and try again" "Align" "left"}}
{{else}}
{{$ly := 20}}
{{template "line" dict "X" 10 "Y" 20 "Text" (printf "Minecraft 1.21.4 (github.com/%s)" .Username) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 36 "Text" (printf "%s -- Developer Profile" .DisplayName) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 68 "Text" "XYZ: 20.462 / 64.000 / -12.830" "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 84 "Text" "Block: 20 64 -12" "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 100 "Text" "Chunk: 1 4 -1 in 0 0" "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 116 "Text" "Facing: north (Towards distributed systems)" "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 148 "Text" (printf "Client Repos: %d" .PublicRepos) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 164 "Text" (printf "Active Chunks: %d of %d" .ActiveChunks .PublicRepos) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 180 "Text" (printf "Stars Collected: %d" .TotalStars) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 196 "Text" (printf "Forks Deployed: %d" .TotalForks) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 212 "Text" (printf "Allies (Followers): %d" .Followers) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 228 "Text" (printf "Following: %d" .Following) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 260 "Text" "Biome: minecraft:open_source_forest" "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 276 "Text" (printf "Days Played: %d" .DaysPlayed) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 292 "Text" (printf "XP Level: %d" .XPLevel) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 308 "Text" (printf "Total XP: %d" .TotalXP) "Align" "left" "Class" ""}}
{{template "line" dict "X" 10 "Y" 340 "Text" "Current Quest: Distributed Systems Mastery" "Align" "left" "Class" "blink"}}

{{template "rline" dict "X" 850 "Y" 20 "Text" "Java: 21.0.3 64bit" "Class" ""}}
{{template "rline" dict "X" 850 "Y" 36 "Text" (printf "Mem: %d%% %d/%d KB" .MemPercent .MemUsed .MemTotal) "Class" ""}}
{{template "rline" dict "X" 850 "Y" 52 "Text" (printf "Allocated: %d KB" .TotalSize) "Class" ""}}
{{template "rline" dict "X" 850 "Y" 84 "Text" "CPU: GitHub Actions Runner" "Class" ""}}
{{template "rline" dict "X" 850 "Y" 100 "Text" "Display: 1920x1080 (github.com)" "Class" ""}}
{{template "rline" dict "X" 850 "Y" 116 "Text" (printf "Server: github.com/%s" .Username) "Class" ""}}
{{template "rline" dict "X" 850 "Y" 132 "Text" "Protocol: REST API v3" "Class" ""}}
{{template "rline" dict "X" 850 "Y" 164 "Text" "Top Processes:" "Class" ""}}
{{range $i, $r := .TopRepos}}
{{template "rline" dict "X" 850 "Y" (add 180 (mul $i 16)) "Text" (printf " %s [%s]" (truncate $r.Name 24) (lang $r.Language)) "Class" ""}}
{{end}}
{{$afterRepos := add 180 (mul (len .TopRepos) 16)}}
{{template "rline" dict "X" 850 "Y" (add $afterRepos 16) "Text" "Targeted Block: main branch" "Class" ""}}
{{template "rline" dict "X" 850 "Y" (add $afterRepos 32) "Text" "Looking at: README.md" "Class" ""}}

{{if .Languages}}
<g transform="translate(230, 400)">
<rect x="-2" y="-2" width="{{add (int .BarWidth) 4}}" height="18" fill="#000000" opacity="0.6" rx="1" shape-rendering="crispEdges"/>
{{range .Languages}}
<rect x="{{.X}}" y="0" width="{{.Width}}" height="14" fill="{{.Color}}" shape-rendering="crispEdges"/>
{{end}}
</g>
<g transform="translate(230, 430)">
{{range $i, $l := .Languages}}
<rect x="{{mul $i 80}}" y="0" width="10" height="10" fill="{{$l.Color}}" shape-rendering="crispEdges"/>
<text x="{{add (mul $i 80) 14}}" y="9" fill="#3f3f3f" font-family="'Courier New', monospace" font-size="10">{{$l.Name}}</text>
<text x="{{add (mul $i 80) 13}}" y="8" fill="#ffffff" font-family="'Courier New', monospace" font-size="10">{{$l.Name}}</text>
{{end}}
</g>
{{end}}

{{end}}
</svg>
{{define "line"}}
<rect x="{{sub .X 2}}" y="{{sub .Y 12}}" width="{{textWidth .Text}}" height="14" fill="#000000" opacity="0.6" rx="1" shape-rendering="crispEdges"/>
<text x="{{add .X 1}}" y="{{.Y}}" fill="#3f3f3f" font-family="'Courier New', monospace" font-size="12">{{.Text}}</text>
<text x="{{.X}}" y="{{sub .Y 1}}" fill="#ffffff" font-family="'Courier New', monospace" font-size="12"{{if .Class}} class="{{.Class}}"{{end}}>{{.Text}}</text>
{{end}}
{{define "rline"}}
<rect x="{{sub .X (add (textWidth .Text) 2)}}" y="{{sub .Y 12}}" width="{{textWidth .Text}}" height="14" fill="#000000" opacity="0.6" rx="1" shape-rendering="crispEdges"/>
<text x="{{sub (add .X 1) (textWidth .Text)}}" y="{{.Y}}" fill="#3f3f3f" font-family="'Courier New', monospace" font-size="12" text-anchor="start">{{.Text}}</text>
<text x="{{sub .X (textWidth .Text)}}" y="{{sub .Y 1}}" fill="#ffffff" font-family="'Courier New', monospace" font-size="12" text-anchor="start"{{if .Class}} class="{{.Class}}"{{end}}>{{.Text}}</text>
{{end}}`

func main() {
	data := fetchData()

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"int": func(f float64) int { return int(f) },
		"textWidth": func(s string) int { return textWidth(s) },
		"truncate": func(s string, n int) string { return truncate(s, n) },
		"lang": func(s string) string {
			if s == "" {
				return "Unknown"
			}
			return s
		},
		"dict": func(pairs ...interface{}) map[string]interface{} {
			m := make(map[string]interface{})
			for i := 0; i+1 < len(pairs); i += 2 {
				m[pairs[i].(string)] = pairs[i+1]
			}
			return m
		},
	}

	tmpl, err := template.New("svg").Funcs(funcMap).Parse(svgTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "template parse error: %v\n", err)
		os.Exit(1)
	}

	if err := tmpl.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "template execute error: %v\n", err)
		os.Exit(1)
	}
}
