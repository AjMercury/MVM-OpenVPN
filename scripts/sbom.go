//go:build ignore

// Generates a dependency SBOM from the actual binary build info, without claiming license discovery.
package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: go run scripts/sbom.go BINARY VERSION OUTPUT")
	}
	info, e := buildinfo.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	raw, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	sum := sha256.Sum256(raw)
	root := map[string]any{"SPDXID": "SPDXRef-Application", "name": "mikrotik-vpn-manager", "versionInfo": os.Args[2], "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "licenseConcluded": "NOASSERTION", "licenseDeclared": "NOASSERTION", "copyrightText": "NOASSERTION", "checksums": []any{map[string]string{"algorithm": "SHA256", "checksumValue": hex.EncodeToString(sum[:])}}}
	packages := []any{root}
	relations := []any{map[string]string{"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES", "relatedSpdxElement": "SPDXRef-Application"}}
	for i, dep := range info.Deps {
		if dep.Replace != nil {
			dep = dep.Replace
		}
		id := fmt.Sprintf("SPDXRef-Dependency-%d", i)
		packages = append(packages, map[string]any{"SPDXID": id, "name": dep.Path, "versionInfo": dep.Version, "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "licenseConcluded": "NOASSERTION", "licenseDeclared": "NOASSERTION", "copyrightText": "NOASSERTION", "externalRefs": []any{map[string]string{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": "pkg:golang/" + dep.Path + "@" + dep.Version}}})
		relations = append(relations, map[string]string{"spdxElementId": "SPDXRef-Application", "relationshipType": "DEPENDS_ON", "relatedSpdxElement": id})
	}
	doc := map[string]any{"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT", "name": "mvm-binary-dependencies", "documentNamespace": "https://spdx.org/spdxdocs/mvm-" + hex.EncodeToString(sum[:]), "creationInfo": map[string]any{"creators": []string{"Tool: mvm-buildinfo-sbom"}, "created": time.Now().UTC().Format(time.RFC3339)}, "packages": packages, "relationships": relations}
	out, e := json.MarshalIndent(doc, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(os.Args[3], out, 0644); e != nil {
		panic(e)
	}
}
