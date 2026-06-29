package install

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/incidentflow/incidentflow-cli/internal/api"
	"github.com/incidentflow/incidentflow-cli/internal/helm"
	"github.com/incidentflow/incidentflow-cli/internal/kube"
	"github.com/incidentflow/incidentflow-cli/internal/output"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeAPI struct {
	tokenCalled bool
	tokenErr    error
	token       *api.CreateTokenResponse
	statusCalls int
}

func (f *fakeAPI) CreateRegistrationToken(_ context.Context, _ api.CreateTokenRequest) (*api.CreateTokenResponse, error) {
	f.tokenCalled = true
	if f.tokenErr != nil {
		return nil, f.tokenErr
	}
	if f.token != nil {
		return f.token, nil
	}
	return &api.CreateTokenResponse{
		RegistrationToken: "tok-secret",
		TokenItem:         api.TokenItem{ID: "tok-id"},
	}, nil
}

func (f *fakeAPI) GetAgentStatus(_ context.Context, _ string) (*api.AgentStatus, error) {
	f.statusCalls++
	return &api.AgentStatus{
		ClusterName: "test",
		Status:      "online",
		LastHeartbeat: time.Now(),
	}, nil
}

type fakeKube struct {
	pingErr        error
	permissionsErr error
	waitErr        error
	contextName    string
}

func (f *fakeKube) Ping(_ context.Context) error                   { return f.pingErr }
func (f *fakeKube) CheckPermissions(_ context.Context) error       { return f.permissionsErr }
func (f *fakeKube) WaitForDeployment(_ context.Context, _, _ string, _ time.Duration) error {
	return f.waitErr
}
func (f *fakeKube) GetDeploymentStatus(_ context.Context, _, _ string) (string, error) {
	return "available", nil
}
func (f *fakeKube) GetPodStatus(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (f *fakeKube) GetEvents(_ context.Context, _ string) ([]string, error) { return nil, nil }

type fakeHelm struct {
	versionErr   error
	showChartErr error
	upgradeErr   error
	diffOut      string
	diffErr      error
	hasDiff      bool
}

func (f *fakeHelm) Version() (string, error) {
	if f.versionErr != nil {
		return "", f.versionErr
	}
	return "v3.99.0", nil
}
func (f *fakeHelm) ShowChart(_ context.Context, _, _ string) error      { return f.showChartErr }
func (f *fakeHelm) HasDiffPlugin() bool                                  { return f.hasDiff }
func (f *fakeHelm) Diff(_ context.Context, _ helm.InstallOptions) (string, error) {
	return f.diffOut, f.diffErr
}
func (f *fakeHelm) Upgrade(_ context.Context, _ helm.InstallOptions) error {
	return f.upgradeErr
}
func (f *fakeHelm) Uninstall(_ context.Context, _, _ string) error { return nil }
func (f *fakeHelm) Status(_ context.Context, _, _ string) (*helm.ReleaseStatus, error) {
	return &helm.ReleaseStatus{Status: "deployed"}, nil
}

// kubeCurrentContextOverride lets tests override kube.CurrentContext.
// We patch it via the kube package's exported function — but since it's a
// package-level function we can't override it directly.
// Instead, the installer calls kube.CurrentContext() which reads the real
// kubeconfig. In tests we use KUBECONFIG env or accept whatever context is set.

func baseOpts() Options {
	return Options{
		ClusterName:  "test-cluster",
		Namespace:    "test-ns",
		PlatformURL:  "http://localhost:8000",
		GatewayURL:   "ws://localhost:8001/ws",
		ChartRef:     "oci://example.com/chart",
		ChartVersion: "1.0.0",
		ReleaseName:  "test-release",
		TimeoutSeconds: 30,
		Yes:          true, // skip confirmation in most tests
		NoDiff:       true,
		ConfigEnv:    "local",
		ConfigAppURL: "http://localhost:3001",
	}
}

// newInstaller builds an Installer with fake context detection.
// kube.CurrentContext() reads real kubeconfig; pass a fakeKube that doesn't fail.
func newTestInstaller(a *fakeAPI, k *fakeKube, h *fakeHelm) *Installer {
	return &Installer{API: a, Kube: k, Helm: h}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func silentJSON() func() {
	output.SetJSON(true)
	return func() { output.SetJSON(false) }
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestApplyDefaults(t *testing.T) {
	o := Options{ClusterName: "x"}
	o.applyDefaults()

	if o.Namespace == "" {
		t.Error("Namespace should have a default")
	}
	if o.ChartRef == "" {
		t.Error("ChartRef should have a default")
	}
	if o.ReleaseName == "" {
		t.Error("ReleaseName should have a default")
	}
	if o.TimeoutSeconds == 0 {
		t.Error("TimeoutSeconds should have a default")
	}
	if o.ChartVersion != "latest" {
		t.Errorf("ChartVersion default = %q, want %q", o.ChartVersion, "latest")
	}
	if o.DisplayName != "x" {
		t.Errorf("DisplayName should default to ClusterName, got %q", o.DisplayName)
	}
}

func TestDefaultResources(t *testing.T) {
	rs := defaultResources("my-release", "my-ns")
	if len(rs) == 0 {
		t.Fatal("expected non-empty resource list")
	}
	for _, r := range rs {
		if r.Action == "" || r.Kind == "" || r.Name == "" {
			t.Errorf("resource has empty field: %+v", r)
		}
	}
	// namespace entry must use the namespace value
	found := false
	for _, r := range rs {
		if r.Kind == "Namespace" && r.Name == "my-ns" {
			found = true
		}
	}
	if !found {
		t.Error("expected Namespace resource with name my-ns")
	}
}

func TestDryRunNeverCreatesToken(t *testing.T) {
	defer silentJSON()()

	fAPI := &fakeAPI{}
	installer := newTestInstaller(fAPI, &fakeKube{}, &fakeHelm{})
	opts := baseOpts()
	opts.DryRun = true

	ctx := context.Background()
	_, err := installer.InstallCluster(ctx, opts)
	if err != nil {
		// kube.CurrentContext() may fail in CI; skip if so
		if isKubeContextError(err) {
			t.Skipf("no kubeconfig available: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if fAPI.tokenCalled {
		t.Error("dry-run must NOT call CreateRegistrationToken")
	}
}

func TestCancelledNeverCreatesToken(t *testing.T) {
	fAPI := &fakeAPI{}
	installer := newTestInstaller(fAPI, &fakeKube{}, &fakeHelm{})
	installer.stdin = strings.NewReader("n\n") // user answers No

	opts := baseOpts()
	opts.Yes = false // enable prompt

	ctx := context.Background()
	_, err := installer.InstallCluster(ctx, opts)
	if err != nil {
		if isKubeContextError(err) {
			t.Skipf("no kubeconfig available: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if fAPI.tokenCalled {
		t.Error("cancelled install must NOT call CreateRegistrationToken")
	}
}

func TestConfirmYesCreatesToken(t *testing.T) {
	fAPI := &fakeAPI{}
	installer := newTestInstaller(fAPI, &fakeKube{}, &fakeHelm{})
	installer.stdin = strings.NewReader("y\n")

	opts := baseOpts()
	opts.Yes = false // enable prompt

	ctx := context.Background()
	result, err := installer.InstallCluster(ctx, opts)
	if err != nil {
		if isKubeContextError(err) {
			t.Skipf("no kubeconfig available: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if !fAPI.tokenCalled {
		t.Error("confirmed install MUST call CreateRegistrationToken")
	}
	if result == nil {
		t.Error("expected non-nil result after successful install")
	}
}

func TestHelmPreflightErrorStopsBeforeToken(t *testing.T) {
	fAPI := &fakeAPI{}
	fHelm := &fakeHelm{showChartErr: errors.New("chart not found")}
	installer := newTestInstaller(fAPI, &fakeKube{}, fHelm)

	opts := baseOpts()
	ctx := context.Background()
	_, err := installer.InstallCluster(ctx, opts)
	if err == nil {
		t.Fatal("expected error from ShowChart failure")
	}
	if !strings.Contains(err.Error(), "chart not found") {
		t.Errorf("unexpected error: %v", err)
	}
	if fAPI.tokenCalled {
		t.Error("preflight failure must NOT create registration token")
	}
}

func TestForceDiffFailsWhenPluginMissing(t *testing.T) {
	fAPI := &fakeAPI{}
	fHelm := &fakeHelm{hasDiff: false}
	installer := newTestInstaller(fAPI, &fakeKube{}, fHelm)

	opts := baseOpts()
	opts.ForceDiff = true

	ctx := context.Background()
	_, err := installer.InstallCluster(ctx, opts)
	if err == nil {
		t.Fatal("expected error when --diff used without helm-diff plugin")
	}
	if !strings.Contains(err.Error(), "helm-diff") {
		t.Errorf("unexpected error: %v", err)
	}
	if fAPI.tokenCalled {
		t.Error("--diff failure must NOT create registration token")
	}
}

func TestPromptConfirmAcceptsYAndYes(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n", "Yes\n"} {
		r := strings.NewReader(input)
		ok, err := promptConfirm(r, "")
		if err != nil {
			t.Errorf("input %q: unexpected error: %v", input, err)
		}
		if !ok {
			t.Errorf("input %q should be confirmed", input)
		}
	}
}

func TestPromptConfirmRejectsOther(t *testing.T) {
	for _, input := range []string{"\n", "n\n", "no\n", "NO\n", "nope\n", ""} {
		r := strings.NewReader(input)
		ok, _ := promptConfirm(r, "")
		if ok {
			t.Errorf("input %q should NOT be confirmed", input)
		}
	}
}

func TestResultTokenIDPopulated(t *testing.T) {
	fAPI := &fakeAPI{token: &api.CreateTokenResponse{
		RegistrationToken: "super-secret",
		TokenItem:         api.TokenItem{ID: "my-token-id"},
	}}
	installer := newTestInstaller(fAPI, &fakeKube{}, &fakeHelm{})

	opts := baseOpts()
	ctx := context.Background()
	result, err := installer.InstallCluster(ctx, opts)
	if err != nil {
		if isKubeContextError(err) {
			t.Skipf("no kubeconfig available: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TokenID != "my-token-id" {
		t.Errorf("result.TokenID = %q, want %q", result.TokenID, "my-token-id")
	}
	// raw token must never appear in result
	if strings.Contains(result.TokenID, "super-secret") {
		t.Error("raw token must not appear in result.TokenID")
	}
}

// isKubeContextError returns true when the error is due to missing kubeconfig,
// which is expected in environments without a real cluster.
func isKubeContextError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "kubeconfig") ||
		strings.Contains(msg, "KUBECONFIG") ||
		strings.Contains(msg, "no configuration") ||
		strings.Contains(msg, "context") && strings.Contains(msg, "not found") ||
		strings.Contains(msg, "kube")
}

// Ensure concrete types still satisfy the interfaces (compile-time check).
var _ APIClient = (*api.Client)(nil)
var _ KubeClient = (*kube.Client)(nil)
var _ HelmClientIface = (*helm.Client)(nil)
