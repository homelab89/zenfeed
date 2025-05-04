package crawl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/pkg/errors"
	"github.com/temoto/robotstxt"

	textconvert "github.com/glidea/zenfeed/pkg/util/text_convert"
)

var httpClient = &http.Client{}

func Markdown(ctx context.Context, u string) (string, error) {
	// Check if the page is allowed.
	if err := checkAllowed(ctx, u); err != nil {
		return "", errors.Wrapf(err, "check robots.txt for %s", u)
	}

	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", errors.Wrapf(err, "create request for %s", u)
	}
	req.Header.Set("User-Agent", userAgent)

	// Send the request.
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "fetch %s", u)
	}
	defer resp.Body.Close()

	// Parse the response.
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("received non-200 status code %d from %s", resp.StatusCode, u)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "read body from %s", u)
	}

	// Convert the body to markdown.
	mdBytes, err := textconvert.HTMLToMarkdown(bodyBytes)
	if err != nil {
		return "", errors.Wrap(err, "convert html to markdown")
	}

	return string(mdBytes), nil
}

const userAgent = "ZenFeed"

func checkAllowed(ctx context.Context, u string) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return errors.Wrapf(err, "parse url %s", u)
	}

	d, err := getRobotsData(ctx, parsedURL.Host)
	if err != nil {
		return errors.Wrapf(err, "check robots.txt for %s", parsedURL.Host)
	}
	if !d.TestAgent(parsedURL.Path, userAgent) {
		return errors.Errorf("disallowed by robots.txt for %s", u)
	}

	return nil
}

var robotsDataCache sync.Map

// getRobotsData fetches and parses robots.txt for a given host.
func getRobotsData(ctx context.Context, host string) (*robotstxt.RobotsData, error) {
	// Check the cache.
	if data, found := robotsDataCache.Load(host); found {
		return data.(*robotstxt.RobotsData), nil
	}

	// Prepare the request.
	robotsURL := fmt.Sprintf("https://%s/robots.txt", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "create request for %s", robotsURL)
	}
	req.Header.Set("User-Agent", userAgent)

	// Send the request.
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "fetch %s", robotsURL)
	}
	defer resp.Body.Close()

	// Parse the response.
	switch resp.StatusCode {
	case http.StatusOK:
		data, err := robotstxt.FromResponse(resp)
		if err != nil {
			return nil, errors.Wrapf(err, "parse robots.txt from %s", robotsURL)
		}
		robotsDataCache.Store(host, data)
		return data, nil
	case http.StatusNotFound:
		data := &robotstxt.RobotsData{}
		robotsDataCache.Store(host, data)
		return data, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, errors.Errorf("access to %s denied (status %d)", robotsURL, resp.StatusCode)
	default:
		return nil, errors.Errorf("unexpected status %d fetching %s", resp.StatusCode, robotsURL)
	}
}
