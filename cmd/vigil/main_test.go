package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This file is the wiring test. Every other test in this repository calls a
// function directly, and every one of them would stay green if main stopped
// starting a server, if the UI stopped being embedded, or if a flag were
// renamed. So this one builds the actual binary and runs it the way a user
// does: issue a token, start the server, ask it something, stop it.

func build(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the test drives the process with SIGTERM")
	}
	binary := filepath.Join(t.TempDir(), "vigil")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binary
}

var listening = regexp.MustCompile(`listening on (http://[^\s]+)`)

func TestTheBinaryServesWhatTheDocumentedCommandsPromise(t *testing.T) {
	binary := build(t)
	db := filepath.Join(t.TempDir(), "vigil.db")

	tokenOut, err := exec.Command(binary, "token", "create", "--db", db, "--name", "e2e").Output()
	if err != nil {
		t.Fatalf("token create: %v", err)
	}
	secret := strings.TrimSpace(string(tokenOut))
	if !strings.HasPrefix(secret, "vgl_") {
		t.Fatalf("token create printed %q", secret)
	}

	serve := exec.Command(binary, "serve", "--db", db, "--addr", "127.0.0.1:0")
	stdout, err := serve.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	serve.Stderr = &stderr
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		serve.Process.Kill()
		serve.Wait()
	})

	base := ""
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if m := listening.FindStringSubmatch(scanner.Text()); m != nil {
			base = m[1]
			break
		}
	}
	if base == "" {
		t.Fatalf("the server never announced an address.\nstderr: %s", stderr.String())
	}
	// Keep draining, or the child blocks on a full pipe once it logs enough.
	go io.Copy(io.Discard, stdout)

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	var health struct {
		Status       string `json:"status"`
		ActiveTokens int    `json:"active_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if health.Status != "ok" || health.ActiveTokens != 1 {
		t.Fatalf("health = %+v", health)
	}

	// The UI has to come out of the binary: no files next to it, no CDN.
	resp, err = client.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Contains(page, []byte("<title>vigil</title>")) {
		t.Fatalf("the embedded UI is not being served: %.120s", page)
	}

	// And the token the CLI printed has to be the one the API accepts. Those
	// are two halves that no unit test puts together.
	req, _ := http.NewRequest("POST", base+"/api/v1/ingest", strings.NewReader(`{
		"account": {"name": "Acme GmbH", "profile": "https://www.linkedin.com/company/acme/"},
		"signals": [{"kind": "job_change", "source": "linkedin",
		             "url": "https://www.linkedin.com/in/jane-doe/",
		             "occurred_at": "2026-09-15T08:00:00Z"}]
	}`))
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/ingest: %v", err)
	}
	var ingest struct {
		Inserted int `json:"inserted"`
	}
	json.NewDecoder(resp.Body).Decode(&ingest)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || ingest.Inserted != 1 {
		t.Fatalf("ingest: status %d, inserted %d", resp.StatusCode, ingest.Inserted)
	}
}

func TestTheBinaryRefusesAPublicBindWithNoToken(t *testing.T) {
	binary := build(t)
	db := filepath.Join(t.TempDir(), "vigil.db")

	out, err := exec.Command(binary, "serve", "--db", db, "--addr", "0.0.0.0:0").CombinedOutput()
	if err == nil {
		t.Fatal("the binary started on 0.0.0.0 with no token issued")
	}
	if !bytes.Contains(out, []byte("token create")) {
		t.Fatalf("the refusal does not say how to fix it: %s", out)
	}
}
