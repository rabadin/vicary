package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func buildDockerImage(t *testing.T) string {
	imageName := fmt.Sprintf("%s:latest", randSeq(10))
	cmd := exec.Command("docker", "build", "-t", imageName, ".")
	cmd.Dir = cmd.Dir + getGitDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(out)
	}
	t.Cleanup(func() { deleteDockerImage(imageName) })
	return imageName
}

func runVicary(t *testing.T, image string) string {
	port, err := getFreePort()
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("docker", "run", "-d", "-p", fmt.Sprintf("%d:%d", port, port),
		"-e", fmt.Sprintf("VICARY_PORT=%d", port),
		"-e", "VICARY_SCHEME=http", "-e", "VICARY_STORE=/tmp", "-e", "VICARY_RESOLVER=1.1.1.1",
		image)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(out)
	}
	ret := strings.TrimSpace(string(out))
	t.Cleanup(func() { deleteDockerContainer(ret) })
	return fmt.Sprintf("http://localhost:%d", port)
}

func getGitDir() string {
	pwd, err := os.Getwd()
	if err != nil {
		panic("Failed to get working directory")
	}
	for len(pwd) > 1 {
		fp := path.Join(pwd, ".git")
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			pwd = path.Dir(pwd)
			continue
		}
		return pwd
	}
	panic("Cannot find git directory")
}

func deleteDockerContainer(runId string) error {
	return exec.Command("docker", "rm", "-f", runId).Run()
}

func deleteDockerImage(imageName string) error {
	return exec.Command("docker", "image", "rm", imageName).Run()
}

func initTestVicary(t *testing.T) string {
	image := buildDockerImage(t)
	return runVicary(t, image)
}

func waitUntilUp(url string, code int, timeout time.Duration) {
	start := time.Now()
	for {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == code {
			break
		}
		time.Sleep(1 * time.Second)
		elapsed := time.Since(start)
		if elapsed > timeout {
			panic(fmt.Sprintf("Timed out waiting for service at %v to start", url))
		}
	}
}

type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	IssuedAt    string `json:"issued_at"`
}

type ManifestResponse struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	Tag           string `json:"tag"`
	Architecture  string `json:"architecture"`
}

func TestVicary(t *testing.T) {
	hostUrl := initTestVicary(t)

	// Wait until service comes up.
	waitUntilUp(fmt.Sprintf("%s/health/", hostUrl), 200, 5*time.Second)

	// Test basic endpoints.
	table := []struct {
		uri               string
		expected_content  string
		expected_ret_code int
	}{
		{"/health/", "OK", 200},
		{"/unknown/", "", 404},
		{"/v2/", "", 401},
		{"/token/", "{\"token\":\"bogus\"}", 200},
	}
	for _, tc := range table {
		tc := tc
		t.Run(fmt.Sprintf("Endpoint %s", tc.uri), func(t *testing.T) {
			t.Parallel()
			resp, err := http.Get(fmt.Sprintf("%s%s", hostUrl, tc.uri))
			assert.Nil(t, err)
			if tc.expected_ret_code != 0 {
				assert.Equal(t, tc.expected_ret_code, resp.StatusCode, "Incorrect return code")
			}
			if tc.expected_content != "" {
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				assert.Nil(t, err)
				assert.Equal(t, tc.expected_content, string(body), "Incorrect body")
			}
		})
	}

	// Test full interactions.
	inputs := []struct {
		uri     string
		scope   string
		service string
		tag     string
		sha256  string
	}{
		{"library/python", "library/python", "registry.docker.io", "3.10", "6f9f74896dfa93fe0172f594faba85e0b4e8a0481a0fefd9112efc7e4d3c78f7"},
		{"docker.io/library/python", "library/python", "registry.docker.io", "3.10", "6f9f74896dfa93fe0172f594faba85e0b4e8a0481a0fefd9112efc7e4d3c78f7"},
		{"quay.io/jitesoft/debian", "jitesoft/debian", "", "10", "210d19e01db0473d65bbcb90d2df57b2d168433f4df4b8a6c176a776733e0cc2"},
		{"gcr.io/google-containers/busybox", "google-containers/busybox", "", "1.27", "aab39f0bc16d3c109d7017bcbc13ee053b9b1b1c6985c432ec9b5dde1eb0d066"},
	}
	for _, tc := range inputs {
		tc := tc
		t.Run(fmt.Sprintf("Full interaction %s", tc.uri), func(t *testing.T) {
			t.Parallel()
			var tokenResp TokenResponse
			if tc.service != "" {
				// Get token.
				tokenUrl := fmt.Sprintf("%s/token?scope=repository%%3A%s%%3Apull&service=%s", hostUrl, url.QueryEscape(tc.scope), tc.service)
				resp, err := http.Get(tokenUrl)
				assert.Nil(t, err)
				assert.Equal(t, 200, resp.StatusCode)
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				assert.Nil(t, err)
				err = json.Unmarshal(body, &tokenResp)
				assert.Nil(t, err)
				assert.Greater(t, len(tokenResp.Token), 10)
				assert.Greater(t, len(tokenResp.AccessToken), 10)
			}

			// Get manifest.
			manifestUrl := fmt.Sprintf("%s/v2/%s/manifests/%s", hostUrl, tc.uri, tc.tag)
			client := &http.Client{}
			req, err := http.NewRequest("GET", manifestUrl, nil)
			assert.Nil(t, err)
			if tc.service != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.Token))
			}
			resp, err := client.Do(req)
			assert.Nil(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			assert.Nil(t, err)
			var manifestResp ManifestResponse
			err = json.Unmarshal(body, &manifestResp)
			assert.Nil(t, err)
			assert.Equal(t, 1, manifestResp.SchemaVersion)

			// Get layer.
			layerUrl := fmt.Sprintf("%s/v2/%s/blobs/sha256:%s", hostUrl, tc.uri, tc.sha256)
			req, err = http.NewRequest("HEAD", layerUrl, nil)
			assert.Nil(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			if tc.service != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.Token))
			}
			resp, err = client.Do(req)
			assert.Nil(t, err)
			assert.Contains(t, []string{"application/octet-stream", "binary/octet-stream"}, resp.Header.Get("content-type"))
			contentLength, err := strconv.Atoi(resp.Header.Get("content-length"))
			assert.Nil(t, err)
			assert.Greater(t, contentLength, 100)
		})
	}
}
