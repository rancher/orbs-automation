package ociregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// manifestTypes are what a manifest request must accept. A multi-architecture
// image is an index, and registries serve it only to a client that says so.
const manifestTypes = "application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json"

// challengeParam picks key="value" pairs out of a WWW-Authenticate header.
var challengeParam = regexp.MustCompile(`(\w+)="([^"]*)"`)

// Status is the outcome of looking for one image.
type Status string

// The outcomes. NotFound means the registry answered and has no such image,
// which is a release that was not published. Unreachable means nothing is known:
// the registry could not be asked, or answered with something else.
const (
	Found       Status = "found"
	NotFound    Status = "not found"
	Unreachable Status = "unreachable"
)

// Result is a Status and, when it is Unreachable, why.
type Result struct {
	Status Status
	Detail string
}

// Checker asks registries whether an image exists, as docker manifest inspect
// does but without Docker. The zero value works: it uses https and a client
// with a timeout.
type Checker struct {
	Client *http.Client // nil means a client with a 20 second timeout
	Scheme string       // "" means https; tests use http
}

// Exists reports whether image:tag is in the registry at host. It asks for the
// manifest with HEAD, and on a bearer challenge it fetches an anonymous token
// from the realm the registry names and asks once more. It never fails: a
// problem reaching the registry is an Unreachable result with the reason.
func (c Checker) Exists(ctx context.Context, host, image, tag string) Result {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	scheme := c.Scheme
	if scheme == "" {
		scheme = "https"
	}
	manifest := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, host, image, url.PathEscape(tag))

	resp, err := head(ctx, client, manifest, "")
	if err != nil {
		return Result{Status: Unreachable, Detail: err.Error()}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		_ = resp.Body.Close()
		token, err := anonymousToken(ctx, client, challenge)
		if err != nil {
			return Result{Status: Unreachable, Detail: err.Error()}
		}
		if resp, err = head(ctx, client, manifest, token); err != nil {
			return Result{Status: Unreachable, Detail: err.Error()}
		}
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return Result{Status: Found}
	case http.StatusNotFound:
		return Result{Status: NotFound}
	}
	return Result{Status: Unreachable, Detail: fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))}
}

func head(ctx context.Context, client *http.Client, target, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", manifestTypes)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return client.Do(req)
}

// anonymousToken follows a bearer challenge to the realm it names and returns
// the token the realm hands out without credentials. A challenge that is not a
// bearer one means the registry wants a login, which is not anonymous.
func anonymousToken(ctx context.Context, client *http.Client, challenge string) (string, error) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(challenge), "Bearer ")
	if !ok {
		return "", errors.New("authentication required, and the registry offers no anonymous token")
	}
	params := map[string]string{}
	for _, m := range challengeParam.FindAllStringSubmatch(rest, -1) {
		params[m[1]] = m[2]
	}
	realm, err := url.Parse(params["realm"])
	if err != nil || params["realm"] == "" {
		return "", errors.New("the registry's token challenge names no realm")
	}
	q := realm.Query()
	if v := params["service"]; v != "" {
		q.Set("service", v)
	}
	if v := params["scope"]; v != "" {
		q.Set("scope", v)
	}
	realm.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realm.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch an anonymous token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch an anonymous token: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("read the anonymous token: %w", err)
	}
	if body.Token != "" {
		return body.Token, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, nil
	}
	return "", errors.New("the token realm returned no token")
}
