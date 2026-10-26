package rustfs

import (
	"context"
	"encoding/json"
	"fmt"

	"net/http"
	"net/url"

	"aurago/internal/config"
	"github.com/minio/minio-go/v7"
)

type serviceAccountInfo struct {
	Parent      string `json:"parentUser"`
	Status      string `json:"accountStatus"`
	Implied     bool   `json:"impliedPolicy"`
	Policy      string `json:"policy"`
	Description string `json:"description"`
}

func (a *adminClient) mutation(ctx context.Context, method, name string, q url.Values, body, result any) error {
	code, err := a.native(ctx, method, name, q, body, result)
	if err != nil && (code == 0 || code >= 500 || code >= 200 && code < 300) {
		return failure("outcome_unknown", err)
	}
	return err
}
func (m *Manager) rootAdmin(s persistedState) (*adminClient, error) {
	return newAdminClient(s.Endpoint, s.RootAccess, s.RootSecret, m.http)
}
func (m *Manager) verifyChildCredentials(ctx context.Context, s persistedState) error {
	child, err := newAdminClient(s.Endpoint, s.ChildAccess, s.ChildSecret, m.http)
	if err != nil {
		return err
	}
	if _, err = child.native(ctx, http.MethodGet, "account/info", nil, nil, nil); err != nil {
		return failure("recovery_required", err)
	}
	return nil
}
func (m *Manager) disableChild(ctx context.Context, s persistedState) error {
	admin, err := m.rootAdmin(s)
	if err != nil {
		return err
	}
	var info serviceAccountInfo
	if _, err = admin.native(ctx, http.MethodGet, "info-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, &info); err != nil {
		return err
	}
	if info.Parent != s.ParentAccess || info.Description != "AuraGo managed S3 "+s.Instance || info.Implied {
		return failure("recovery_required", nil)
	}
	switch info.Status {
	case "off", "disabled":
		return nil
	case "on", "enabled":
		if err = m.verifyChildCredentials(ctx, s); err != nil {
			return err
		}
	default:
		return failure("credential_revocation_failed", nil)
	}
	if err = admin.mutation(ctx, http.MethodPost, "update-service-account", url.Values{"accessKey": {s.ChildAccess}}, map[string]any{"newStatus": "off"}, nil); err != nil {
		return err
	}
	if _, err = admin.native(ctx, http.MethodGet, "info-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, &info); err != nil {
		return err
	}
	if info.Parent != s.ParentAccess || info.Description != "AuraGo managed S3 "+s.Instance || info.Implied {
		return failure("recovery_required", nil)
	}
	if info.Status != "off" && info.Status != "disabled" {
		return failure("credential_revocation_failed", nil)
	}
	return nil
}
func (m *Manager) reconcileIAM(ctx context.Context, cfg *config.Config, s *persistedState) error {
	if s.Provisioned {
		if err := m.inspectEstablishedIAM(ctx, cfg, *s, true); err != nil {
			return err
		}
	}
	admin, err := m.rootAdmin(*s)
	if err != nil {
		return err
	}
	root, err := newDataClient(s.Endpoint, s.RootAccess, s.RootSecret, m.http)
	if err != nil {
		return err
	}
	exists, err := root.BucketExists(ctx, cfg.RustFS.Bucket)
	if err != nil {
		return failure("bucket_probe_failed", err)
	}
	if !exists {
		if err = root.MakeBucket(ctx, cfg.RustFS.Bucket, minio.MakeBucketOptions{Region: Region}); err != nil {
			return failure("outcome_unknown", err)
		}
	}
	policy := generatePolicy(effectivePermissions(cfg))
	// Disable existing child authority before changing either parent or embedded policy.
	var child serviceAccountInfo
	code, err := admin.native(ctx, http.MethodGet, "info-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, &child)
	if code != http.StatusNotFound && err != nil {
		return err
	}
	childExists := code == http.StatusOK
	if childExists {
		if child.Parent != s.ParentAccess || child.Description != "AuraGo managed S3 "+s.Instance || child.Implied {
			return failure("recovery_required", nil)
		}
	}
	desiredPolicyHash := policyHash(policy)
	if !s.IAMPending {
		s.IAMPending = true
		s.PendingPolicyHash = desiredPolicyHash
		if err = m.saveState(*s); err != nil {
			return err
		}
	}
	if childExists && child.Status != "off" && child.Status != "disabled" {
		if err = m.disableChild(ctx, *s); err != nil {
			return err
		}
	}
	if err = admin.mutation(ctx, http.MethodPut, "add-canned-policy", url.Values{"name": {s.PolicyName}}, json.RawMessage(policy), nil); err != nil {
		return err
	}
	var parent struct {
		Policy string `json:"policyName"`
	}
	code, err = admin.native(ctx, http.MethodGet, "user-info", url.Values{"accessKey": {s.ParentAccess}}, nil, &parent)
	if code != http.StatusNotFound && err != nil {
		return err
	}
	if code == http.StatusNotFound {
		if err = admin.mutation(ctx, http.MethodPut, "add-user", url.Values{"accessKey": {s.ParentAccess}}, map[string]any{"secretKey": s.ParentSecret, "status": "enabled"}, nil); err != nil {
			return err
		}
	} else {
		proof, proofErr := newAdminClient(s.Endpoint, s.ParentAccess, s.ParentSecret, m.http)
		if proofErr != nil {
			return proofErr
		}
		if _, proofErr = proof.native(ctx, http.MethodGet, "account/info", nil, nil, nil); proofErr != nil {
			return failure("recovery_required", nil)
		}
	}
	if err = admin.mutation(ctx, http.MethodPut, "set-user-or-group-policy", url.Values{"policyName": {s.PolicyName}, "userOrGroup": {s.ParentAccess}, "isGroup": {"false"}}, nil, nil); err != nil {
		return err
	}
	if !childExists {
		var created struct {
			Credentials struct {
				Access string `json:"accessKey"`
				Secret string `json:"secretKey"`
			} `json:"credentials"`
		}
		if err = admin.mutation(ctx, http.MethodPut, "add-service-account", nil, map[string]any{"targetUser": s.ParentAccess, "accessKey": s.ChildAccess, "secretKey": s.ChildSecret, "policy": json.RawMessage(policy), "name": "AuraGoS3", "description": "AuraGo managed S3 " + s.Instance}, &created); err != nil {
			return err
		}
		if created.Credentials.Access != s.ChildAccess || created.Credentials.Secret != s.ChildSecret {
			return failure("recovery_required", nil)
		}
	}
	status := "off"
	if hasDataPermissions(effectivePermissions(cfg)) {
		status = "on"
	}
	if err = admin.mutation(ctx, http.MethodPost, "update-service-account", url.Values{"accessKey": {s.ChildAccess}}, map[string]any{"newPolicy": json.RawMessage(policy), "newStatus": status}, nil); err != nil {
		return err
	}
	if _, err = admin.native(ctx, http.MethodGet, "info-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, &child); err != nil {
		return err
	}
	var expected, actual any
	parseExpected := json.Unmarshal(policy, &expected)
	parseActual := json.Unmarshal([]byte(child.Policy), &actual)
	normalizePolicyConditions(expected)
	normalizePolicyConditions(actual)
	if parseExpected != nil || parseActual != nil || hashValues(expected) != hashValues(actual) || child.Parent != s.ParentAccess || child.Implied {
		return failure("iam_verification_failed", nil)
	}
	machine, err := newAdminClient(s.Endpoint, s.ChildAccess, s.ChildSecret, m.http)
	if err != nil {
		return err
	}
	if status == "on" {
		var account struct {
			Identity string `json:"identity_type"`
		}
		if _, err = machine.native(ctx, http.MethodGet, "account/info", nil, nil, &account); err != nil {
			return failure("machine_credentials_failed", err)
		}
		deniedCode, _ := machine.native(ctx, http.MethodGet, "list-users", nil, nil, nil)
		if deniedCode != http.StatusForbidden && deniedCode != http.StatusUnauthorized {
			return failure("iam_verification_failed", nil)
		}
	} else if child.Status != "off" && child.Status != "disabled" {
		return failure("credential_revocation_failed", nil)
	}
	s.Provisioned = true
	s.PolicyHash = desiredPolicyHash
	s.IAMPending = false
	s.PendingPolicyHash = ""
	return nil
}

// Initialized stores are recovery targets, never candidates for silent IAM or bucket bootstrap.
func (m *Manager) inspectEstablishedIAM(ctx context.Context, cfg *config.Config, s persistedState, allowPending bool) error {
	if s.InitialBucket == "" || cfg.RustFS.Bucket != s.InitialBucket {
		return failure("recovery_required", nil)
	}
	if (s.Provisioned && s.PolicyHash == "") || (s.IAMPending && !allowPending) {
		return failure("recovery_required", nil)
	}
	admin, err := m.rootAdmin(s)
	if err != nil {
		return err
	}
	root, err := newDataClient(s.Endpoint, s.RootAccess, s.RootSecret, m.http)
	if err != nil {
		return err
	}
	exists, err := root.BucketExists(ctx, cfg.RustFS.Bucket)
	if err != nil || !exists {
		return failure("recovery_required", fmt.Errorf("established IAM artifact missing or changed"))
	}
	var parent struct {
		Policy string `json:"policyName"`
		Status string `json:"status"`
	}
	if _, err = admin.native(ctx, http.MethodGet, "user-info", url.Values{"accessKey": {s.ParentAccess}}, nil, &parent); err != nil || parent.Policy != s.PolicyName || parent.Status != "enabled" && parent.Status != "on" {
		return failure("recovery_required", fmt.Errorf("established IAM artifact missing or changed"))
	}
	var policy struct {
		Document json.RawMessage `json:"policy"`
	}
	if _, err = admin.native(ctx, http.MethodGet, "info-canned-policy", url.Values{"name": {s.PolicyName}}, nil, &policy); err != nil {
		return failure("recovery_required", fmt.Errorf("established IAM artifact missing or changed"))
	}
	var child serviceAccountInfo
	if _, err = admin.native(ctx, http.MethodGet, "info-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, &child); err != nil || child.Parent != s.ParentAccess || child.Description != "AuraGo managed S3 "+s.Instance || child.Implied {
		return failure("recovery_required", fmt.Errorf("established IAM artifact missing or changed"))
	}
	actualPolicyHash := policyHash(policy.Document)
	childPolicyHash := policyHash([]byte(child.Policy))
	if s.IAMPending {
		allowed := map[string]bool{s.PolicyHash: true, policyHash(generatePolicy(effectivePermissions(cfg))): true}
		if s.PendingPolicyHash != "" {
			allowed[s.PendingPolicyHash] = true
		}
		if !allowed[actualPolicyHash] || !allowed[childPolicyHash] {
			return failure("recovery_required", fmt.Errorf("established IAM artifact missing or changed"))
		}
		switch child.Status {
		case "on", "enabled", "off", "disabled":
		default:
			return failure("recovery_required", nil)
		}
	} else if actualPolicyHash != s.PolicyHash || childPolicyHash != s.PolicyHash {
		return failure("recovery_required", fmt.Errorf("established IAM artifact missing or changed"))
	}
	if s.IAMPending {
		if child.Status == "on" || child.Status == "enabled" {
			if err = m.verifyChildCredentials(ctx, s); err != nil {
				return err
			}
		}
	} else if s.PolicyHash == policyHash(generatePolicy(effectivePermissions(cfg))) {
		enabled := hasDataPermissions(effectivePermissions(cfg))
		if enabled && child.Status != "on" && child.Status != "enabled" || !enabled && child.Status != "off" && child.Status != "disabled" {
			return failure("recovery_required", nil)
		}
		if enabled {
			if err = m.verifyChildCredentials(ctx, s); err != nil {
				return err
			}
		}
	}
	return nil
}
