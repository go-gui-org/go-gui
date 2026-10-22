package shaderport

import (
	"slices"
	"strings"
)

// mslTypes maps GLSL type names to MSL. Both languages store matrices
// column-major and multiply them the same way, so only the names change.
var mslTypes = map[string]string{
	"vec2": "float2", "vec3": "float3", "vec4": "float4",
	"ivec2": "int2", "ivec3": "int3", "ivec4": "int4",
	"uvec2": "uint2", "uvec3": "uint3", "uvec4": "uint4",
	"bvec2": "bool2", "bvec3": "bool3", "bvec4": "bool4",
	"mat2": "float2x2", "mat3": "float3x3", "mat4": "float4x4",
	"mat2x2": "float2x2", "mat2x3": "float2x3", "mat2x4": "float2x4",
	"mat3x2": "float3x2", "mat3x3": "float3x3", "mat3x4": "float3x4",
	"mat4x2": "float4x2", "mat4x3": "float4x3", "mat4x4": "float4x4",
}

// glslScalars are the scalar type names, the same in both languages.
// With mslTypes they tell a constructor call from a function call.
var glslScalars = []string{"float", "int", "uint", "bool"}

func isGLSLType(name string) bool {
	_, ok := mslTypes[name]
	return ok || slices.Contains(glslScalars, name)
}

// mslRenamedFuncs are GLSL built-in functions whose MSL name differs.
// A call keeps its arguments; only the name changes.
var mslRenamedFuncs = map[string]string{
	"dFdx":        "dfdx",
	"dFdy":        "dfdy",
	"inversesqrt": "rsqrt",
	"roundEven":   "rint", // MSL rint rounds half to even
	// GLSL mod is x - y*floor(x/y). MSL fmod truncates instead, so the
	// two differ for a negative x. _gg_mod is the GLSL version, written
	// into MetalDecls when the shader calls mod.
	"mod": modHelperName,
}

const modHelperName = "_gg_mod"

// modHelper is the MSL definition of _gg_mod. A template covers
// mod(genType, genType) and mod(genType, float) in one definition.
const modHelper = `// GLSL mod: x - y * floor(x / y). MSL fmod truncates instead,
// which differs for a negative x.
template <typename T, typename U>
T _gg_mod(T x, U y) { return x - y * floor(x / y); }
`

// mslCompareFuncs are GLSL's component-wise comparisons. MSL compares
// vectors with the plain operators, which return a bool vector.
var mslCompareFuncs = map[string]string{
	"lessThan":         "<",
	"lessThanEqual":    "<=",
	"greaterThan":      ">",
	"greaterThanEqual": ">=",
	"equal":            "==",
	"notEqual":         "!=",
}

// rejected names a GLSL or Shadertoy identifier the translator cannot
// carry over, with the reason given to the user.
var rejected = map[string]string{
	"iMouse": "iMouse is not supported yet: go-gui does not pass the " +
		"pointer to shaders. Replace it with a constant or a Params entry",
	"iDate":       "iDate is not provided by go-gui",
	"iSampleRate": "iSampleRate is not provided by go-gui",
	"iFrameRate":  "iFrameRate is not provided by go-gui; use 1.0 / iTimeDelta",
	"iChannelTime": "texture input (iChannel*) is not supported yet: " +
		"go-gui binds no textures to a custom shader",
	"iChannelResolution": "texture input (iChannel*) is not supported yet: " +
		"go-gui binds no textures to a custom shader",
	"gl_FragCoord": "gl_FragCoord is not supported: use mainImage's fragCoord",
	"gl_FragColor": "gl_FragColor is not supported: use mainImage's fragColor",
	"uniform":      "uniforms are not supported: go-gui provides iResolution, iTime, iTimeDelta and iFrame, and Shader.Params for app values",
	// GLSL built-in functions with no MSL counterpart.
	"inverse":         "inverse() has no MSL built-in; write the inverse by hand",
	"outerProduct":    "outerProduct() has no MSL built-in; write it by hand",
	"matrixCompMult":  "matrixCompMult() has no MSL built-in; multiply the columns by hand",
	"floatBitsToInt":  "floatBitsToInt() is not supported yet",
	"floatBitsToUint": "floatBitsToUint() is not supported yet",
	"intBitsToFloat":  "intBitsToFloat() is not supported yet",
	"uintBitsToFloat": "uintBitsToFloat() is not supported yet",
	"packUnorm2x16":   "packUnorm2x16() is not supported",
	"packSnorm2x16":   "packSnorm2x16() is not supported",
	"unpackUnorm2x16": "unpackUnorm2x16() is not supported",
	"unpackSnorm2x16": "unpackSnorm2x16() is not supported",
	"packHalf2x16":    "packHalf2x16() is not supported",
	"unpackHalf2x16":  "unpackHalf2x16() is not supported",
}

// rejectReason returns why name cannot be translated, or "".
func rejectReason(name string) string {
	if r, ok := rejected[name]; ok {
		return r
	}
	switch {
	case strings.HasPrefix(name, "iChannel"),
		strings.HasPrefix(name, "texture"),
		strings.HasPrefix(name, "texel"),
		strings.HasPrefix(name, "sampler"):
		return name + ": texture input (iChannel*, texture(), samplers) is " +
			"not supported yet: go-gui binds no textures to a custom shader"
	}
	return ""
}

// precisionWords are GLSL ES precision qualifiers. MSL has none: the
// translator drops them.
var precisionWords = []string{"highp", "mediump", "lowp"}

// builtin is one of go-gui's Shadertoy inputs. In GLSL they are globals
// any function can read. In MSL they are locals of the fragment
// function, so a helper that reads one gets it as a parameter.
type builtin struct {
	name    string
	mslType string
	anim    bool // the value changes from frame to frame
}

// builtins lists the inputs in the order hidden parameters are added.
var builtins = []builtin{
	{"iResolution", "float3", false},
	{"iTime", "float", true},
	{"iTimeDelta", "float", true},
	{"iFrame", "int", true},
}

// builtinBit returns the bit for a built-in input name, or 0.
func builtinBit(name string) uint8 {
	for i, b := range builtins {
		if b.name == name {
			return 1 << i
		}
	}
	return 0
}

// mslKeywords are identifiers that GLSL ES 3.00 allows as names but MSL
// (a C++14 dialect) reserves or predefines as types. The translator
// appends "_" to every use, in MSL output only.
var mslKeywords = map[string]bool{
	"alignas": true, "alignof": true, "array": true, "auto": true,
	"catch": true, "char": true, "char16_t": true, "char32_t": true,
	"concept": true, "const_cast": true, "constant": true,
	"constexpr": true, "decltype": true, "delete": true, "device": true,
	"dynamic_cast": true, "explicit": true, "export": true,
	"fragment": true, "friend": true, "kernel": true, "metal": true,
	"mutable": true, "new": true, "noexcept": true, "nullptr": true,
	"object_data": true, "operator": true, "private": true,
	"protected": true, "ptrdiff_t": true, "ray_data": true,
	"register": true, "reinterpret_cast": true, "requires": true,
	"size_t": true, "static_assert": true, "static_cast": true,
	"thread": true, "thread_local": true, "threadgroup": true,
	"throw": true, "try": true, "typename": true, "uchar": true,
	"ulong": true, "ushort": true, "vec": true, "vertex": true,
	"visible": true, "wchar_t": true,
}

// isMSLKeyword also covers MSL's sized vector and matrix type names
// (float2, int3x3, ...), which GLSL leaves free.
func isMSLKeyword(name string) bool {
	if mslKeywords[name] {
		return true
	}
	for _, base := range []string{"float", "int", "uint", "bool", "half",
		"short", "ushort", "char", "uchar", "long", "ulong"} {
		rest, ok := strings.CutPrefix(name, base)
		if ok && len(rest) > 0 && rest[0] >= '2' && rest[0] <= '4' &&
			(len(rest) == 1 || (len(rest) == 3 && rest[1] == 'x' &&
				rest[2] >= '2' && rest[2] <= '4')) {
			return true
		}
	}
	return false
}

// mslStdlibNames are metal_stdlib functions a shader may define for
// itself (saturate is common). With `using namespace metal` a user
// function of the same signature is an ambiguous call, so a
// program-scope definition of one of these names is renamed.
var mslStdlibNames = map[string]bool{
	"absdiff": true, "acospi": true, "asinpi": true, "atan2": true,
	"atan2pi": true, "atanpi": true, "clz": true, "copysign": true,
	"cospi": true, "ctz": true, "dfdx": true, "dfdy": true,
	"distance_squared": true, "divide": true, "exp10": true,
	"extract_bits": true, "fdim": true, "fmax": true, "fmax3": true,
	"fmedian3": true, "fmin": true, "fmin3": true, "fmod": true,
	"ilogb": true, "insert_bits": true, "isfinite": true,
	"isnormal": true, "isordered": true, "isunordered": true,
	"length_squared": true, "log10": true, "mad": true, "max3": true,
	"median3": true, "min3": true, "nextafter": true, "popcount": true,
	"powr": true, "precise": true, "fast": true, "reverse_bits": true,
	"rint": true, "rotate": true, "rsqrt": true, "saturate": true,
	"select": true, "signbit": true, "sincos": true, "sinpi": true,
	"tanpi": true,
}

// wrapperNames are program-scope names the go-gui wrappers declare
// (gui.BuildGLSLFragmentDecls, msl.BuildCustomDecls). A shader that
// declares one of them at program scope is renamed in both outputs.
var wrapperNames = map[string]bool{
	"tex": true, "uv": true, "color": true, "params": true,
	"p0": true, "p1": true, "p2": true, "p3": true, "main": true,
	"_frag_out": true, "VertexIn": true, "VertexOut": true,
	"vs_main": true, "fs_main": true, "smp": true,
}

// bareInputs are the wrapper varyings a go-gui body reads by bare name
// in GLSL. In MSL they are fields of the fragment input, `in`.
var bareInputs = map[string]bool{
	"uv": true, "color": true, "params": true,
	"p0": true, "p1": true, "p2": true, "p3": true,
}

// stpqToXYZW maps GLSL's texture-coordinate swizzle letters. MSL
// accepts xyzw and rgba but not stpq.
func stpqToXYZW(name string) (string, bool) {
	if len(name) == 0 || len(name) > 4 {
		return "", false
	}
	b := []byte(name)
	for i, c := range b {
		switch c {
		case 's':
			b[i] = 'x'
		case 't':
			b[i] = 'y'
		case 'p':
			b[i] = 'z'
		case 'q':
			b[i] = 'w'
		default:
			return "", false
		}
	}
	return string(b), true
}
