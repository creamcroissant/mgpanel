package service

import (
	"fmt"
	"sort"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/template"
)

type singBoxArtifactRenderer struct {
	converter *template.SingBoxConverter
}

func newSingBoxArtifactRenderer() *singBoxArtifactRenderer {
	return &singBoxArtifactRenderer{converter: &template.SingBoxConverter{}}
}

func (r *singBoxArtifactRenderer) CoreType() string {
	return "sing-box"
}

func (r *singBoxArtifactRenderer) Render(
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

	section, sectionWarnings, err := artifactExtractCoreSection(spec, r.CoreType(), coreSpecific, "sing-box", "singbox")
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
			"type":        {},
			"tag":         {},
			"listen":      {},
			"listen_port": {},
		}, spec, r.CoreType(), "core_specific.sing-box")
		if err != nil {
			return nil, nil, err
		}

		warnings = append(warnings, singBoxWarningsFromSection(spec, section)...) // explicit non-fatal compatibility warnings
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

func singBoxWarningsFromSection(spec *repository.InboundSpec, section map[string]any) []ArtifactRenderWarning {
	if spec == nil || len(section) == 0 {
		return nil
	}

	warnings := make([]ArtifactRenderWarning, 0)
	if _, exists := section["server_names"]; exists {
		warnings = append(warnings, ArtifactRenderWarning{
			CoreType: "sing-box",
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    "core_specific.sing-box.server_names",
			Message:  "sing-box reality only uses one server_name; renderer keeps first value / sing-box reality 仅使用一个 server_name，渲染器仅保留首个值",
		})
	}

	if value, exists := section["users"]; exists {
		if users, ok := value.([]any); ok {
			for idx, userRaw := range users {
				userMap, ok := artifactToMap(userRaw)
				if !ok {
					continue
				}
				if _, hasFlow := userMap["flow"]; hasFlow {
					warnings = append(warnings, ArtifactRenderWarning{
						CoreType: "sing-box",
						SpecID:   spec.ID,
						Tag:      spec.Tag,
						Field:    fmt.Sprintf("core_specific.sing-box.users[%d].flow", idx),
						Message:  "flow is effective only when reality is enabled / flow 仅在启用 reality 时生效",
					})
				}
			}
		}
	}

	if len(warnings) == 0 {
		return nil
	}

	sort.SliceStable(warnings, func(i, j int) bool {
		return warnings[i].Field < warnings[j].Field
	})
	return warnings
}
