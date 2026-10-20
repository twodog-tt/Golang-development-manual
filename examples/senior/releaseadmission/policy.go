package releaseadmission

import (
	"crypto/ed25519"
	"encoding/hex"
	"reflect"
	"regexp"
	"strings"
	"time"
)

type Target struct{ Environment, Image, Platform, Config, Migration, Flags, ChainID, Proxy, Implementation, CodeHash, Inputs string }
type Signer struct {
	PublicKey                                    ed25519.PublicKey
	Issuer, Identity, Role, Environment, Builder string
}
type Policy struct {
	Version, Repository, SourceRepo, SourceRef, Commit, BuildType, Builder, Issuer, Suite, Generator, Scanner, DBVersion string
	MaxAge, MaxDBAge                                                                                                     int64
	Trust                                                                                                                map[string]Signer
	Identities                                                                                                           map[string]string
}
type Bundle struct {
	Artifact, SBOMReport, ScanReport        []byte
	Provenance, Tests, SBOM, Scan, Approval Envelope
	Exception                               *Envelope
}

var spdxID = regexp.MustCompile(`^SPDXRef-[A-Za-z0-9.-]+$`)

func TargetDigest(t Target) string { return binding(t) }

// EvidenceDigest excludes approval/exception to avoid circular signatures. Their
// exact bytes are nevertheless bound by the gate's intent/ticket binding.
func EvidenceDigest(b Bundle) string {
	return binding([]any{Digest(b.Artifact), Digest(b.SBOMReport), Digest(b.ScanReport), b.Provenance, b.Tests, b.SBOM, b.Scan})
}
func (p Policy) valid() bool {
	if p.Version == "" || p.Repository == "" || strings.ContainsAny(p.Repository, "@?#") || p.SourceRepo == "" || p.SourceRef == "" || p.BuildType == "" || p.Builder == "" || p.Issuer == "" || p.Suite == "" || p.Generator == "" || p.Scanner == "" || p.DBVersion == "" || p.MaxAge <= 0 || p.MaxAge > 3600 || p.MaxDBAge <= 0 || p.MaxDBAge > 7*86400 {
		return false
	}
	b, err := hex.DecodeString(p.Commit)
	if err != nil || len(b) != 20 || hex.EncodeToString(b) != p.Commit {
		return false
	}
	seen := map[string]bool{}
	for id, s := range p.Trust {
		if id == "" || len(s.PublicKey) != ed25519.PublicKeySize || seen[string(s.PublicKey)] || s.Identity == "" || s.Environment == "" {
			return false
		}
		seen[string(s.PublicKey)] = true
	}
	for _, role := range []string{"build", "test", "sbom", "scan", "approve", "exception"} {
		if p.Identities[role] == "" {
			return false
		}
	}
	return true
}
func (p Policy) artifact(t Target) (string, bool) {
	const marker = "@sha256:"
	d := strings.TrimPrefix(t.Image, p.Repository+marker)
	if !isDigest(d) || t.Image != p.Repository+marker+d || t.Environment == "" || t.Platform == "" || t.ChainID == "" || t.Proxy == "" || t.Implementation == "" {
		return "", false
	}
	for _, h := range []string{t.Config, t.Migration, t.Flags, t.CodeHash, t.Inputs} {
		if !isDigest(h) {
			return "", false
		}
	}
	return d, true
}
func fresh(timestamp, now, age int64) bool {
	return timestamp > 0 && timestamp <= now && now-timestamp <= age
}
func (p Policy) verify(e Envelope, role, typ string, t Target, revoked map[string]bool) (object, string, error) {
	d, ok := p.artifact(t)
	if !ok || len(e.Payload) > 128<<10 || e.PayloadType != PayloadType || len(e.Signatures) != 1 {
		return nil, "", ErrDenied
	}
	sig := e.Signatures[0]
	s, ok := p.Trust[sig.KeyID]
	if !ok || revoked[sig.KeyID] || len(s.PublicKey) != ed25519.PublicKeySize || !ed25519.Verify(s.PublicKey, pae(e.PayloadType, e.Payload), sig.Sig) || s.Role != role || s.Issuer != p.Issuer || s.Identity != p.Identities[role] || s.Environment != t.Environment {
		return nil, "", ErrDenied
	}
	m, err := parse(e.Payload)
	if err != nil || str(m["_type"]) != StatementType || str(m["predicateType"]) != typ {
		return nil, "", ErrDenied
	}
	subjects := list(m["subject"])
	if len(subjects) != 1 {
		return nil, "", ErrDenied
	}
	subject := obj(subjects[0])
	hashes := obj(subject["digest"])
	if str(subject["name"]) != p.Repository || str(hashes["sha256"]) != d {
		return nil, "", ErrDenied
	}
	pred := obj(m["predicate"])
	if pred == nil {
		return nil, "", ErrDenied
	}
	return pred, sig.KeyID, nil
}
func (p Policy) provenance(e Envelope, t Target, now int64, revoked map[string]bool) (string, error) {
	x, id, err := p.verify(e, "build", ProvenanceType, t, revoked)
	if err != nil {
		return "", err
	}
	b := obj(x["buildDefinition"])
	run := obj(x["runDetails"])
	metadata := obj(run["metadata"])
	if str(b["buildType"]) != p.BuildType || str(obj(run["builder"])["id"]) != p.Builder || p.Trust[id].Builder != p.Builder {
		return "", ErrDenied
	}
	expected := object{"repository": p.SourceRepo, "ref": p.SourceRef}
	if !reflect.DeepEqual(obj(b["externalParameters"]), expected) {
		return "", ErrDenied
	}
	source, inputs := false, false
	for _, raw := range list(b["resolvedDependencies"]) {
		v := obj(raw)
		d := obj(v["digest"])
		switch str(v["uri"]) {
		case p.SourceRepo:
			if source || str(d["gitCommit"]) != p.Commit {
				return "", ErrDenied
			}
			source = true
		case "file:source-snapshot":
			if inputs || str(d["sha256"]) != t.Inputs {
				return "", ErrDenied
			}
			inputs = true
		}
	}
	finished, err := time.Parse(time.RFC3339, str(metadata["finishedOn"]))
	if err != nil || !fresh(finished.Unix(), now, p.MaxAge) || !source || !inputs || str(metadata["invocationId"]) == "" {
		return "", ErrDenied
	}
	return str(metadata["invocationId"]), nil
}
func (p Policy) tests(e Envelope, t Target, run string, now int64, revoked map[string]bool) error {
	x, _, err := p.verify(e, "test", TestType, t, revoked)
	if err != nil {
		return err
	}
	ts, ok := integer(x["completedAt"])
	if !ok || !fresh(ts, now, p.MaxAge) || str(x["commit"]) != p.Commit || str(x["inputs"]) != t.Inputs || str(x["run"]) != run || str(x["suite"]) != p.Suite || str(x["platform"]) != t.Platform {
		return ErrDenied
	}
	results := obj(x["results"])
	if !exactKeys(results, "unit", "race", "vet") {
		return ErrDenied
	}
	for _, v := range results {
		if str(v) != "passed" {
			return ErrDenied
		}
	}
	return nil
}
func (p Policy) reportLink(e Envelope, role, typ string, t Target, raw []byte, now int64, revoked map[string]bool) error {
	x, _, err := p.verify(e, role, typ, t, revoked)
	if err != nil {
		return err
	}
	ts, ok := integer(x["completedAt"])
	tool := p.Generator
	if role == "scan" {
		tool = p.Scanner
	}
	if !ok || !fresh(ts, now, p.MaxAge) || str(x["reportDigest"]) != Digest(raw) || str(x["platform"]) != t.Platform || str(x["tool"]) != tool {
		return ErrDenied
	}
	return nil
}

// sbom accepts ONLY a documented SPDX-2.3 JSON profile, not every SPDX document.
// The root package checksums the actual artifact; relationships are validated.
func (p Policy) sbom(b Bundle, t Target, now int64, revoked map[string]bool) error {
	if err := p.reportLink(b.SBOM, "sbom", SBOMType, t, b.SBOMReport, now, revoked); err != nil {
		return err
	}
	m, err := parse(b.SBOMReport)
	if err != nil {
		return err
	}
	if !exactKeys(m, "spdxVersion", "dataLicense", "SPDXID", "name", "documentNamespace", "creationInfo", "packages", "relationships") {
		return ErrDenied
	}
	if str(m["spdxVersion"]) != "SPDX-2.3" || str(m["dataLicense"]) != "CC0-1.0" || str(m["SPDXID"]) != "SPDXRef-DOCUMENT" || str(m["name"]) == "" || !isDigest(strings.TrimPrefix(str(m["documentNamespace"]), "https://knowledgebase.invalid/sbom/")) || !strings.HasPrefix(str(m["documentNamespace"]), "https://knowledgebase.invalid/sbom/") {
		return ErrDenied
	}
	info := obj(m["creationInfo"])
	if !exactKeys(info, "created", "creators") {
		return ErrDenied
	}
	created, err := time.Parse(time.RFC3339, str(info["created"]))
	if err != nil || !fresh(created.Unix(), now, p.MaxAge) {
		return ErrDenied
	}
	creators := list(info["creators"])
	if len(creators) != 1 || str(creators[0]) != "Tool: "+p.Generator {
		return ErrDenied
	}
	packages := list(m["packages"])
	if len(packages) == 0 || len(packages) > 64 {
		return ErrDenied
	}
	ids := map[string]bool{"SPDXRef-DOCUMENT": true}
	root := false
	artifact, _ := p.artifact(t)
	for _, v := range packages {
		pkg := obj(v)
		if !exactKeys(pkg, "SPDXID", "name", "versionInfo", "downloadLocation", "filesAnalyzed", "licenseConcluded", "licenseDeclared", "copyrightText", "checksums") {
			return ErrDenied
		}
		id := str(pkg["SPDXID"])
		if !spdxID.MatchString(id) || ids[id] || str(pkg["name"]) == "" || str(pkg["versionInfo"]) == "" || str(pkg["downloadLocation"]) != "NOASSERTION" || pkg["filesAnalyzed"] != false || str(pkg["licenseConcluded"]) != "NOASSERTION" || str(pkg["licenseDeclared"]) != "NOASSERTION" || str(pkg["copyrightText"]) != "NOASSERTION" {
			return ErrDenied
		}
		ids[id] = true
		hashes := list(pkg["checksums"])
		if len(hashes) != 1 || !exactKeys(obj(hashes[0]), "algorithm", "checksumValue") || str(obj(hashes[0])["algorithm"]) != "SHA256" || !isDigest(str(obj(hashes[0])["checksumValue"])) {
			return ErrDenied
		}
		if id == "SPDXRef-Artifact" {
			if len(hashes) != 1 || str(obj(hashes[0])["algorithm"]) != "SHA256" || str(obj(hashes[0])["checksumValue"]) != artifact {
				return ErrDenied
			}
			root = true
		}
	}
	describes := 0
	relations := map[string]bool{}
	for _, v := range list(m["relationships"]) {
		rel := obj(v)
		if !exactKeys(rel, "spdxElementId", "relatedSpdxElement", "relationshipType") {
			return ErrDenied
		}
		a, c, kind := str(rel["spdxElementId"]), str(rel["relatedSpdxElement"]), str(rel["relationshipType"])
		k := binding([]string{a, kind, c})
		if !ids[a] || !ids[c] || relations[k] {
			return ErrDenied
		}
		relations[k] = true
		if a == "SPDXRef-DOCUMENT" && kind == "DESCRIBES" && c == "SPDXRef-Artifact" {
			describes++
		} else if a != "SPDXRef-Artifact" || kind != "DEPENDS_ON" || c == "SPDXRef-DOCUMENT" || c == a {
			return ErrDenied
		}
	}
	if !root || describes != 1 {
		return ErrDenied
	}
	return nil
}
func (p Policy) scan(b Bundle, t Target, now int64, revoked map[string]bool) ([]string, error) {
	if err := p.reportLink(b.Scan, "scan", ScanType, t, b.ScanReport, now, revoked); err != nil {
		return nil, err
	}
	m, err := parse(b.ScanReport)
	if err != nil {
		return nil, err
	}
	db, ok := integer(m["dbUpdatedAt"])
	a, hasFindings := m["findings"].([]any)
	if !ok || !fresh(db, now, p.MaxDBAge) || str(m["schema"]) != "local-scan-v1" || str(m["status"]) != "complete" || str(m["dbVersion"]) != p.DBVersion || !hasFindings {
		return nil, ErrDenied
	}
	seen := map[string]bool{}
	blocking := []string{}
	for _, v := range a {
		finding := obj(v)
		id := str(finding["id"])
		if id == "" || seen[id] {
			return nil, ErrDenied
		}
		seen[id] = true
		switch str(finding["severity"]) {
		case "critical", "high":
			blocking = append(blocking, id)
		case "medium", "low":
		default:
			return nil, ErrDenied
		}
	}
	return blocking, nil
}
func (p Policy) approval(b Bundle, t Target, now int64, revokedKeys, revokedApprovals map[string]bool) error {
	x, _, err := p.verify(b.Approval, "approve", ApprovalType, t, revokedKeys)
	if err != nil {
		return err
	}
	expires, ok := integer(x["expiresAt"])
	issued, issuedOK := integer(x["issuedAt"])
	id := str(x["id"])
	if !ok || !issuedOK || !fresh(issued, now, p.MaxAge) || expires <= now || expires-issued > p.MaxAge || id == "" || revokedApprovals[id] || str(x["policy"]) != p.Version || str(x["targetDigest"]) != TargetDigest(t) || str(x["evidenceDigest"]) != EvidenceDigest(b) {
		return ErrDenied
	}
	return nil
}
func (p Policy) exception(b Bundle, t Target, risks []string, now int64, revokedKeys, revokedExceptions map[string]bool) error {
	if len(risks) == 0 {
		if b.Exception != nil {
			return ErrDenied
		}
		return nil
	}
	if b.Exception == nil {
		return ErrDenied
	}
	x, _, err := p.verify(*b.Exception, "exception", ExceptionType, t, revokedKeys)
	if err != nil {
		return err
	}
	expires, ok := integer(x["expiresAt"])
	issued, issuedOK := integer(x["issuedAt"])
	if !ok || !issuedOK || !fresh(issued, now, p.MaxAge) || expires <= now || expires-issued > p.MaxAge || str(x["id"]) == "" || revokedExceptions[str(x["id"])] || str(x["owner"]) == "" || str(x["reason"]) == "" || str(x["control"]) == "" || str(x["policy"]) != p.Version || str(x["targetDigest"]) != TargetDigest(t) || str(x["evidenceDigest"]) != EvidenceDigest(b) {
		return ErrDenied
	}
	ids := list(x["risks"])
	if len(ids) != len(risks) {
		return ErrDenied
	}
	remaining := map[string]bool{}
	for _, v := range risks {
		remaining[v] = true
	}
	for _, v := range ids {
		id := str(v)
		if !remaining[id] {
			return ErrDenied
		}
		delete(remaining, id)
	}
	return nil
}
