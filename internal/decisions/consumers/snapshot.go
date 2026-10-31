package consumers

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/decisions"
)

type ImageSupport string

const (
	ImagesSupported   ImageSupport = "supported"
	ImagesUnsupported ImageSupport = "unsupported"
	ImagesUnknown     ImageSupport = "unknown"
)

type Capabilities struct {
	Images   ImageSupport `json:"images"`
	External bool         `json:"external"`
	CPUOnly  bool         `json:"cpu_only"`
}

// Layer codes; service codes come from internal/decisions.
const (
	CodeAvailable                  = "available"
	CodeConsumerDisabled           = "consumer_disabled"
	CodeImagesUnknown              = "images_unknown"
	CodeImagesExternalNotPermitted = "images_external_not_permitted"
	CodeAdmissionDeferred          = "admission_deferred"
	CodePreempted                  = "preempted"
	CodeBudgetBlocked              = "budget_blocked"
	CodeConfigurationChanged       = "configuration_changed"
)

// Snapshot is passive. Evaluate never starts or installs a runtime and is
// pinned to the configuration generation that produced the snapshot.
type Snapshot struct {
	Consumer      ID           `json:"consumer"`
	Available     bool         `json:"available"`
	Code          string       `json:"code"`
	Backend       string       `json:"backend"`
	Provider      string       `json:"provider,omitempty"`
	Model         string       `json:"model,omitempty"`
	Caps          Capabilities `json:"capabilities"`
	ImagesAllowed bool         `json:"images_allowed"`

	Identity [32]byte                                                             `json:"-"`
	Evaluate func(context.Context, decisions.Request) (decisions.Response, error) `json:"-"`
	Current  func() bool                                                          `json:"-"`
}

type Service interface {
	Snapshot(cfg *config.Config, id ID) Snapshot
}
