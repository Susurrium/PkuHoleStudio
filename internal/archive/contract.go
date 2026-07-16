package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	ArchiveSpecVersion             = "2.1.0"
	ArchiveExtensionMedia          = "io.github.susurrium.pkuhole.media"
	ArchiveExtensionStudioMetadata = "io.github.susurrium.pkuhole.studio-metadata"
	ArchiveExtensionStudioSources  = "io.github.susurrium.pkuhole.studio-sources"
)

var (
	archiveSpecVersionPattern  = regexp.MustCompile(`^2\.[0-9]+\.[0-9]+$`)
	archiveExtensionPattern    = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)+$`)
	supportedArchiveExtensions = map[string]int{
		ArchiveExtensionMedia:          1,
		ArchiveExtensionStudioMetadata: 1,
		ArchiveExtensionStudioSources:  1,
	}
)

type archiveExtensionDescriptor struct {
	Version  int  `json:"version"`
	Required bool `json:"required,omitempty"`
}

// PortableStudioSource is the stable v1 representation of Studio's optional
// multi-source extension. Timestamps and database-specific keys are excluded.
type PortableStudioSource struct {
	Source      string `json:"source"`
	SourceRef   string `json:"sourceRef,omitempty"`
	ContextOnly bool   `json:"contextOnly,omitempty"`
}

func (s *PortableStudioSource) UnmarshalJSON(data []byte) error {
	var value struct {
		Source            string `json:"source"`
		SourceRef         string `json:"sourceRef"`
		ContextOnly       *bool  `json:"contextOnly"`
		LegacySourceRef   string `json:"source_ref"`
		LegacyContextOnly bool   `json:"context_only"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	s.Source = value.Source
	s.SourceRef = value.SourceRef
	if s.SourceRef == "" {
		s.SourceRef = value.LegacySourceRef
	}
	s.ContextOnly = value.LegacyContextOnly
	if value.ContextOnly != nil {
		s.ContextOnly = *value.ContextOnly
	}
	return nil
}

type ArchiveContractCapabilities struct {
	SchemaVersions   []int          `json:"schema_versions"`
	WriteSpecVersion string         `json:"write_spec_version"`
	ReadZIPMethods   []string       `json:"read_zip_methods"`
	WriteZIPMethod   string         `json:"write_zip_method"`
	Extensions       map[string]int `json:"extensions"`
	MaxArchiveBytes  int64          `json:"max_archive_bytes"`
	MaxUncompressed  int64          `json:"max_uncompressed_bytes"`
}

// ContractCapabilities is independent of Toolkit and can be exposed by any
// Studio transport without coupling the archive implementation to that client.
func ContractCapabilities() ArchiveContractCapabilities {
	extensions := make(map[string]int, len(supportedArchiveExtensions))
	for name, version := range supportedArchiveExtensions {
		extensions[name] = version
	}
	return ArchiveContractCapabilities{
		SchemaVersions: []int{1, 2}, WriteSpecVersion: ArchiveSpecVersion,
		ReadZIPMethods: []string{"store", "deflate"}, WriteZIPMethod: "store",
		Extensions: extensions, MaxArchiveBytes: MaxArchiveBytes, MaxUncompressed: MaxUncompressedBytes,
	}
}

func validateV2ManifestContract(fields map[string]json.RawMessage) error {
	rawSpecVersion, revisionDeclared := fields["specVersion"]
	if !revisionDeclared {
		// v2.0 archives predate the precise contract revision. The original
		// structural checks remain their compatibility boundary.
		return nil
	}
	var specVersion string
	if err := json.Unmarshal(rawSpecVersion, &specVersion); err != nil || !archiveSpecVersionPattern.MatchString(specVersion) {
		return errors.New("manifest specVersion must identify a compatible 2.x.y contract")
	}
	if rawProducer, ok := fields["producer"]; ok {
		var producer struct {
			Name    string `json:"name"`
			Version any    `json:"version,omitempty"`
		}
		if err := json.Unmarshal(rawProducer, &producer); err != nil || strings.TrimSpace(producer.Name) == "" {
			return errors.New("manifest producer must contain a non-empty name")
		}
		if producer.Version != nil {
			version, ok := producer.Version.(string)
			if !ok || strings.TrimSpace(version) == "" {
				return errors.New("manifest producer version must be a string")
			}
		}
	}
	var manifestErrors []json.RawMessage
	if err := json.Unmarshal(fields["errors"], &manifestErrors); err != nil {
		return errors.New("manifest errors must be an array")
	}
	for _, rawError := range manifestErrors {
		if !jsonObject(rawError) {
			return errors.New("manifest errors must contain objects")
		}
	}
	if err := validateArchiveScope(fields["scope"]); err != nil {
		return err
	}

	extensions := map[string]archiveExtensionDescriptor{}
	if rawExtensions, ok := fields["extensions"]; ok {
		if err := json.Unmarshal(rawExtensions, &extensions); err != nil {
			return errors.New("manifest extensions must be an object")
		}
		for name, descriptor := range extensions {
			if !archiveExtensionPattern.MatchString(name) || descriptor.Version < 1 {
				return fmt.Errorf("manifest extension %q is invalid", name)
			}
			if descriptor.Required {
				if err := requireSupportedArchiveExtension(name, descriptor.Version); err != nil {
					return err
				}
			}
		}
	}
	if rawRequired, ok := fields["requiredExtensions"]; ok {
		var required []string
		if err := json.Unmarshal(rawRequired, &required); err != nil {
			return errors.New("manifest requiredExtensions must be an array of names")
		}
		seen := map[string]bool{}
		for _, name := range required {
			if seen[name] {
				return fmt.Errorf("manifest required extension %q is duplicated", name)
			}
			seen[name] = true
			descriptor, declared := extensions[name]
			if !declared {
				return fmt.Errorf("manifest required extension %q is not declared", name)
			}
			if err := requireSupportedArchiveExtension(name, descriptor.Version); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateArchiveScope(raw json.RawMessage) error {
	var scope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &scope); err != nil {
		return errors.New("manifest scope must be an object")
	}
	if nested, ok := scope["scope"]; ok {
		if err := json.Unmarshal(nested, &scope); err != nil {
			return errors.New("manifest scope.scope must be an object")
		}
	}
	var scopeType string
	if err := json.Unmarshal(scope["type"], &scopeType); err != nil || strings.TrimSpace(scopeType) == "" {
		return errors.New("manifest scope must contain a non-empty type")
	}
	return nil
}

func requireSupportedArchiveExtension(name string, version int) error {
	supportedVersion, ok := supportedArchiveExtensions[name]
	if !ok || version > supportedVersion {
		return fmt.Errorf("archive requires unsupported extension %s@%d", name, version)
	}
	return nil
}
