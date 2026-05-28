// Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aicr

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aicrerrors "github.com/NVIDIA/aicr/pkg/errors"
	"github.com/NVIDIA/aicr/pkg/recipe"
)

// newRecipeResultForBundleTest builds a facade RecipeResult with its
// unexported internal field populated, side-stepping the requirement
// that callers obtain RecipeResults via ResolveRecipe. This is
// internal-only because the internal field is unexported on purpose;
// the production contract only allows the facade itself to set it.
//
// owner stamps the unexported owner pointer that BundleComponents /
// ValidateState check against. Pass the same *Client the test will
// invoke BundleComponents on so the cross-client guard accepts the
// result; pass a different (or nil) *Client to deliberately exercise
// the rejection path.
func newRecipeResultForBundleTest(owner *Client, refs []recipe.ComponentRef, facadeComponents []ComponentRef) *RecipeResult {
	internal := &recipe.RecipeResult{
		Kind:          "RecipeResult",
		APIVersion:    "v1",
		ComponentRefs: refs,
	}
	return &RecipeResult{
		Name:       "test",
		Components: facadeComponents,
		internal:   internal,
		owner:      owner,
	}
}

// newClientForBundleTest builds a Client whose builder is non-nil so
// BundleComponents passes the closed-Client guard. Only the closed-
// Client check looks at builder; the bundling path itself doesn't,
// so a placeholder Builder is enough.
//
// dp binds to the embedded recipe FS so the per-Client DataProvider
// snapshot in BundleComponents has a non-nil provider for values +
// manifest reads. Without an explicit dp, those reads fall back to
// recipe.GetDataProvider() — the package-global singleton — which
// races against any other test that touches it under -race.
func newClientForBundleTest(t *testing.T) *Client {
	t.Helper()
	return &Client{
		builder: recipe.NewBuilder(),
		dp:      recipe.NewEmbeddedDataProvider(recipe.GetEmbeddedFS(), "."),
	}
}

// TestBundleComponents_RejectsUnknownKind locks in the change from
// silent empty bundle (pre-fix) to a clear ErrCodeInvalidRequest
// (post-fix) when a recipe's component carries a Kind that doesn't
// normalise to "helm" or "kustomize" — typo bait at the recipe-emit
// boundary.
func TestBundleComponents_RejectsUnknownKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
	}{
		{"empty kind", ""},
		{"typo kind", "Hlem"},
		{"trailing space", "Helm "},
		{"truncation", "kustom"},
	}

	client := newClientForBundleTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRecipeResultForBundleTest(client,
				[]recipe.ComponentRef{{Name: "c1"}},
				[]ComponentRef{{Name: "c1", Kind: tt.kind}},
			)
			_, err := client.BundleComponents(context.Background(), r)
			if err == nil {
				t.Fatalf("expected error for unknown kind %q, got nil", tt.kind)
			}
			var se *aicrerrors.StructuredError
			if !stderrors.As(err, &se) {
				t.Fatalf("expected *aicrerrors.StructuredError, got %T: %v", err, err)
			}
			if se.Code != aicrerrors.ErrCodeInvalidRequest {
				t.Errorf("expected ErrCodeInvalidRequest, got %s", se.Code)
			}
		})
	}
}

// TestBundleComponents_AcceptsLowercasedKind locks in the case-
// insensitive normalisation. provider-nvidia's emit code already
// accepts both forms; the AICR contract should match. A pure
// lowercase "helm" Kind with no values must produce a successful
// bundle (HelmValues nil, no error).
func TestBundleComponents_AcceptsLowercasedKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
	}{
		{"canonical Helm", "Helm"},
		{"lowercased helm", "helm"},
		{"mixed-case Helm", "HELM"},
	}

	client := newClientForBundleTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRecipeResultForBundleTest(client,
				// No ValuesFile, no Overrides → GetValuesForComponent
				// returns an empty map and never touches the global
				// DataProvider. Keeps the test hermetic.
				[]recipe.ComponentRef{{Name: "c1", Type: recipe.ComponentTypeHelm}},
				[]ComponentRef{{Name: "c1", Kind: tt.kind}},
			)
			bundles, err := client.BundleComponents(context.Background(), r)
			if err != nil {
				t.Fatalf("unexpected error for kind %q: %v", tt.kind, err)
			}
			if len(bundles) != 1 {
				t.Fatalf("expected 1 bundle, got %d", len(bundles))
			}
			if bundles[0].HelmValues != nil {
				t.Errorf("expected nil HelmValues for empty values map, got %q",
					bundles[0].HelmValues)
			}
		})
	}
}

// TestBundleComponents_HelmComponentLoadsManifestFiles locks in the
// "Helm components carry supplemental manifests too" contract. Recipes
// like h100-gke-cos-training and the base gpu-operator overlay attach
// extra raw manifests to a Helm component (gke-nccl-tcpxo installer +
// nri-device-injector; gpu-operator's dcgm-exporter overlay). Pre-fix
// the switch in BundleComponents only loaded ManifestFiles for the
// "Kustomize" branch, so these supplemental resources fell on the
// floor — bundle.Manifests was nil and the deployer had no way to
// know they should be applied.
//
// Post-fix: a Helm component with non-empty ManifestFiles produces a
// bundle whose Manifests is the multi-doc concatenation of those
// files (and HelmValues remains populated independently). A Helm
// component WITHOUT ManifestFiles still produces Manifests == nil so
// the existing one-Release-per-component path is unchanged.
//
// The test reads from the embedded recipe FS (the same
// components/gpu-operator/manifests/dcgm-exporter.yaml the existing
// TestGetManifestContent uses), keeping the hermetic-fixture style
// consistent with the rest of this file.
func TestBundleComponents_HelmComponentLoadsManifestFiles(t *testing.T) {
	t.Parallel()

	const manifestPath = "components/gpu-operator/manifests/dcgm-exporter.yaml"

	tests := []struct {
		name           string
		manifestFiles  []string
		wantNilOutput  bool
		mustContainAll []string // substrings expected in the joined manifest blob
	}{
		{
			name:          "Helm component with no manifestFiles → Manifests nil",
			manifestFiles: nil,
			wantNilOutput: true,
		},
		{
			name:           "Helm component with one manifestFile → Manifests populated",
			manifestFiles:  []string{manifestPath},
			wantNilOutput:  false,
			mustContainAll: []string{"apiVersion", "kind"},
		},
		{
			name:           "Helm component with multiple manifestFiles → multi-doc joined with ---",
			manifestFiles:  []string{manifestPath, manifestPath},
			wantNilOutput:  false,
			mustContainAll: []string{"\n---\n"},
		},
	}

	client := newClientForBundleTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRecipeResultForBundleTest(client,
				[]recipe.ComponentRef{{
					Name:          "gpu-operator",
					Type:          recipe.ComponentTypeHelm,
					ManifestFiles: tt.manifestFiles,
				}},
				[]ComponentRef{{Name: "gpu-operator", Kind: "Helm"}},
			)
			bundles, err := client.BundleComponents(context.Background(), r)
			if err != nil {
				t.Fatalf("BundleComponents: %v", err)
			}
			if len(bundles) != 1 {
				t.Fatalf("expected 1 bundle, got %d", len(bundles))
			}
			got := bundles[0]
			if tt.wantNilOutput {
				if got.Manifests != nil {
					t.Errorf("expected nil Manifests for Helm component without manifestFiles, got %d bytes",
						len(got.Manifests))
				}
				return
			}
			if len(got.Manifests) == 0 {
				t.Fatalf("expected non-empty Manifests for Helm component with manifestFiles, got nil/empty")
			}
			for _, sub := range tt.mustContainAll {
				if !strings.Contains(string(got.Manifests), sub) {
					t.Errorf("Manifests missing expected substring %q; full content (%d bytes):\n%s",
						sub, len(got.Manifests), got.Manifests)
				}
			}
		})
	}
}

// TestRecipeResultFromInternal_PlumbsHelmFields locks in that the
// translation from pkg/recipe.ComponentRef into the facade's
// ComponentRef carries Source, Chart, and Namespace through. Without
// these, downstream consumers (provider-nvidia) can't build a
// usable Helm Release — chart.repository, chart.name, and
// forProvider.namespace all come from this triplet.
func TestRecipeResultFromInternal_PlumbsHelmFields(t *testing.T) {
	t.Parallel()

	internal := &recipe.RecipeResult{
		Kind:       "RecipeResult",
		APIVersion: "v1",
		Criteria:   &recipe.Criteria{},
		ComponentRefs: []recipe.ComponentRef{
			{
				Name:      "nfd",
				Type:      recipe.ComponentTypeHelm,
				Version:   "0.15.5",
				Source:    "https://kubernetes-sigs.github.io/node-feature-discovery/charts",
				Chart:     "node-feature-discovery",
				Namespace: "node-feature-discovery",
			},
			{
				Name:      "gpu-operator",
				Type:      recipe.ComponentTypeHelm,
				Version:   "v25.10.0",
				Source:    "https://helm.ngc.nvidia.com/nvidia",
				Chart:     "gpu-operator",
				Namespace: "gpu-operator",
			},
		},
	}

	out, err := recipeResultFromInternal(internal)
	if err != nil {
		t.Fatalf("recipeResultFromInternal: %v", err)
	}
	if len(out.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(out.Components))
	}

	for i, want := range internal.ComponentRefs {
		got := out.Components[i]
		if got.Source != want.Source {
			t.Errorf("Components[%d].Source = %q, want %q", i, got.Source, want.Source)
		}
		if got.Chart != want.Chart {
			t.Errorf("Components[%d].Chart = %q, want %q", i, got.Chart, want.Chart)
		}
		if got.Namespace != want.Namespace {
			t.Errorf("Components[%d].Namespace = %q, want %q", i, got.Namespace, want.Namespace)
		}
	}
}

// TestBundleComponents_PerClientDataProviderIsolation pins the gap
// closed in v0.2: BundleComponents must read values AND manifest
// files via the per-Client DataProvider rather than the
// process-global recipe.GetDataProvider() singleton.
//
// Pre-fix, two Clients pointing at distinct FilesystemSource
// directories shared the package-global DataProvider for values and
// manifest reads. The metadata-store + component-registry caches
// were already per-Client, but values + manifests would silently
// resolve against whichever Client most recently caused a global-
// provider read. This test would fail under that model: clientA's
// bundle could surface clientB's "cluster-id" marker (or vice
// versa).
//
// The setup uses the unexported `internal` field so the test can
// pin a hand-built RecipeResult to each Client without dropping a
// fully-resolvable recipe set into each tempdir — that surface is
// covered by e2e. The contract under test is narrow: given two
// real Clients with different DataProviders, BundleComponents must
// route values + manifest reads through each Client's own provider.
func TestBundleComponents_PerClientDataProviderIsolation(t *testing.T) {
	t.Parallel()

	// Per-Client filesystem sources. Each carries:
	//   1. registry.yaml at the root — required by the layered
	//      provider for any external data source.
	//   2. components/regtest/values.yaml — the values file the
	//      regression test resolves through the per-Client DP.
	//   3. components/regtest/manifests/extra.yaml — the manifest
	//      file the regression test resolves through the per-Client
	//      DP.
	// The "cluster-id" key in values.yaml and the metadata.name in
	// the manifest both encode the source's identity so we can tell
	// which Client's source actually got read.
	type fixture struct {
		dir    string
		marker string // unique cluster-id token
		client *Client
		recipe *RecipeResult
	}
	build := func(t *testing.T, marker string) *fixture {
		t.Helper()
		dir := t.TempDir()
		// Layered provider scans the external dir at construction
		// time; every file we want it to surface must exist BEFORE
		// NewClient is called.
		if err := os.WriteFile(filepath.Join(dir, "registry.yaml"),
			[]byte("components: []\n"), 0o600); err != nil {
			t.Fatalf("write registry.yaml in %s: %v", dir, err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "components", "regtest", "manifests"), 0o755); err != nil {
			t.Fatalf("mkdir components/regtest in %s: %v", dir, err)
		}
		valuesContent := "cluster-id: " + marker + "\n"
		if err := os.WriteFile(
			filepath.Join(dir, "components", "regtest", "values.yaml"),
			[]byte(valuesContent), 0o600); err != nil {
			t.Fatalf("write values.yaml in %s: %v", dir, err)
		}
		manifestContent := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + marker + "\n"
		if err := os.WriteFile(
			filepath.Join(dir, "components", "regtest", "manifests", "extra.yaml"),
			[]byte(manifestContent), 0o600); err != nil {
			t.Fatalf("write extra.yaml in %s: %v", dir, err)
		}

		c, err := NewClient(WithRecipeSource(FilesystemSource(dir)))
		if err != nil {
			t.Fatalf("NewClient(%s): %v", dir, err)
		}
		t.Cleanup(func() { _ = c.Close() })

		// Hand-built RecipeResult with internal populated so
		// BundleComponents bypasses ResolveRecipe (which would
		// require a fully-resolvable recipe set in each tempdir).
		// The ValuesFile path equals the base values.yaml path,
		// triggering the simple base-load branch in
		// GetValuesForComponentWithProvider rather than the
		// overlay-merge branch.
		r := newRecipeResultForBundleTest(c,
			[]recipe.ComponentRef{{
				Name:          "regtest",
				Type:          recipe.ComponentTypeHelm,
				ValuesFile:    "components/regtest/values.yaml",
				ManifestFiles: []string{"components/regtest/manifests/extra.yaml"},
			}},
			[]ComponentRef{{Name: "regtest", Kind: "Helm"}},
		)
		return &fixture{dir: dir, marker: marker, client: c, recipe: r}
	}

	a := build(t, "client-a")
	b := build(t, "client-b")

	// Bundle through each Client. The crucial assertion is that
	// each bundle's HelmValues + Manifests carry the marker from
	// THAT Client's source, not the other's. Bundle B FIRST so the
	// metadata-store / registry caches for the package-global DP
	// (which the legacy code paths fell back to) are unlikely to
	// hold A's data — the original bug surfaced precisely when the
	// global flipped between Clients across a metadata-store
	// repopulate, so flipping the order at least makes the test
	// fail under the legacy implementation in addition to the
	// per-Client one.
	bundlesB, err := b.client.BundleComponents(context.Background(), b.recipe)
	if err != nil {
		t.Fatalf("BundleComponents B: %v", err)
	}
	bundlesA, err := a.client.BundleComponents(context.Background(), a.recipe)
	if err != nil {
		t.Fatalf("BundleComponents A: %v", err)
	}

	checkBundle := func(t *testing.T, label string, bundles []ComponentBundle, wantMarker, otherMarker string) {
		t.Helper()
		if len(bundles) != 1 {
			t.Fatalf("%s: expected 1 bundle, got %d", label, len(bundles))
		}
		got := bundles[0]
		hv := string(got.HelmValues)
		if !strings.Contains(hv, wantMarker) {
			t.Errorf("%s: HelmValues missing own marker %q; got %q", label, wantMarker, hv)
		}
		if strings.Contains(hv, otherMarker) {
			t.Errorf("%s: HelmValues contains OTHER client's marker %q (cross-contamination); got %q",
				label, otherMarker, hv)
		}
		mf := string(got.Manifests)
		if !strings.Contains(mf, wantMarker) {
			t.Errorf("%s: Manifests missing own marker %q; got %q", label, wantMarker, mf)
		}
		if strings.Contains(mf, otherMarker) {
			t.Errorf("%s: Manifests contains OTHER client's marker %q (cross-contamination); got %q",
				label, otherMarker, mf)
		}
	}

	checkBundle(t, "client-a", bundlesA, "client-a", "client-b")
	checkBundle(t, "client-b", bundlesB, "client-b", "client-a")
}

// TestCollectSnapshot_RejectsNilClient locks in the nil-receiver
// guard that mirrors ResolveRecipe / BundleComponents. Calling on
// a nil Client must return ErrCodeInvalidRequest, not panic.
func TestCollectSnapshot_RejectsNilClient(t *testing.T) {
	t.Parallel()

	var c *Client
	_, err := c.CollectSnapshot(context.Background(), &AgentConfig{Namespace: "x"})
	if err == nil {
		t.Fatalf("expected error from nil Client, got nil")
	}
	var se *aicrerrors.StructuredError
	if !stderrors.As(err, &se) {
		t.Fatalf("expected *aicrerrors.StructuredError, got %T: %v", err, err)
	}
	if se.Code != aicrerrors.ErrCodeInvalidRequest {
		t.Errorf("expected ErrCodeInvalidRequest, got %s", se.Code)
	}
}

// TestCollectSnapshot_RejectsNilConfig locks in that an explicit
// nil AgentConfig surfaces as ErrCodeInvalidRequest at the facade
// before any K8s deployment is attempted. The underlying snapshotter
// rejects nil too, but doing it at the facade keeps the error code
// consistent across paths.
func TestCollectSnapshot_RejectsNilConfig(t *testing.T) {
	t.Parallel()

	c := newClientForBundleTest(t)
	_, err := c.CollectSnapshot(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error from nil config, got nil")
	}
	var se *aicrerrors.StructuredError
	if !stderrors.As(err, &se) {
		t.Fatalf("expected *aicrerrors.StructuredError, got %T: %v", err, err)
	}
	if se.Code != aicrerrors.ErrCodeInvalidRequest {
		t.Errorf("expected ErrCodeInvalidRequest, got %s", se.Code)
	}
}

// TestCollectSnapshot_RejectsClosedClient locks in the closed-Client
// guard. After Close() clears the builder, CollectSnapshot must
// surface that as ErrCodeInvalidRequest rather than calling through
// to snapshotter.DeployAndGetSnapshot with stale state.
func TestCollectSnapshot_RejectsClosedClient(t *testing.T) {
	t.Parallel()

	c := newClientForBundleTest(t)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := c.CollectSnapshot(context.Background(), &AgentConfig{Namespace: "x"})
	if err == nil {
		t.Fatalf("expected error from closed Client, got nil")
	}
	if got != nil {
		t.Errorf("expected nil Snapshot on error, got %v", got)
	}
	var se *aicrerrors.StructuredError
	if !stderrors.As(err, &se) {
		t.Fatalf("expected *aicrerrors.StructuredError, got %T: %v", err, err)
	}
	if se.Code != aicrerrors.ErrCodeInvalidRequest {
		t.Errorf("expected ErrCodeInvalidRequest, got %s", se.Code)
	}
}

// TestValidateState_RejectsBadInput locks in every facade-side guard
// that runs before the validator is even constructed. Each row
// triggers a different path (nil client, nil recipe, recipe missing
// internal state, nil snapshot) and asserts the same outer error
// code so callers can rely on a uniform branch.
func TestValidateState_RejectsBadInput(t *testing.T) {
	t.Parallel()

	validClient := newClientForBundleTest(t)
	validRecipe := newRecipeResultForBundleTest(validClient,
		[]recipe.ComponentRef{{Name: "c1", Type: recipe.ComponentTypeHelm}},
		[]ComponentRef{{Name: "c1", Kind: "Helm"}},
	)
	validSnap := &Snapshot{}

	tests := []struct {
		name   string
		client *Client
		recipe *RecipeResult
		snap   *Snapshot
	}{
		{"nil client", nil, validRecipe, validSnap},
		{"nil recipe", validClient, nil, validSnap},
		{"recipe missing internal", validClient, &RecipeResult{Name: "no-internal"}, validSnap},
		{"nil snapshot", validClient, validRecipe, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := tt.client.ValidateState(context.Background(), tt.recipe, tt.snap)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var se *aicrerrors.StructuredError
			if !stderrors.As(err, &se) {
				t.Fatalf("expected *aicrerrors.StructuredError, got %T: %v", err, err)
			}
			if se.Code != aicrerrors.ErrCodeInvalidRequest {
				t.Errorf("expected ErrCodeInvalidRequest, got %s", se.Code)
			}
		})
	}
}

// TestValidateState_RejectsClosedClient locks in the closed-Client
// guard. After Close() clears the builder, ValidateState must surface
// that as ErrCodeInvalidRequest rather than constructing a Validator
// from a half-torn-down Client.
func TestValidateState_RejectsClosedClient(t *testing.T) {
	t.Parallel()

	c := newClientForBundleTest(t)
	r := newRecipeResultForBundleTest(c,
		[]recipe.ComponentRef{{Name: "c1", Type: recipe.ComponentTypeHelm}},
		[]ComponentRef{{Name: "c1", Kind: "Helm"}},
	)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := c.ValidateState(context.Background(), r, &Snapshot{})
	if err == nil {
		t.Fatalf("expected error from closed Client, got nil")
	}
	if got != nil {
		t.Errorf("expected nil PhaseResult slice on error, got %v", got)
	}
	var se *aicrerrors.StructuredError
	if !stderrors.As(err, &se) {
		t.Fatalf("expected *aicrerrors.StructuredError, got %T: %v", err, err)
	}
	if se.Code != aicrerrors.ErrCodeInvalidRequest {
		t.Errorf("expected ErrCodeInvalidRequest, got %s", se.Code)
	}
}
