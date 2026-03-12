package api

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ImportLumaRequest struct {
	URL string `json:"url"`
}

type ImportLumaResponse struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	DescriptionHTML string `json:"descriptionHtml,omitempty"`
	EventDate       string `json:"eventDate"`
	Location        string `json:"location"`
	OrganizerName   string `json:"organizerName"`
	EventImageURL   string `json:"eventImageUrl"`
	Warning         string `json:"warning,omitempty"`
}

func ImportLumaEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ImportLumaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	host := strings.ToLower(u.Host)
	if !strings.Contains(host, "luma.com") {
		http.Error(w, "only luma.com urls are supported", http.StatusBadRequest)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		http.Error(w, "unable to fetch event page", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "unable to access event page (if private, please set event to public)", http.StatusBadRequest)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read event page", http.StatusBadGateway)
		return
	}
	body := string(bodyBytes)

	out := parseLumaHTML(body)
	if strings.TrimSpace(out.Title) == "" {
		http.Error(w, "could not parse public event data (if private, please set event to public)", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func parseLumaHTML(body string) ImportLumaResponse {
	out := ImportLumaResponse{}
	descriptionScore := -1000
	descriptionHTMLScore := -1000
	for _, rawURL := range extractLumaImageURLs(body) {
		applyBestImageCandidate(&out.EventImageURL, rawURL)
	}
	grab := func(pattern string) string {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(body)
		if len(match) > 1 {
			return strings.TrimSpace(htmlUnescape(match[1]))
		}
		return ""
	}

	out.Title = grab(`(?is)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	if out.Title == "" {
		out.Title = grab(`(?is)<title[^>]*>([^<]+)</title>`)
	}
	out.Description = grab(`(?is)<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']+)["']`)
	applyBestImageCandidate(&out.EventImageURL, grab(`(?is)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`))
	applyBestImageCandidate(&out.EventImageURL, grab(`(?is)<meta[^>]+name=["']twitter:image["'][^>]+content=["']([^"']+)["']`))
	out.Location = grab(`(?is)<meta[^>]+property=["']event:location["'][^>]+content=["']([^"']+)["']`)

	// Parse JSON-LD blocks first.
	jsonLDRe := regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	for _, m := range jsonLDRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 2 {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &payload); err != nil {
			continue
		}
		extractFromGenericJSON(payload, &out, &descriptionScore, &descriptionHTMLScore)
	}

	// Parse Next.js payload for rich fields (full description + hosts).
	nextDataRe := regexp.MustCompile(`(?is)<script[^>]+id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)
	if m := nextDataRe.FindStringSubmatch(body); len(m) > 1 {
		var payload interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &payload); err == nil {
			extractFromGenericJSON(payload, &out, &descriptionScore, &descriptionHTMLScore)
		}
	}

	if out.EventDate == "" {
		dateGuess := grab(`(?is)<meta[^>]+property=["']event:start_time["'][^>]+content=["']([^"']+)["']`)
		out.EventDate = normalizeDate(dateGuess)
	}
	if out.OrganizerName == "" {
		out.OrganizerName = grab(`(?is)<meta[^>]+name=["']author["'][^>]+content=["']([^"']+)["']`)
	}
	if out.Title == "" {
		out.Warning = "Could not detect event metadata automatically; fill fields manually."
	}
	return out
}

func extractFromGenericJSON(node interface{}, out *ImportLumaResponse, descriptionScore *int, descriptionHTMLScore *int) {
	switch v := node.(type) {
	case map[string]interface{}:
		isHostProfile := looksLikeHostProfile(v)
		isEventNode := looksLikeEventNode(v)
		if isEventNode {
			if mirror, ok := v["description_mirror"]; ok {
				applyBestDescriptionHTMLCandidate(
					&out.DescriptionHTML,
					descriptionHTMLScore,
					renderLumaDescriptionMirrorHTML(mirror),
					"description_mirror",
				)
			}
		}
		// Prefer markdown/rich descriptions first, then plain summaries.
		for _, key := range []string{
			"event_description_md",
			"event_description",
			"description_md",
			"long_description_md",
			"long_description",
			"description",
			"about",
		} {
			if isHostProfile {
				break
			}
			if !isEventNode && !strings.HasPrefix(key, "event_") {
				continue
			}
			if val, ok := v[key].(string); ok {
				applyBestDescriptionCandidate(&out.Description, descriptionScore, val, key)
			}
		}
		for _, key := range []string{"start_at", "start_time", "startDate", "starts_at"} {
			if val, ok := v[key].(string); ok && out.EventDate == "" {
				out.EventDate = normalizeDate(val)
			}
		}
		for _, key := range []string{"location_name", "location", "venue"} {
			switch loc := v[key].(type) {
			case string:
				if strings.TrimSpace(out.Location) == "" {
					out.Location = strings.TrimSpace(loc)
				}
			case map[string]interface{}:
				if name, ok := loc["name"].(string); ok && strings.TrimSpace(out.Location) == "" {
					out.Location = strings.TrimSpace(name)
				}
			}
		}
		for _, key := range []string{
			"cover_url",
			"cover_image_url",
			"event_cover_url",
			"social_image_url",
			"share_image_url",
			"image",
			"image_url",
			"thumbnail_url",
		} {
			if img, ok := v[key].(string); ok {
				applyBestImageCandidate(&out.EventImageURL, img)
			}
		}
		// Handle hosts array explicitly.
		if hosts, ok := v["hosts"].([]interface{}); ok && out.OrganizerName == "" {
			names := make([]string, 0, len(hosts))
			for _, h := range hosts {
				if hm, ok := h.(map[string]interface{}); ok {
					if name, ok := hm["name"].(string); ok && strings.TrimSpace(name) != "" {
						names = append(names, strings.TrimSpace(name))
					}
				}
			}
			if len(names) > 0 {
				out.OrganizerName = strings.Join(names, ", ")
			}
		}
		if org, ok := v["organizer"].(map[string]interface{}); ok && out.OrganizerName == "" {
			if name, ok := org["name"].(string); ok {
				out.OrganizerName = strings.TrimSpace(name)
			}
		}
		for _, child := range v {
			extractFromGenericJSON(child, out, descriptionScore, descriptionHTMLScore)
		}
	case []interface{}:
		for _, child := range v {
			extractFromGenericJSON(child, out, descriptionScore, descriptionHTMLScore)
		}
	}
}

func normalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Keep explicit timezone if already present.
	if strings.Contains(raw, "T") && (strings.Contains(raw, "+") || strings.HasSuffix(raw, "Z")) {
		return raw
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return raw
}

func htmlUnescape(s string) string {
	return strings.ReplaceAll(html.UnescapeString(s), "\u00a0", " ")
}

func applyBestDescriptionCandidate(current *string, bestScore *int, candidate string, key string) {
	candidate = normalizeImportedDescription(candidate)
	if candidate == "" {
		return
	}
	score := descriptionCandidateScore(candidate, key)
	if score > *bestScore {
		*current = candidate
		*bestScore = score
	}
}

func applyBestDescriptionHTMLCandidate(current *string, bestScore *int, candidate string, key string) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return
	}
	score := descriptionCandidateScore(candidate, key)
	if score > *bestScore {
		*current = candidate
		*bestScore = score
	}
}

func descriptionCandidateScore(value string, key string) int {
	if strings.TrimSpace(value) == "" {
		return -1000
	}
	score := len(strings.TrimSpace(value))
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "description_mirror":
		score += 12000
	case "event_jsonld_description":
		score += 10000
	case "description_md", "long_description_md":
		score += 3000
	case "long_description":
		score += 2000
	case "description":
		score += 1000
	case "about":
		score += 500
	case "bio_short":
		score += 100
	}
	if strings.Contains(value, "\n- ") || strings.Contains(value, "\n• ") {
		score += 400
	}
	if strings.Contains(value, "\t") {
		score += 100
	}
	return score
}

func renderLumaDescriptionMirrorHTML(node interface{}) string {
	switch v := node.(type) {
	case map[string]interface{}:
		nodeType, _ := v["type"].(string)
		content, _ := v["content"].([]interface{})
		switch nodeType {
		case "doc":
			return renderMirrorChildren(content)
		case "paragraph":
			return "<p>" + renderMirrorChildren(content) + "</p>"
		case "heading":
			level := 2
			if attrs, ok := v["attrs"].(map[string]interface{}); ok {
				if n, ok := attrs["level"].(float64); ok && n >= 1 && n <= 6 {
					level = int(n)
				}
			}
			tag := "h" + strconv.Itoa(level)
			return "<" + tag + ">" + renderMirrorChildren(content) + "</" + tag + ">"
		case "bullet_list":
			return "<ul>" + renderMirrorChildren(content) + "</ul>"
		case "ordered_list":
			return "<ol>" + renderMirrorChildren(content) + "</ol>"
		case "list_item":
			return "<li>" + renderMirrorChildren(content) + "</li>"
		case "hard_break":
			return "<br/>"
		case "image":
			if attrs, ok := v["attrs"].(map[string]interface{}); ok {
				src, _ := attrs["src"].(string)
				alt, _ := attrs["alt"].(string)
				if strings.TrimSpace(src) != "" {
					return `<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) + `"/>`
				}
			}
			return ""
		case "text":
			text, _ := v["text"].(string)
			if text == "" {
				return ""
			}
			out := html.EscapeString(text)
			marks, _ := v["marks"].([]interface{})
			for _, rawMark := range marks {
				mark, ok := rawMark.(map[string]interface{})
				if !ok {
					continue
				}
				markType, _ := mark["type"].(string)
				switch markType {
				case "bold":
					out = "<strong>" + out + "</strong>"
				case "italic":
					out = "<em>" + out + "</em>"
				case "underline":
					out = "<u>" + out + "</u>"
				case "strike":
					out = "<s>" + out + "</s>"
				case "code":
					out = "<code>" + out + "</code>"
				case "link":
					href := ""
					if attrs, ok := mark["attrs"].(map[string]interface{}); ok {
						if h, ok := attrs["href"].(string); ok {
							href = h
						}
					}
					if strings.TrimSpace(href) != "" {
						out = `<a href="` + html.EscapeString(href) + `" target="_blank" rel="noreferrer">` + out + "</a>"
					}
				}
			}
			return out
		default:
			return renderMirrorChildren(content)
		}
	case []interface{}:
		return renderMirrorChildren(v)
	default:
		return ""
	}
}

func renderMirrorChildren(children []interface{}) string {
	if len(children) == 0 {
		return ""
	}
	var b strings.Builder
	for _, child := range children {
		b.WriteString(renderLumaDescriptionMirrorHTML(child))
	}
	return b.String()
}

func normalizeImportedDescription(raw string) string {
	s := strings.TrimSpace(htmlUnescape(raw))
	if s == "" {
		return ""
	}
	// Handle escaped newlines/tabs from embedded JSON text.
	s = strings.ReplaceAll(s, `\r\n`, "\n")
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")

	// Preserve simple structure when the source is HTML.
	htmlPatternReplacements := []struct {
		pattern string
		to      string
	}{
		{`(?i)<br\s*/?>`, "\n"},
		{`(?i)</p>`, "\n\n"},
		{`(?i)</div>`, "\n"},
		{`(?i)<li>`, "- "},
		{`(?i)</li>`, "\n"},
		{`(?i)&nbsp;|&#160;|&#xA0;`, " "},
		{`(?i)&bullet;`, "• "},
	}
	for _, repl := range htmlPatternReplacements {
		s = regexp.MustCompile(repl.pattern).ReplaceAllString(s, repl.to)
	}
	// Remove remaining HTML tags.
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)
	s = tagRe.ReplaceAllString(s, "")
	// Collapse excessive blank lines while preserving readable spacing.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func applyBestImageCandidate(current *string, candidate string) {
	candidate = normalizeLumaImageCandidate(candidate)
	if candidate == "" {
		return
	}
	if imageCandidateScore(candidate) > imageCandidateScore(strings.TrimSpace(*current)) {
		*current = candidate
	}
}

func imageCandidateScore(url string) int {
	if url == "" {
		return -1000
	}
	low := strings.ToLower(url)
	score := 0

	// Prefer event cover/square image variants.
	for _, key := range []string{
		"gallery-images",
		"width=300",
		"height=300",
		"fit=cover",
		"event-cover",
		"event_cover",
		"event-covers",
		"cover",
		"square",
		"share",
		"social",
	} {
		if strings.Contains(low, key) {
			score += 20
		}
	}
	// Penalize non-event assets.
	for _, key := range []string{
		"avatar",
		"avatars",
		"profile",
		"user",
		"host",
		"speaker",
		"logo",
	} {
		if strings.Contains(low, key) {
			score -= 25
		}
	}
	if strings.Contains(low, "lumacdn.com/event-covers") {
		score += 40
	}
	if strings.Contains(low, "lumacdn.com/gallery-images") {
		score += 120
	}
	if strings.Contains(low, "width=300") && strings.Contains(low, "height=300") {
		score += 140
	}
	if strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") || strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".webp") {
		score += 5
	}
	return score
}

func extractLumaImageURLs(body string) []string {
	re := regexp.MustCompile(`https://images\.lumacdn\.com[^"'\\\s<>]+`)
	matches := re.FindAllString(body, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, raw := range matches {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeLumaImageCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if !strings.Contains(strings.ToLower(parsed.Host), "images.lumacdn.com") {
		return raw
	}
	path := parsed.Path
	if strings.Contains(path, "/gallery-images/") {
		path = path[strings.Index(path, "/gallery-images/"):]
		return "https://images.lumacdn.com/cdn-cgi/image/format=auto,fit=cover,dpr=2,background=white,quality=75,width=300,height=300" + path
	}
	return raw
}

func looksLikeHostProfile(v map[string]interface{}) bool {
	hostSignals := 0
	for _, key := range []string{
		"api_id",
		"username",
		"avatar_url",
		"bio_short",
		"twitter_handle",
		"instagram_handle",
		"linkedin_handle",
		"youtube_handle",
		"tiktok_handle",
		"last_online_at",
	} {
		if _, ok := v[key]; ok {
			hostSignals++
		}
	}
	return hostSignals >= 3
}

func looksLikeEventNode(v map[string]interface{}) bool {
	if t, ok := v["@type"].(string); ok && strings.EqualFold(strings.TrimSpace(t), "Event") {
		return true
	}
	eventSignals := 0
	for _, key := range []string{
		"start_at",
		"start_time",
		"startDate",
		"starts_at",
		"endDate",
		"event_api_id",
		"event_description",
		"event_description_md",
		"cover_url",
		"event_cover_url",
		"location_name",
		"ticket_types",
		"calendar_api_id",
	} {
		if _, ok := v[key]; ok {
			eventSignals++
		}
	}
	return eventSignals >= 2
}
