package profile

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	infiniteTrafficThreshold = int64(1<<63 - 1)
	infiniteTimeThreshold    = int64(92233720368)
)

// RemoteProfile is the parsed result of downloading a subscription URL.
type RemoteProfile struct {
	Name           string
	URL            string
	Usage          Usage
	UpdateInterval int64
	Content        string
}

// FetchRemote downloads a subscription URL and parses the Hiddify subscription
// format: base64 body whose leading "#" comments carry the profile metadata.
func FetchRemote(ctx context.Context, rawURL string) (RemoteProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return RemoteProfile{}, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return RemoteProfile{}, fmt.Errorf("download subscription: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RemoteProfile{}, fmt.Errorf("download subscription: status %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return RemoteProfile{}, err
	}

	decoded := safeDecodeBase64(string(body))
	headers := parseHeaders(decoded)
	result := RemoteProfile{URL: rawURL, Content: strings.TrimSpace(decoded)}

	result.Name = headerName(headers, rawURL)
	if interval := headers.Get("Profile-Update-Interval"); interval != "" {
		if duration, err := time.ParseDuration(interval + "h"); err == nil {
			result.UpdateInterval = duration.Milliseconds()
		}
	}
	if userinfo := headers.Get("Subscription-Userinfo"); userinfo != "" {
		result.Usage = parseSubscriptionInfo(userinfo)
	}
	if result.Usage.Total == 0 {
		result.Usage.Total = infiniteTrafficThreshold
	}
	if result.Usage.Expire == 0 {
		result.Usage.Expire = infiniteTimeThreshold
	}
	result.Usage.WebPageURL = headers.Get("Profile-Web-Page-Url")
	result.Usage.SupportURL = headers.Get("Support-Url")
	return result, nil
}

func parseHeaders(content string) http.Header {
	headers := make(http.Header)
	lines := strings.SplitN(content, "\n", 30)
	for i := 0; i < len(lines)-1; i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
			continue
		}
		index := strings.Index(line, ":")
		if index == -1 || len(line) <= index+1 || line[index+1] == '/' {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(line[:index], "#"), "//")))
		value := strings.TrimSpace(line[index+1:])
		if value != "" {
			headers.Set(key, value)
		}
	}
	return headers
}

func headerName(headers http.Header, rawURL string) string {
	if title := headers.Get("Profile-Title"); title != "" {
		if strings.HasPrefix(title, "base64:") {
			if decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(title, "base64:")); err == nil {
				return string(decoded)
			}
		}
		return strings.TrimSpace(title)
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		if parsed.Fragment != "" {
			return parsed.Fragment
		}
		parts := strings.Split(parsed.Path, "/")
		last := parts[len(parts)-1]
		if last != "" {
			return regexp.MustCompile(`\.(json|yaml|yml|txt).*`).ReplaceAllString(last, "")
		}
		return parsed.Host
	}
	return "Remote Profile"
}

func parseSubscriptionInfo(info string) Usage {
	usage := Usage{}
	for _, field := range strings.Split(info, ";") {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(parts[0])) {
		case "upload":
			usage.Upload = value
		case "download":
			usage.Download = value
		case "total":
			usage.Total = value
		case "expire":
			usage.Expire = value
		}
	}
	return usage
}

func safeDecodeBase64(content string) string {
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return content
	}
	return string(decoded)
}
