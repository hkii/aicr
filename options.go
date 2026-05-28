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
	corev1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/aicr/pkg/validator"
)

// Option configures a Client.
type Option func(*Client)

// ValidateOption configures a validation run launched via
// Client.ValidateState. It is a transparent alias of
// pkg/validator.Option, so any existing validator option function
// (e.g. validator.WithNoCluster) is also a ValidateOption — the
// facade-named With* functions below are the documented surface;
// reaching into pkg/validator directly is supported but not
// semver-protected.
type ValidateOption = validator.Option

// WithValidationNamespace sets the Kubernetes namespace where
// validation Jobs run. Default: "aicr-validation".
func WithValidationNamespace(namespace string) ValidateOption {
	return validator.WithNamespace(namespace)
}

// WithValidationRunID overrides the auto-generated identifier shared
// across the Jobs and resources produced by a single validation run.
// Use this to make repeated runs in the same namespace
// distinguishable (e.g., a controller's reconcile-key suffix).
func WithValidationRunID(runID string) ValidateOption {
	return validator.WithRunID(runID)
}

// WithValidationCleanup controls whether validator-emitted Jobs,
// ConfigMaps, and RBAC are deleted at the end of the run. Default:
// true. Set to false to leave artifacts behind for post-mortem
// inspection.
func WithValidationCleanup(cleanup bool) ValidateOption {
	return validator.WithCleanup(cleanup)
}

// WithValidationImagePullSecrets sets imagePullSecrets on the
// validator pods. Use this when the validator images live in a
// private registry whose credentials live in a Secret in the
// validation namespace.
func WithValidationImagePullSecrets(secrets []string) ValidateOption {
	return validator.WithImagePullSecrets(secrets)
}

// WithValidationNoCluster enables dry-run mode: no Kubernetes
// resources are created, all checks report as "skipped - no-cluster
// mode (test mode)". Constraints are still evaluated inline (they
// don't need cluster access). Use this for unit tests that exercise
// the facade surface without a live cluster.
func WithValidationNoCluster(noCluster bool) ValidateOption {
	return validator.WithNoCluster(noCluster)
}

// WithValidationTolerations passes tolerations through to the
// validation workload pods (e.g. NCCL benchmark pods). Does NOT
// affect the orchestrator Job itself, which runs with
// snapshotter.DefaultTolerations.
func WithValidationTolerations(tolerations []corev1.Toleration) ValidateOption {
	return validator.WithTolerations(tolerations)
}

// WithValidationNodeSelector passes a node selector through to the
// validation workload pods. Use when GPU nodes carry non-standard
// labels and the platform-default selector wouldn't match. Does NOT
// affect the orchestrator Job itself.
func WithValidationNodeSelector(nodeSelector map[string]string) ValidateOption {
	return validator.WithNodeSelector(nodeSelector)
}

// RecipeSourceOption identifies where recipes are sourced from.
type RecipeSourceOption struct {
	internal recipeSource
}

// WithRecipeSource sets the recipe source on the Client. Construct the
// argument with OCISource or FilesystemSource.
func WithRecipeSource(s RecipeSourceOption) Option {
	return func(c *Client) {
		c.source = s.internal
	}
}

// OCISource describes an OCI registry containing AICR recipes.
//
// The tag is optional; if empty, "latest" is assumed by the downstream
// loader.
func OCISource(registry, tag string) RecipeSourceOption {
	return RecipeSourceOption{
		internal: recipeSource{
			kind:     sourceKindOCI,
			registry: registry,
			tag:      tag,
		},
	}
}

// FilesystemSource describes a local filesystem path containing AICR
// recipes.
func FilesystemSource(path string) RecipeSourceOption {
	return RecipeSourceOption{
		internal: recipeSource{
			kind: sourceKindFilesystem,
			path: path,
		},
	}
}

// sourceKind is an unexported enum for recipe source variants.
type sourceKind int

const (
	sourceKindUnset sourceKind = iota
	sourceKindOCI
	sourceKindFilesystem
)

// recipeSource is the internal representation passed to the Client.
type recipeSource struct {
	kind     sourceKind
	registry string
	tag      string
	path     string
}
