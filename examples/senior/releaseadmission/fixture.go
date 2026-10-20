package releaseadmission

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Fixture holds PUBLIC, deterministic teaching keys. Never use it for real
// releases. Synthetic producer reports exercise verification, not build quality.
type Fixture struct {
	Policy Policy
	Target Target
	Bundle Bundle
	Keys   map[string]ed25519.PrivateKey
	Root   string
	Time   time.Time
}

func NewFixture(root string, now time.Time) (*Fixture, error) {
	items, err := os.ReadDir(root)
	if err != nil || len(items) != 0 {
		return nil, ErrDenied
	}
	for name, content := range map[string]string{"go.mod": "module source.fixture\n\ngo 1.24.2\n", "main.go": "package main\nfunc main() {}\n"} {
		if err = os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			return nil, err
		}
	}
	inputs, err := Fingerprint(context.Background(), root)
	if err != nil {
		return nil, err
	}
	f := &Fixture{Root: root, Time: now, Keys: map[string]ed25519.PrivateKey{}}
	p := Policy{Version: "policy-v1", Repository: "registry.fixture.invalid/team/service", SourceRepo: "https://source.fixture.invalid/team/service", SourceRef: "refs/heads/master", Commit: strings.Repeat("a", 40), BuildType: "https://knowledgebase.invalid/build/go-v1", Builder: "https://knowledgebase.invalid/builder/v1", Issuer: "local-fixture-authority", Suite: "release-suite-v1", Generator: "fixture-sbom/1", Scanner: "fixture-scanner/1", DBVersion: "fixture-db-v1", MaxAge: 300, MaxDBAge: 86400, Trust: map[string]Signer{}, Identities: map[string]string{}}
	for _, role := range []string{"build", "test", "sbom", "scan", "approve", "exception"} {
		seed := sha256.Sum256([]byte("PUBLIC RELEASE TEACHING KEY " + role))
		private := ed25519.NewKeyFromSeed(seed[:])
		f.Keys[role] = private
		identity := "fixture/" + role + "/workflow@immutable-v1"
		p.Identities[role] = identity
		s := Signer{PublicKey: private.Public().(ed25519.PublicKey), Issuer: p.Issuer, Identity: identity, Role: role, Environment: "production"}
		if role == "build" {
			s.Builder = p.Builder
		}
		p.Trust[role] = s
	}
	f.Policy = p
	artifact := []byte("synthetic artifact bytes; never executed\n")
	d := Digest(artifact)
	f.Target = Target{"production", p.Repository + "@sha256:" + d, "linux/amd64", Digest([]byte("config")), Digest([]byte("migration")), Digest([]byte("flags")), "fixture:1", "proxy-fixture", "implementation-v1", Digest([]byte("implementation bytecode")), inputs}
	f.Bundle.Artifact = artifact
	f.Bundle.Provenance = f.Seal("build", ProvenanceType, object{
		"buildDefinition": object{"buildType": p.BuildType, "externalParameters": object{"repository": p.SourceRepo, "ref": p.SourceRef}, "resolvedDependencies": []any{object{"uri": p.SourceRepo, "digest": object{"gitCommit": p.Commit}}, object{"uri": "file:source-snapshot", "digest": object{"sha256": inputs}}}},
		"runDetails":      object{"builder": object{"id": p.Builder}, "metadata": object{"invocationId": "build-run-1", "finishedOn": now.UTC().Format(time.RFC3339)}},
	})
	f.Bundle.Tests = f.Seal("test", TestType, object{"completedAt": now.Unix(), "commit": p.Commit, "inputs": inputs, "run": "build-run-1", "suite": p.Suite, "platform": f.Target.Platform, "results": object{"unit": "passed", "race": "passed", "vet": "passed"}})
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	f.Bundle.SBOMReport, _ = json.Marshal(object{"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT", "name": "fixture inventory", "documentNamespace": "https://knowledgebase.invalid/sbom/" + Digest(nonce[:]), "creationInfo": object{"created": now.UTC().Format(time.RFC3339), "creators": []any{"Tool: " + p.Generator}}, "packages": []any{object{"SPDXID": "SPDXRef-Artifact", "name": "fixture artifact", "versionInfo": "v1", "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "licenseConcluded": "NOASSERTION", "licenseDeclared": "NOASSERTION", "copyrightText": "NOASSERTION", "checksums": []any{object{"algorithm": "SHA256", "checksumValue": d}}}}, "relationships": []any{object{"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES", "relatedSpdxElement": "SPDXRef-Artifact"}}})
	f.Bundle.ScanReport, _ = json.Marshal(object{"schema": "local-scan-v1", "status": "complete", "dbVersion": p.DBVersion, "dbUpdatedAt": now.Unix(), "findings": []any{}})
	f.LinkReports()
	f.Approve()
	return f, nil
}
func (f *Fixture) Seal(role, typ string, predicate any) Envelope {
	return Sign(Statement(f.Policy.Repository, Digest(f.Bundle.Artifact), typ, predicate), role, f.Keys[role])
}
func (f *Fixture) LinkReports() {
	for _, role := range []string{"sbom", "scan"} {
		raw, tool, typ := f.Bundle.SBOMReport, f.Policy.Generator, SBOMType
		if role == "scan" {
			raw, tool, typ = f.Bundle.ScanReport, f.Policy.Scanner, ScanType
		}
		e := f.Seal(role, typ, object{"reportDigest": Digest(raw), "platform": f.Target.Platform, "tool": tool, "completedAt": f.Time.Unix()})
		if role == "scan" {
			f.Bundle.Scan = e
		} else {
			f.Bundle.SBOM = e
		}
	}
}
func (f *Fixture) Approve() {
	f.Bundle.Approval = f.Seal("approve", ApprovalType, object{"id": "approval-1", "targetDigest": TargetDigest(f.Target), "evidenceDigest": EvidenceDigest(f.Bundle), "policy": f.Policy.Version, "issuedAt": f.Time.Unix(), "expiresAt": f.Time.Unix() + 240})
}
func (f *Fixture) Waive(risks ...string) {
	e := f.Seal("exception", ExceptionType, object{"id": "exception-1", "owner": "fixture-risk-owner", "reason": "controlled local exercise", "control": "no real deployment or funds", "policy": f.Policy.Version, "targetDigest": TargetDigest(f.Target), "evidenceDigest": EvidenceDigest(f.Bundle), "issuedAt": f.Time.Unix(), "expiresAt": f.Time.Unix() + 120, "risks": risks})
	f.Bundle.Exception = &e
}
