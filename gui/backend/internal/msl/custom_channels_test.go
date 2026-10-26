package msl

import (
	"strings"
	"testing"
)

func TestBuildCustomChannelsAddsOnlyTheChannelParts(t *testing.T) {
	const decls = "float f() { return 1.0; }"
	const body = "float4 frag_color = iChannel0.sample(iChannelSampler, in.uv * 0.5 + 0.5);"
	plain := BuildCustomDecls(decls, body)
	channels := BuildCustomChannels(decls, body)
	for _, want := range []string{
		"texture2d<float> iChannel0 [[texture(1)]],",
		"texture2d<float> iChannel3 [[texture(4)]],",
		"constexpr sampler iChannelSampler(filter::linear, address::clamp_to_edge);",
		"float3 iChannelResolution[4] = {",
	} {
		if !strings.Contains(channels, want) {
			t.Errorf("channel source lacks %q", want)
		}
	}
	if strings.Contains(plain, "texture2d<float> iChannel") || strings.Contains(plain, "iChannelResolution") {
		t.Error("plain source declares channels")
	}
	// Plain plus the two channel parts and nothing else: a shader without
	// a Surface compiles exactly what it did before channels existed.
	stripped := strings.Replace(channels, mslChannelParams, "", 1)
	stripped = strings.Replace(stripped, mslChannelInit, "", 1)
	if stripped != plain {
		t.Error("channel source differs from the plain one beyond the channel parts")
	}
	// The textures are fs_main parameters, and the sampler and sizes are
	// set up before the body runs.
	ordered := strings.Index(channels, "fragment float4 fs_main(") < strings.Index(channels, "iChannel0 [[texture(1)]]") &&
		strings.Index(channels, "iChannel0 [[texture(1)]]") < strings.Index(channels, ") {\n    // Built-in inputs") &&
		strings.Index(channels, "iChannelResolution[4] = {") < strings.Index(channels, body)
	if !ordered {
		t.Error("channel parts out of order")
	}
}
