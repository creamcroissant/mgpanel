package service

import (
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/template"
)

type xrayArtifactRenderer struct {
	converter *template.XrayConverter
}

func newXrayArtifactRenderer() *xrayArtifactRenderer {
	return &xrayArtifactRenderer{converter: &template.XrayConverter{}}
}

func (r *xrayArtifactRenderer) CoreType() string {
	return "xray"
}

func (r *xrayArtifactRenderer) Render(
	spec *repository.InboundSpec,
	semantic *inboundSemanticSpec,
	semanticObject map[string]any,
	coreSpecific map[string]any,
) (*renderedArtifact, []ArtifactRenderWarning, error) {
	if spec == nil {
		return nil, nil, fmt.Errorf("nil inbound spec")
	}
	if semantic == nil {
		return nil, nil, fmt.Errorf("nil inbound semantic spec")
	}

	warnings := make([]ArtifactRenderWarning, 0)
	warnings = append(warnings, artifactSemanticUnknownWarnings(r.CoreType(), spec, semanticObject, map[string]struct{}{
		"tag":       {},
		"protocol":  {},
		"listen":    {},
		"port":      {},
		"tls":       {},
		"transport": {},
		"multiplex": {},
		"sniffing": {},
	})...)

	section, sectionWarnings, err := artifactExtractCoreSection(spec, r.CoreType(), coreSpecific, "xray")
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, sectionWarnings...)

	if !isProtocolSupportedByCore(semantic.Protocol, r.CoreType()) {
		return nil, nil, &ArtifactUnsupportedFieldError{
			CoreType: r.CoreType(),
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    "protocol",
			Message:  fmt.Sprintf("protocol %q is not supported by %s / 协议 %q 不被 %s 支持", semantic.Protocol, r.CoreType(), semantic.Protocol, r.CoreType()),
		}
	}

	inbound, err := buildUnifiedInboundFromSemantic(spec.Tag, semantic, semanticObject)
	if err != nil {
		return nil, nil, err
	}

	payload, err := r.converter.FromUnified([]template.UnifiedInbound{inbound})
	if err != nil {
		return nil, nil, err
	}
	coreInbound, err := artifactSingleInboundFromPayload(payload)
	if err != nil {
		return nil, nil, err
	}

	filenameOverride := ""
	if len(section) > 0 {
		filenameOverride, err = artifactApplyCoreSection(coreInbound, section, map[string]struct{}{
			"protocol":       {},
			"tag":            {},
			"listen":         {},
			"port":           {},
			"settings":       {},
			"streamSettings": {},
		}, spec, r.CoreType(), "core_specific.xray")
		if err != nil {
			return nil, nil, err
		}
	}

	filename, err := artifactResolveFilename(spec, filenameOverride)
	if err != nil {
		return nil, nil, err
	}

	content, err := artifactMarshalInbound(coreInbound)
	if err != nil {
		return nil, nil, err
	}

	return &renderedArtifact{
		Filename: filename,
		Content:  content,
	}, warnings, nil
}
